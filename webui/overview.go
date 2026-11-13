package webui

import (
	"cmp"
	"context"
	"math"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dundee/gdu/v5/pkg/cleanup"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// The grid limits the response for large scans. Every dated file contributes
// to a cell, and both the plot and the file filter use the same coordinates.
const (
	overviewColumns = 96
	overviewRows    = 48
	overviewPage    = 50
	secondsPerDay   = 86400
)

type overviewCache struct {
	mu         sync.Mutex
	root       fs.Item
	gen        uint64
	path       string
	data       *fileInventory
	candidates *cleanup.Result
}

type overviewFile struct {
	fileSafety
	Owner    *fileOwner `json:"owner"`
	Path     string     `json:"path"`
	Name     string     `json:"name"`
	Category string     `json:"category"`
	Usage    int64      `json:"usage"`
	Size     int64      `json:"size"`
	Mtime    int64      `json:"mtime"`
}

type overviewCategory struct {
	ID    string `json:"id"`
	Bytes int64  `json:"bytes"`
	Count int    `json:"count"`
}

type overviewCell struct {
	X     int `json:"x"`
	Y     int `json:"y"`
	Count int `json:"count"`
}

type overviewResult struct {
	Owners          []ownerGroup       `json:"owners"`
	OwnerCount      int                `json:"ownerCount"`
	OwnerOffset     int                `json:"ownerOffset"`
	Directories     []overviewCategory `json:"directories,omitempty"`
	DirectoryCount  int                `json:"directoryCount"`
	DirectoryOffset int                `json:"directoryOffset"`
	Categories      []overviewCategory `json:"categories"`
	Grades          []overviewCategory `json:"grades"`
	Files           []overviewFile     `json:"files"`
	Points          []overviewCell     `json:"points"`
	FileCount       int                `json:"fileCount"`
	FileBytes       int64              `json:"fileBytes"`
	MatchedCount    int                `json:"matchedCount"`
	MatchedBytes    int64              `json:"matchedBytes"`
	UndatedCount    int                `json:"undatedCount"`
	Offset          int                `json:"offset"`
	PageSize        int                `json:"pageSize"`
	Columns         int                `json:"columns"`
	Rows            int                `json:"rows"`
	MaxAgeDays      float64            `json:"maxAgeDays"`
	MaxBytes        int64              `json:"maxBytes"`
}

type fileInventory struct {
	files    []overviewFile
	order    [2][]int
	maxBytes [2]int64
	oldest   int64
	now      int64
}

type overviewFilter struct {
	groupByDirectory bool
	groupByOwner     bool
	owner            string
	ownerOffset      int
	directoryOffset  int
	folder           string
	category         string
	grade            cleanupGrade
	box              *[4]int
	metric           int
	offset           int
}

var fileCategories = []struct {
	id         string
	extensions string
}{
	{"video", ".mp4 .mkv .mov .avi .webm .m4v .m4s .mxf .flv .wmv .mts .m2ts .r3d .braw .vob .mpg .mpeg"},
	{"audio", ".mp3 .wav .flac .aac .m4a .ogg .opus .wma .aiff .alac .amr .mka"},
	{"image", ".jpg .jpeg .png .gif .webp .bmp .heic .heif .avif .svg .tif .tiff .psd .psb .dng .cr2 .cr3 .nef .arw .exr .ico"},
	{"model", ".gguf .safetensors .onnx .ckpt .pt .pth .tflite .ggml .pb .mlmodel .mlpackage"},
	{"installer", ".exe .msi .msix .msixbundle .appx .appxbundle .iso .dmg .pkg .deb .rpm .apk .img .wim .esd"},
	{"archive", ".zip .rar .7z .tar .gz .tgz .bz2 .xz .zst .cab .tbz2 .txz"},
	{"document", ".pdf .txt .md .doc .docx .xls .xlsx .ppt .pptx .csv .rtf .odt .ods .odp .epub .srt .vtt .ass"},
	{"code", ".js .jsx .ts .tsx .html .css .py .go .rs .c .cpp .h .java .cs .sh .ps1 .json .jsonl .xml .yaml .yml .toml .sql .ipynb"},
	{"project", ".drp .dra .aep .aepx .prproj .blend .max .c4d .fcpxml .kdenlive .aup3 .als"},
	{"database", ".db .db3 .sqlite .sqlite3 .mdb .accdb .parquet .duckdb"},
	{"virtual", ".vhd .vhdx .vmdk .vdi .qcow2 .ova .ovf"},
}

var categoryByExtension = func() map[string]string {
	result := make(map[string]string)
	for _, category := range fileCategories {
		for _, ext := range strings.Fields(category.extensions) {
			result[ext] = category.id
		}
	}
	return result
}()

func fileCategory(name string) string {
	if category, ok := categoryByExtension[strings.ToLower(filepath.Ext(name))]; ok {
		return category
	}
	return "other"
}

func (f overviewFile) bytes(metric int) int64 {
	if metric == 1 {
		return f.Size
	}
	return f.Usage
}

func collectInventory(ctx context.Context, root fs.Item, now int64, candidates *cleanup.Result) (*fileInventory, error) {
	data := &fileInventory{oldest: now, now: now}
	linked := make(map[uint64]struct{})
	var walk func(fs.Item, ownerContext, *ownerProfile)
	walk = func(item fs.Item, ownership ownerContext, fallback *ownerProfile) {
		if ctx.Err() != nil {
			return
		}
		if item.IsDir() && !isArchiveRoot(item) {
			// Release the directory lock before looking up classifications.
			// Updating a classification after cleanup can lock the directory.
			children := slices.Collect(item.GetFilesLocked(fs.SortByName, fs.SortAsc))
			ownership = directoryOwner(item, children, ownership, candidates)
			var childFallback *ownerProfile
			for _, child := range children {
				if ctx.Err() != nil {
					break
				}
				if ownership.profile == nil && !child.IsDir() && childFallback == nil {
					childFallback = unassignedOwner(item.GetPath())
				}
				walk(child, ownership, childFallback)
			}
			return
		}
		path := item.GetPath()
		file := overviewFile{
			fileSafety: assessCleanupPath(item, path, candidates),
			Path:       path, Name: item.GetName(), Category: fileCategory(item.GetName()),
			Usage: max(0, item.GetUsage()), Size: max(0, item.GetSize()), Mtime: item.GetMtime().Unix(),
		}
		if ownership.profile == nil {
			if fallback == nil {
				fallback = unassignedOwner(filepath.Dir(path))
			}
			ownership.profile = fallback
		}
		file.Owner = ownership.profile.detail(ownership.impact(file.Grade))
		// Keep every hard-link path visible, but charge the shared bytes once
		// within this inventory, as the directory analyzer does.
		if inode := item.GetMultiLinkedInode(); inode != 0 {
			if _, found := linked[inode]; found {
				file.Usage, file.Size = 0, 0
			}
			linked[inode] = struct{}{}
		}
		if file.Mtime > 0 {
			data.oldest = min(data.oldest, file.Mtime)
		}
		data.maxBytes[0] = max(data.maxBytes[0], file.Usage)
		data.maxBytes[1] = max(data.maxBytes[1], file.Size)
		data.files = append(data.files, file)
	}
	resolver := make(fileOwnerResolver)
	walk(root, resolver.directory(root.GetParent(), candidates), nil)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for metric := range data.order {
		order := make([]int, len(data.files))
		for i := range order {
			order[i] = i
		}
		slices.SortFunc(order, func(a, b int) int {
			left, right := data.files[a], data.files[b]
			if result := cmp.Compare(right.bytes(metric), left.bytes(metric)); result != 0 {
				return result
			}
			return strings.Compare(left.Path, right.Path)
		})
		data.order[metric] = order
	}
	return data, ctx.Err()
}

func plotCell(value, maximum float64, bins int) int {
	return min(bins-1, int(math.Log1p(value)/math.Log1p(maximum)*float64(bins)))
}

func (data *fileInventory) filter(ctx context.Context, filter overviewFilter) overviewResult {
	result := overviewResult{
		Categories: []overviewCategory{}, Grades: []overviewCategory{}, Files: []overviewFile{}, Points: []overviewCell{}, Owners: []ownerGroup{},
		FileCount: len(data.files), PageSize: overviewPage, Offset: filter.offset,
		Columns: overviewColumns, Rows: overviewRows,
		MaxAgeDays: max(1, (float64(data.now)-float64(data.oldest))/secondsPerDay),
		MaxBytes:   max(1, data.maxBytes[filter.metric]),
	}
	counts := [overviewRows][overviewColumns]int{}
	categories := make(map[string]overviewCategory)
	grades := make(map[cleanupGrade]overviewCategory)
	directories := make(map[string]overviewCategory)
	owners := make(map[string]ownerGroup)
	folderPrefix := filter.folder
	if folderPrefix != "" && !strings.HasSuffix(folderPrefix, string(filepath.Separator)) {
		folderPrefix += string(filepath.Separator)
	}
	for _, i := range data.order[filter.metric] {
		if ctx.Err() != nil {
			return result
		}
		file := data.files[i]
		bytes := file.bytes(filter.metric)
		group := categories[file.Category]
		group.ID = file.Category
		group.Bytes += bytes
		group.Count++
		categories[file.Category] = group
		result.FileBytes += bytes
		if filter.category != "" && file.Category != filter.category {
			continue
		}
		relative, inFolder := strings.CutPrefix(file.Path, folderPrefix)
		if !inFolder {
			continue
		}
		ownerMatched := filter.owner == "" || (file.Owner != nil && file.Owner.Path == filter.owner)
		if ownerMatched {
			grade := grades[file.Grade]
			grade.ID = string(file.Grade)
			grade.Bytes += file.Usage
			grade.Count++
			grades[file.Grade] = grade
		}
		if !filter.grade.includes(file.Grade) {
			continue
		}
		x, y := -1, -1
		if file.Mtime > 0 {
			age := max(0, (float64(data.now)-float64(file.Mtime))/secondsPerDay)
			x = plotCell(age, result.MaxAgeDays, overviewColumns)
			y = plotCell(float64(bytes), float64(result.MaxBytes), overviewRows)
			if ownerMatched {
				counts[y][x]++
			}
		} else if ownerMatched {
			result.UndatedCount++
		}
		if box := filter.box; box != nil && (x < box[0] || x > box[1] || y < box[2] || y > box[3]) {
			continue
		}
		if filter.groupByOwner && file.Owner != nil {
			group := owners[file.Owner.Path]
			group.add(file)
			owners[file.Owner.Path] = group
		}
		if !ownerMatched {
			continue
		}
		if filter.groupByDirectory {
			directory, _, found := strings.Cut(relative, string(filepath.Separator))
			if found {
				group := directories[directory]
				group.ID = directory
				group.Bytes += file.Usage
				group.Count++
				directories[directory] = group
			}
		}
		if result.MatchedCount >= filter.offset && len(result.Files) < overviewPage {
			result.Files = append(result.Files, file)
		}
		result.MatchedCount++
		result.MatchedBytes += bytes
	}
	if filter.groupByOwner {
		groups := make([]ownerGroup, 0, len(owners))
		for _, group := range owners {
			groups = append(groups, group.summary())
		}
		slices.SortFunc(groups, compareOwnerGroups)
		result.OwnerCount = len(groups)
		result.OwnerOffset = min(filter.ownerOffset, len(groups))
		end := min(result.OwnerOffset+overviewPage, len(groups))
		result.Owners = groups[result.OwnerOffset:end]
	}
	if filter.groupByDirectory {
		groups := make([]overviewCategory, 0, len(directories))
		for _, group := range directories {
			groups = append(groups, group)
		}
		slices.SortFunc(groups, compareOverviewGroups)
		result.DirectoryCount = len(groups)
		result.DirectoryOffset = min(filter.directoryOffset, len(groups))
		end := min(result.DirectoryOffset+overviewPage, len(groups))
		result.Directories = groups[result.DirectoryOffset:end]
	}
	for _, category := range categories {
		result.Categories = append(result.Categories, category)
	}
	for _, id := range []cleanupGrade{gradeA, gradeB, gradeC, gradeD} {
		grade := grades[id]
		grade.ID = string(id)
		result.Grades = append(result.Grades, grade)
	}
	slices.SortFunc(result.Categories, compareOverviewGroups)
	for y, row := range counts {
		for x, count := range row {
			if count > 0 {
				result.Points = append(result.Points, overviewCell{X: x, Y: y, Count: count})
			}
		}
	}
	return result
}

func compareOverviewGroups(a, b overviewCategory) int {
	if size := cmp.Compare(b.Bytes, a.Bytes); size != 0 {
		return size
	}
	return strings.Compare(a.ID, b.ID)
}

func (ui *UI) fileInventory(ctx context.Context, root, scope fs.Item, gen uint64) (*fileInventory, error) {
	cache := &ui.overview
	path := scope.GetPath()
	candidates := ui.cleanupResult()
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.root == root && cache.gen == gen && cache.path == path && cache.data != nil && cache.candidates == candidates {
		return cache.data, nil
	}
	data, err := collectInventory(ctx, scope, time.Now().Unix(), candidates)
	if err != nil {
		return nil, err
	}
	cache.root, cache.gen, cache.path, cache.data = root, gen, path, data
	cache.candidates = candidates
	return data, nil
}

func parseOverviewFilter(r *http.Request) (overviewFilter, bool) {
	q := r.URL.Query()
	filter := overviewFilter{category: q.Get("category"), grade: cleanupGrade(q.Get("grade"))}
	filter.groupByDirectory = q.Get("group") == "directory"
	filter.groupByOwner = q.Get("group") == "owner"
	filter.owner = q.Get("owner")
	filter.folder = q.Get("folder")
	switch filter.grade {
	case "", gradeAB, gradeA, gradeB, gradeC, gradeD:
	default:
		return filter, false
	}
	if q.Get("metric") == "size" {
		filter.metric = 1
	}
	if raw := q.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return filter, false
		}
		filter.offset = offset
	}
	if raw := q.Get("box"); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) != 4 {
			return filter, false
		}
		box := [4]int{}
		for i, part := range parts {
			value, err := strconv.Atoi(part)
			if err != nil || value < 0 {
				return filter, false
			}
			box[i] = value
		}
		if box[0] > box[1] || box[2] > box[3] || box[1] >= overviewColumns || box[3] >= overviewRows {
			return filter, false
		}
		filter.box = &box
	}
	if raw := q.Get("directoryOffset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return filter, false
		}
		filter.directoryOffset = offset
	}
	if raw := q.Get("ownerOffset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return filter, false
		}
		filter.ownerOffset = offset
	}
	return filter, true
}

func (ui *UI) handleOverview(w http.ResponseWriter, r *http.Request) {
	filter, valid := parseOverviewFilter(r)
	if !valid {
		writeError(w, http.StatusBadRequest, "筛选范围无效")
		return
	}
	gen := ui.overviewRevision.Load()
	root, _, ready := ui.currentTree()
	if !ready || ui.removalSeq.Load()%2 != 0 {
		writeError(w, http.StatusServiceUnavailable, "扫描数据正在更新，请重试")
		return
	}
	scope, err := ui.findNode(r.URL.Query().Get("path"))
	if err != nil {
		writeNodeError(w, err)
		return
	}
	if filter.groupByDirectory || (filter.groupByOwner && filter.folder != "") {
		if filter.folder == "" {
			filter.folder = scope.GetPath()
		}
		folder, err := ui.findNode(filter.folder)
		if err != nil {
			writeNodeError(w, err)
			return
		}
		rel, err := filepath.Rel(scope.GetPath(), folder.GetPath())
		if !folder.IsDir() || isArchiveRoot(folder) || isArchiveDescendant(folder) || err != nil ||
			rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			writeError(w, http.StatusBadRequest, "请选择扫描范围内的目录")
			return
		}
		filter.folder = folder.GetPath()
	} else {
		filter.folder = ""
	}
	data, err := ui.fileInventory(r.Context(), root, scope, gen)
	if err != nil {
		return
	}
	result := data.filter(r.Context(), filter)
	if r.Context().Err() != nil {
		return
	}
	current, _, ready := ui.currentTree()
	if !ready || current != root || ui.overviewRevision.Load() != gen || ui.removalSeq.Load()%2 != 0 {
		writeError(w, http.StatusServiceUnavailable, "扫描数据正在更新，请重试")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
