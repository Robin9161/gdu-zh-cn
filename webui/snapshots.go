package webui

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/dundee/gdu/v5/report"
)

const snapshotVersion = 1

type snapshotMetadata struct {
	Version  int       `json:"version"`
	RootPath string    `json:"rootPath"`
	ScanTime time.Time `json:"scanTime"`
	SavedAt  time.Time `json:"savedAt"`
	Filtered bool      `json:"filtered,omitempty"`
}

type snapshotJob struct {
	Busy    string `json:"busy,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type snapshotStatus struct {
	snapshotJob
	Saved      *snapshotMetadata `json:"saved,omitempty"`
	SavedError string            `json:"savedError,omitempty"`
	CanSave    bool              `json:"canSave"`
}

func (ui *UI) snapshotEnabled() bool {
	return ui.recycleOnly && windowsRecycleSupported && isLoopbackHost(ui.listenAddr) && !ui.archiveBrowsing
}

func (ui *UI) snapshotFile() (string, error) {
	if ui.snapshotPath != "" {
		return ui.snapshotPath, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(executable), "state", "latest-scan.json.gz"), nil
}

func startupRestoreEnabled() bool {
	return os.Getenv("GDU_WEB_RESTORE_LATEST") == "1"
}

func (ui *UI) snapshotAvailable(w http.ResponseWriter) bool {
	w.Header().Set("Cache-Control", "no-store")
	if !ui.snapshotEnabled() {
		writeError(w, http.StatusForbidden, "当前扫描模式不支持保存结果")
		return false
	}
	return true
}

func (ui *UI) handleSnapshotStatus(w http.ResponseWriter, _ *http.Request) {
	if !ui.snapshotAvailable(w) {
		return
	}
	writeJSON(w, http.StatusOK, ui.getSnapshotStatus())
}

func (ui *UI) getSnapshotStatus() snapshotStatus {
	ui.mu.RLock()
	status := snapshotStatus{
		snapshotJob: ui.snapshotJob,
		CanSave:     ui.canSaveSnapshotLocked(),
	}
	ui.mu.RUnlock()
	if status.Busy != "" {
		return status
	}

	path, err := ui.snapshotFile()
	if err == nil {
		var file *os.File
		var reader *gzip.Reader
		file, reader, status.Saved, err = openSnapshot(path)
		if err == nil {
			reader.Close()
			file.Close()
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		status.SavedError = "无法读取上一次保存的扫描"
	}
	return status
}

func (ui *UI) canSaveSnapshotLocked() bool {
	return !ui.scanning && ui.scanErr == nil && !ui.scanTime.IsZero() &&
		ui.topDir != nil && !analyze.IsVirtualRootDir(ui.topDir) && filepath.IsAbs(ui.topDirPath)
}

func (ui *UI) handleSnapshotAction(w http.ResponseWriter, r *http.Request) {
	if !ui.snapshotAvailable(w) {
		return
	}
	var request struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&request); err != nil ||
		(request.Action != "save" && request.Action != "restore") {
		writeError(w, http.StatusBadRequest, "请选择保存或恢复扫描")
		return
	}
	if !ui.actionMu.TryLock() {
		writeError(w, http.StatusConflict, "已有操作正在处理，请稍后再试")
		return
	}
	workerOwnsLock := false
	defer func() {
		if !workerOwnsLock {
			ui.actionMu.Unlock()
		}
	}()
	ui.mu.Lock()
	if ui.scanning || ui.snapshotJob.Busy != "" {
		ui.mu.Unlock()
		writeError(w, http.StatusConflict, "请等待当前扫描或操作结束")
		return
	}
	if request.Action == "save" && !ui.canSaveSnapshotLocked() {
		ui.mu.Unlock()
		writeError(w, http.StatusConflict, "完成一次单目录扫描后即可保存")
		return
	}
	root := ui.topDir
	metadata := snapshotMetadata{
		Version: snapshotVersion, RootPath: ui.topDirPath, ScanTime: ui.scanTime,
		SavedAt: time.Now().UTC(), Filtered: ui.snapshotFiltered || ui.IsFilteringFiles(),
	}
	action := request.Action
	ui.snapshotJob = snapshotJob{Busy: action}
	ui.mu.Unlock()
	ui.hub.publish(ui.statusJSON())

	// Hand the action lock to the worker. HTTP requests return immediately
	// even when serializing a disk with millions of files.
	workerOwnsLock = true
	go ui.runSnapshotJob(action, root, metadata)
	writeJSON(w, http.StatusAccepted, snapshotStatus{snapshotJob: snapshotJob{Busy: action}})
}

func (ui *UI) runSnapshotJob(action string, root fs.Item, metadata snapshotMetadata) {
	defer ui.actionMu.Unlock()
	var message, failure string
	path, err := ui.snapshotFile()
	if err == nil {
		switch action {
		case "save":
			err = writeSnapshot(path, root, metadata)
			message, failure = "扫描结果已保存", "保存失败，请检查磁盘空间或目录写入权限"
		case "restore":
			var dir *analyze.Dir
			var saved *snapshotMetadata
			dir, saved, err = readSnapshot(path)
			if err == nil {
				ui.installSnapshot(dir, saved)
			}
			message, failure = "已恢复保存的扫描", "恢复失败，请重新扫描并保存"
			if errors.Is(err, os.ErrNotExist) {
				failure = "暂无可恢复的扫描"
			}
		}
	} else {
		failure = "无法定位扫描保存目录"
	}
	ui.finishSnapshotJob(message, failure, err)
}

func (ui *UI) tryStartupSnapshot(root string) bool {
	path, err := ui.snapshotFile()
	if err != nil {
		return false
	}
	file, reader, metadata, err := openSnapshot(path)
	if err != nil {
		return false
	}
	defer file.Close()
	defer reader.Close()

	root, err = filepath.Abs(root)
	if err != nil || !sameSnapshotRoot(metadata.RootPath, root) || metadata.Filtered || ui.IsFilteringFiles() {
		return false
	}
	ui.mu.Lock()
	if ui.scanCancelled {
		ui.mu.Unlock()
		return false
	}
	ui.snapshotJob = snapshotJob{Busy: "restore"}
	ui.mu.Unlock()
	ui.hub.publish(ui.statusJSON())

	dir, err := decodeSnapshot(reader, metadata)
	if err != nil {
		ui.finishSnapshotJob("", "保存的扫描无法恢复，正在重新扫描", err)
		return false
	}
	ui.installSnapshot(dir, metadata)
	ui.finishSnapshotJob("已恢复保存的扫描", "", nil)
	return true
}

func (ui *UI) installSnapshot(dir *analyze.Dir, metadata *snapshotMetadata) {
	ui.mu.Lock()
	ui.topDir = dir
	ui.topDirPath = dir.GetPath()
	ui.scanning = false
	ui.scanTarget = ""
	ui.scanCancelled = false
	ui.scanErr = nil
	ui.progress = common.CurrentProgress{}
	ui.scanRevision++
	ui.scanTime = metadata.ScanTime
	ui.restoredSnapshot = true
	ui.snapshotFiltered = metadata.Filtered
	ui.recyclePlan = nil
	ui.overviewRevision.Add(1)
	ui.treeGen.Add(1)
	ui.mu.Unlock()
	ui.classify()
}

func (ui *UI) finishSnapshotJob(message, failure string, err error) {
	ui.mu.Lock()
	if err != nil {
		log.Printf("webui: snapshot: %s", err)
		ui.snapshotJob = snapshotJob{Error: failure}
	} else {
		ui.snapshotJob = snapshotJob{Message: message}
	}
	ui.mu.Unlock()
	ui.hub.publish(ui.statusJSON())
}

func sameSnapshotRoot(first, second string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(first), filepath.Clean(second))
	}
	return filepath.Clean(first) == filepath.Clean(second)
}

func openSnapshot(path string) (*os.File, *gzip.Reader, *snapshotMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	reader, err := gzip.NewReader(file)
	if err != nil {
		file.Close()
		return nil, nil, nil, err
	}
	var metadata snapshotMetadata
	if err := json.Unmarshal(reader.Header.Extra, &metadata); err != nil ||
		metadata.Version != snapshotVersion || !filepath.IsAbs(metadata.RootPath) ||
		metadata.ScanTime.IsZero() || metadata.SavedAt.IsZero() {
		reader.Close()
		file.Close()
		return nil, nil, nil, errors.New("invalid snapshot metadata")
	}
	return file, reader, &metadata, nil
}

func readSnapshot(path string) (*analyze.Dir, *snapshotMetadata, error) {
	file, reader, metadata, err := openSnapshot(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	defer reader.Close()
	dir, err := decodeSnapshot(reader, metadata)
	return dir, metadata, err
}

func decodeSnapshot(reader *gzip.Reader, metadata *snapshotMetadata) (*analyze.Dir, error) {
	dir, err := report.ReadAnalysis(reader)
	if err != nil {
		return nil, err
	}
	if !sameSnapshotRoot(dir.GetPath(), metadata.RootPath) {
		return nil, errors.New("report root does not match snapshot metadata")
	}
	if metadata.Filtered {
		dir.UpdateStatsWithFileFiltering(make(fs.HardLinkedItems))
	} else {
		dir.UpdateStats(make(fs.HardLinkedItems))
	}
	return dir, nil
}

func writeSnapshot(path string, root fs.Item, metadata snapshotMetadata) error {
	header, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	folder := filepath.Dir(path)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(folder, ".scan-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()

	compressor, err := gzip.NewWriterLevel(temp, gzip.BestSpeed)
	if err != nil {
		return err
	}
	compressor.Header.Extra = header
	writer := bufio.NewWriterSize(compressor, 64<<10)
	if _, err := fmt.Fprint(writer, "[1,2,{},\n"); err != nil {
		return err
	}
	if err := root.EncodeJSON(writer, true, nil); err != nil {
		return err
	}
	if _, err := fmt.Fprint(writer, "]\n"); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if err := compressor.Close(); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}
