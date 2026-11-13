package report

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/dundee/gdu/v5/pkg/analyze"
)

// ReadAnalysis reads analysis report from JSON file and returns directory item
func ReadAnalysis(input io.Reader) (dir *analyze.Dir, err error) {
	decoder := json.NewDecoder(input)
	token, err := decoder.Token()
	if err == io.EOF {
		return nil, errors.New("unexpected end of JSON input")
	}
	if err != nil {
		return nil, err
	}
	if token != json.Delim('[') {
		return nil, errors.New("JSON file does not contain top level array")
	}
	for range 3 {
		if !decoder.More() {
			return nil, errors.New("top level array must have at least 4 items")
		}
		var header json.RawMessage
		if err := decoder.Decode(&header); err != nil {
			return nil, err
		}
	}
	if !decoder.More() {
		return nil, errors.New("top level array must have at least 4 items")
	}
	token, err = decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('[') {
		return nil, errors.New("array of maps not found in the top level array on 4th position")
	}
	dir, err = processDir(decoder)
	if err != nil {
		return nil, err
	}
	for decoder.More() {
		var extra json.RawMessage
		if err := decoder.Decode(&extra); err != nil {
			return nil, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	// Reach EOF so invalid trailing input and compressed-stream checksum
	// failures are detected before publishing the tree.
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			err = errors.New("unexpected data after analysis")
		}
		return nil, err
	}
	return dir, nil
}

// The opening array delimiter has already been consumed. Decode each item
// into the tree as it arrives instead of holding another tree of JSON maps.
func processDir(decoder *json.Decoder) (*analyze.Dir, error) {
	if !decoder.More() {
		return nil, errors.New("directory array is empty")
	}
	var metadata any
	if err := decoder.Decode(&metadata); err != nil {
		return nil, err
	}
	dir, hasSize, hasUsage, hasItemCount, err := parseDirectoryMetadata(metadata)
	if err != nil {
		return nil, err
	}

	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch token {
		case json.Delim('{'):
			file, err := processFile(decoder, dir)
			if err != nil {
				return nil, err
			}
			dir.AddFile(file)
		case json.Delim('['):
			subdir, err := processDir(decoder)
			if err != nil {
				return nil, err
			}
			subdir.Parent = dir
			dir.AddFile(subdir)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if hasUsage {
		dir.OwnUsage = exportedOwnUsage(dir)
	}
	preserveTruncatedStats(dir, hasSize, hasUsage, hasItemCount)

	return dir, nil
}

func processFile(decoder *json.Decoder, parent *analyze.Dir) (*analyze.File, error) {
	file := &analyze.File{Parent: parent, Flag: ' '}
	var hasName, notRegular, hardLinked bool
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		number, _ := value.(float64)
		switch key {
		case "name":
			file.Name, hasName = value.(string)
		case "asize":
			file.Size = int64(number)
		case "dsize":
			file.Usage = int64(number)
		case "shsize":
			file.SharedUsage = int64(number)
		case "mtime":
			file.Mtime = time.Time{}
			if _, ok := value.(float64); ok {
				file.Mtime = time.Unix(int64(number), 0)
			}
		case "notreg":
			_, notRegular = value.(bool)
		case "ino":
			file.Mli = uint64(number)
		case "hlnkc":
			_, hardLinked = value.(bool)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if !hasName {
		return nil, errors.New("file name is not a string")
	}
	if notRegular {
		file.Flag = '@'
	}
	if hardLinked {
		file.Flag = 'H'
	}
	return file, nil
}

// exportedOwnUsage recovers the disk usage of the directory itself, which the
// export folds into the directory's total. A hard-linked file counted elsewhere
// in the tree can make the entries add up to more than the total, so the
// result is clamped at zero rather than going negative.
func exportedOwnUsage(dir *analyze.Dir) int64 {
	own := dir.Usage
	for _, item := range dir.Files {
		own -= item.GetUsage()
	}
	return max(own, 0)
}

func parseDirectoryMetadata(item any) (
	dir *analyze.Dir,
	hasSize bool,
	hasUsage bool,
	hasItemCount bool,
	err error,
) {
	dirMap, ok := item.(map[string]any)
	if !ok {
		return nil, false, false, false, errors.New("directory item is not a map")
	}
	name, ok := dirMap["name"].(string)
	if !ok {
		return nil, false, false, false, errors.New("directory name is not a string")
	}

	dir = &analyze.Dir{File: &analyze.File{Flag: ' '}}
	if mtime, ok := dirMap["mtime"].(float64); ok {
		dir.Mtime = time.Unix(int64(mtime), 0)
	}
	if readError, ok := dirMap["read_error"].(bool); ok && readError {
		dir.Flag = '!'
	}
	if asize, ok := dirMap["asize"].(float64); ok {
		dir.Size = int64(asize)
		hasSize = true
	}
	if dsize, ok := dirMap["dsize"].(float64); ok {
		dir.Usage = int64(dsize)
		hasUsage = true
	}
	if itemCount, ok := dirMap["items"].(float64); ok {
		dir.ItemCount = int64(itemCount)
		hasItemCount = true
	}

	slashPos := strings.LastIndexByte(name, '/')
	if filepath.Separator != '/' {
		slashPos = max(slashPos, strings.LastIndexByte(name, filepath.Separator))
	}
	if slashPos > -1 {
		dir.Name = name[slashPos+1:]
		dir.BasePath = name[:slashPos+1]
	} else {
		dir.Name = name
	}

	return dir, hasSize, hasUsage, hasItemCount, nil
}

func preserveTruncatedStats(dir *analyze.Dir, hasSize, hasUsage, hasItemCount bool) {
	if !hasSize || !hasUsage || !hasItemCount || len(dir.Files) != 0 {
		return
	}
	// A leaf directory whose imported stats match the empty-dir placeholder
	// carries nothing worth pinning: recomputation reproduces the same values.
	// Only genuinely-empty leaves reach this point with placeholder stats;
	// depth-truncated directories that held content export with items >= 2.
	if dir.HasEmptyDirPlaceholderStats() {
		return
	}
	dir.SetStatsFromJSON()
}
