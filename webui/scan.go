package webui

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
)

var errScanRunning = errors.New("已有扫描正在进行")

type scanDrive struct {
	Path string `json:"path"`
}

func (ui *UI) scanEnabled(w http.ResponseWriter) bool {
	w.Header().Set("Cache-Control", "no-store")
	if !ui.buildStatus().ScanAllowed {
		writeError(w, http.StatusForbidden, "此实例未启用本地扫描")
		return false
	}
	return true
}

func (ui *UI) handleScanDrives(w http.ResponseWriter, _ *http.Request) {
	if !ui.scanEnabled(w) {
		return
	}
	drives, err := localScanDrives()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "无法读取盘符，请直接输入文件夹路径")
		return
	}
	writeJSON(w, http.StatusOK, drives)
}

func (ui *UI) handleScanFolders(w http.ResponseWriter, r *http.Request) {
	if !ui.scanEnabled(w) {
		return
	}
	path, err := localScanPath(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "无法读取此文件夹")
		return
	}
	parent := filepath.Dir(path)
	if parent == path {
		parent = ""
	}
	result := struct {
		Path    string   `json:"path"`
		Parent  string   `json:"parent"`
		Folders []string `json:"folders"`
	}{path, parent, []string{}}
	for _, entry := range entries {
		if entry.IsDir() {
			result.Folders = append(result.Folders, entry.Name())
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (ui *UI) handleScan(w http.ResponseWriter, r *http.Request) {
	if !ui.scanEnabled(w) {
		return
	}
	var request struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "扫描路径无效")
		return
	}

	if !ui.actionMu.TryLock() {
		writeError(w, http.StatusConflict, "已有操作正在处理，请稍后再试")
		return
	}
	defer ui.actionMu.Unlock()
	if ui.buildStatus().State == scanStateScanning {
		writeError(w, http.StatusConflict, errScanRunning.Error())
		return
	}
	path, err := localScanPath(request.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := ui.analyzePath(path, nil, false); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	ui.recyclePlan = nil
	writeJSON(w, http.StatusAccepted, ui.buildStatus())
}

func (ui *UI) handleScanCancel(w http.ResponseWriter, _ *http.Request) {
	if !ui.scanEnabled(w) {
		return
	}
	ui.mu.Lock()
	if ui.scanning && ui.scanTarget != "" && !ui.scanCancelled && ui.snapshotJob.Busy != "restore" {
		ui.scanCancelled = true
		ui.Analyzer.Cancel()
	}
	ui.mu.Unlock()
	ui.hub.publish(ui.statusJSON())
	writeJSON(w, http.StatusAccepted, ui.buildStatus())
}
