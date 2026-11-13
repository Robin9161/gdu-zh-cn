package webui

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

const maxRecycleFiles = 500

type recycleEntry struct {
	Owner *fileOwner `json:"owner,omitempty"`
	Path  string     `json:"path"`
	Name  string     `json:"name"`
	Usage int64      `json:"usage"`
	Error string     `json:"error,omitempty"`
	fileSafety
	info os.FileInfo
}

type recyclePlan struct {
	ID      string         `json:"id"`
	Files   []recycleEntry `json:"files"`
	Usage   int64          `json:"usage"`
	expires time.Time
}

type recycleResult struct {
	Path  string `json:"path"`
	Error string `json:"error,omitempty"`
}

func (ui *UI) recycleEnabled(w http.ResponseWriter) bool {
	if !ui.buildStatus().RecycleAllowed {
		writeError(w, http.StatusForbidden, "此扫描当前未启用 Windows 回收站清理")
		return false
	}
	return true
}

func decodeRecycleRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "待清理清单无效")
		return false
	}
	return true
}

func (ui *UI) recycleNode(path string) (fs.Item, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("文件路径无效")
	}
	node, err := ui.findNode(path)
	if err != nil {
		return nil, errors.New("此文件已不在扫描结果中")
	}
	parent := node.GetParent()
	if node.IsDir() || parent == nil || analyze.IsVirtualRootDir(parent) || isArchiveDescendant(node) {
		return nil, errors.New("仅支持清理扫描范围内的独立文件")
	}
	if err := checkNotRelocated(node); err != nil {
		return nil, errors.New("文件所在目录已改变，无法安全清理")
	}
	return node, nil
}

// Take identity from an open handle. On Windows a path-only FileInfo can
// resolve its identity lazily, after the user has reviewed the plan.
func recycleFileInfo(node fs.Item) (os.FileInfo, error) {
	path := node.GetPath()
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("文件已不存在或无法访问")
	}
	if err := validateRecycleFile(path, info); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("无法读取文件信息，文件可能正在使用")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, errors.New("无法核对文件身份")
	}
	if err := validateRecycleFile(path, opened); err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) || opened.Size() != node.GetSize() || opened.ModTime().Unix() != node.GetMtime().Unix() {
		return nil, errors.New("文件在扫描后已变化，请重新扫描")
	}
	return opened, nil
}

func (ui *UI) recycleSafety(node fs.Item) (fileSafety, error) {
	safety := assessCleanupSafety(node, ui.cleanupResult())
	if safety.Grade != gradeD {
		// A scan can start from a directory alias. Apply protected path rules
		// to its destination as well as to the path displayed in the list.
		parent, err := filepath.EvalSymlinks(filepath.Dir(node.GetPath()))
		if err != nil {
			return safety, errors.New("无法核对文件所在目录")
		}
		if reason := protectedReason(pathParts(filepath.Join(parent, node.GetName())), fileCategory(node.GetName())); reason != "" {
			safety = fileSafety{gradeD, reason}
		}
	}
	if safety.Grade == gradeD {
		return safety, errors.New("D 级文件禁止移入回收站")
	}
	return safety, nil
}

func (ui *UI) handleRecyclePreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Paths []string `json:"paths"`
	}
	if !decodeRecycleRequest(w, r, &body) {
		return
	}
	if len(body.Paths) == 0 || len(body.Paths) > maxRecycleFiles {
		writeError(w, http.StatusBadRequest, "每批请选择 1 至 500 个文件")
		return
	}

	if !ui.actionMu.TryLock() {
		writeError(w, http.StatusConflict, "已有操作正在处理，请稍后重新核对")
		return
	}
	defer ui.actionMu.Unlock()
	if !ui.recycleEnabled(w) {
		return
	}

	plan := recyclePlan{ID: generateActionToken(), expires: time.Now().Add(15 * time.Minute)}
	seen := make(map[string]bool)
	owners := make(fileOwnerResolver)
	candidates := ui.cleanupResult()
	for _, path := range body.Paths {
		if r.Context().Err() != nil {
			return
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		entry := recycleEntry{Path: path, Name: filepath.Base(path), fileSafety: fileSafety{gradeD, "无法核对扫描记录，暂不允许清理"}}
		node, err := ui.recycleNode(path)
		if err == nil {
			entry.fileSafety, err = ui.recycleSafety(node)
			entry.Owner = owners.describe(node, entry.fileSafety, candidates)
		}
		if err == nil {
			entry.info, err = recycleFileInfo(node)
		}
		if err != nil {
			entry.Error = err.Error()
		} else {
			entry.Usage = node.GetUsage()
			plan.Usage += entry.Usage
		}
		plan.Files = append(plan.Files, entry)
	}
	ui.recyclePlan = &plan
	writeJSON(w, http.StatusOK, plan)
}

func (ui *UI) handleRecycleConfirm(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if !decodeRecycleRequest(w, r, &body) {
		return
	}

	if !ui.actionMu.TryLock() {
		writeError(w, http.StatusConflict, "已有操作正在处理，请稍后重新核对")
		return
	}
	defer ui.actionMu.Unlock()
	if !ui.recycleEnabled(w) {
		return
	}
	plan := ui.recyclePlan
	if plan == nil || body.ID != plan.ID || time.Now().After(plan.expires) {
		writeError(w, http.StatusConflict, "待清理清单已失效，请重新核对")
		return
	}
	ui.recyclePlan = nil

	results := make([]recycleResult, 0, len(plan.Files))
	for _, entry := range plan.Files {
		result := recycleResult{Path: entry.Path, Error: entry.Error}
		if result.Error == "" {
			if r.Context().Err() != nil {
				result.Error = "连接中断，此文件尚未处理"
			} else if err := ui.recycleReviewedFile(entry); err != nil {
				result.Error = err.Error()
			}
		}
		results = append(results, result)
	}
	writeJSON(w, http.StatusOK, results)
}

func (ui *UI) recycleReviewedFile(entry recycleEntry) error {
	node, err := ui.recycleNode(entry.Path)
	if err != nil {
		return err
	}
	if _, err := ui.recycleSafety(node); err != nil {
		return err
	}
	info, err := recycleFileInfo(node)
	if err != nil {
		return err
	}
	if !os.SameFile(entry.info, info) || entry.info.Size() != info.Size() || !entry.info.ModTime().Equal(info.ModTime()) {
		return errors.New("文件在确认前已更改或被替换")
	}
	removed, err := ui.removeNode(func(parent, item fs.Item) error {
		if err := ui.recycleToBin(item.GetPath()); err != nil {
			return err
		}
		parent.RemoveFile(item)
		return nil
	}, node.GetParent(), node)
	if err == nil && !removed {
		return errors.New("未能确认文件已移入回收站")
	}
	return err
}
