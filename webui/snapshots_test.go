//go:build windows

package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/analyze"
)

func TestSnapshotWindowsVolumeRootRoundTrip(t *testing.T) {
	dir := &analyze.Dir{File: &analyze.File{Name: `\`}, BasePath: `C:\`}
	dir.AddFile(&analyze.File{Name: "item.txt", Parent: dir, Size: 12, Usage: 4096})
	dir.UpdateStats(nil)
	require.Equal(t, `C:\`, dir.GetPath())
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "snapshot.json.gz")
	require.NoError(t, writeSnapshot(path, dir, snapshotMetadata{
		Version: snapshotVersion, RootPath: dir.GetPath(), ScanTime: now, SavedAt: now,
	}))
	restored, metadata, err := readSnapshot(path)
	require.NoError(t, err)
	require.Equal(t, dir.GetPath(), metadata.RootPath)
	require.Equal(t, dir.GetPath(), restored.GetPath())
	require.Len(t, restored.Files, 1)
	require.Equal(t, `C:\item.txt`, restored.Files[0].GetPath())
	require.Equal(t, int64(4096), restored.GetUsage())
}

func snapshotRequestForTest(t *testing.T, ui *UI, method, route string, payload any, token string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(method, "http://127.0.0.1"+route, bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5678"
	req.Header.Set(actionHeader, token)
	res := httptest.NewRecorder()
	ui.routes().ServeHTTP(res, req)
	return res
}

func waitSnapshot(t *testing.T, ui *UI) snapshotStatus {
	t.Helper()
	var status snapshotStatus
	require.Eventually(t, func() bool {
		if !ui.actionMu.TryLock() {
			return false
		}
		defer ui.actionMu.Unlock()
		status = ui.getSnapshotStatus()
		return status.Busy == ""
	}, 5*time.Second, 20*time.Millisecond)
	return status
}

func TestSnapshotSaveReplaceAndRestore(t *testing.T) {
	t.Setenv("GDU_WEB_RESTORE_LATEST", "")
	ui := newTestUI()
	ui.SetRecycleOnly(true)
	ui.snapshotPath = filepath.Join(t.TempDir(), "state", "latest-scan.json.gz")
	first := cleanupFixtureDir(t)
	second := cleanupFixtureDir(t)
	existing := filepath.Join(first, "existing.txt")
	changed := filepath.Join(first, "changed.txt")
	require.NoError(t, os.WriteFile(existing, []byte("original"), 0o600))
	require.NoError(t, os.WriteFile(changed, []byte("original"), 0o600))

	scan(t, ui, second)
	const route = "/api/v1/snapshot"
	request := func(action, token string) *httptest.ResponseRecorder {
		return snapshotRequestForTest(t, ui, http.MethodPost, route, map[string]string{"action": action}, token)
	}
	require.Equal(t, http.StatusForbidden, request("save", "").Code)
	ui.actionMu.Lock()
	conflict := request("save", ui.actionToken)
	blockedPreview := snapshotRequestForTest(t, ui, http.MethodPost, "/api/v1/recycle/preview",
		map[string][]string{"paths": {existing}}, ui.actionToken)
	blockedConfirm := snapshotRequestForTest(t, ui, http.MethodPost, "/api/v1/recycle/confirm",
		map[string]string{"id": "previous-plan"}, ui.actionToken)
	ui.actionMu.Unlock()
	require.Equal(t, http.StatusConflict, conflict.Code)
	require.Equal(t, http.StatusConflict, blockedPreview.Code)
	require.Equal(t, http.StatusConflict, blockedConfirm.Code)
	require.Equal(t, http.StatusAccepted, request("save", ui.actionToken).Code)
	firstSave := waitSnapshot(t, ui)
	require.Empty(t, firstSave.Error)
	require.Equal(t, second, firstSave.Saved.RootPath)

	scan(t, ui, first)
	scanTime := ui.buildStatus().ScanTime
	require.Equal(t, http.StatusAccepted, request("save", ui.actionToken).Code)
	secondSave := waitSnapshot(t, ui)
	require.Empty(t, secondSave.Error)
	require.Equal(t, first, secondSave.Saved.RootPath)
	require.Equal(t, scanTime, secondSave.Saved.ScanTime)

	scan(t, ui, second)
	prior := ui.buildStatus()
	require.NoError(t, os.WriteFile(changed, []byte("longer than original"), 0o600))
	require.Equal(t, http.StatusAccepted, request("restore", ui.actionToken).Code)
	require.Empty(t, waitSnapshot(t, ui).Error)
	waitClassified(t, ui)
	restored := ui.buildStatus()
	require.Equal(t, first, restored.RootPath)
	require.True(t, restored.RestoredSnapshot)
	require.Equal(t, scanTime, restored.ScanTime)
	require.Greater(t, restored.ScanRevision, prior.ScanRevision)
	require.Greater(t, restored.TreeRevision, prior.TreeRevision)
	require.True(t, restored.RecycleAllowed)

	response := snapshotRequestForTest(t, ui, http.MethodPost, "/api/v1/recycle/preview",
		map[string][]string{"paths": {existing, changed}}, ui.actionToken)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var plan recyclePlan
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &plan))
	require.Len(t, plan.Files, 2)
	require.Empty(t, plan.Files[0].Error)
	require.Contains(t, plan.Files[1].Error, "文件在扫描后已变化")

	require.NoError(t, os.WriteFile(ui.snapshotPath, []byte("invalid"), 0o600))
	require.Equal(t, http.StatusAccepted, request("restore", ui.actionToken).Code)
	require.NotEmpty(t, waitSnapshot(t, ui).Error)
	require.Equal(t, restored, ui.buildStatus())

	require.Equal(t, http.StatusAccepted, request("save", ui.actionToken).Code)
	require.Empty(t, waitSnapshot(t, ui).Error)
	restarter := newTestUI()
	restarter.SetRecycleOnly(true)
	restarter.snapshotPath = ui.snapshotPath
	t.Setenv("GDU_WEB_RESTORE_LATEST", "1")
	scan(t, restarter, first)
	startup := restarter.buildStatus()
	require.True(t, startup.RestoredSnapshot)
	require.Equal(t, scanTime, startup.ScanTime)
	require.Equal(t, first, startup.RootPath)

	response = snapshotRequestForTest(t, restarter, http.MethodPost, "/api/v1/scan",
		map[string]string{"path": first}, restarter.actionToken)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	waitClassified(t, restarter)
	fresh := restarter.buildStatus()
	require.False(t, fresh.RestoredSnapshot)
	require.Greater(t, fresh.ScanRevision, startup.ScanRevision)
	require.True(t, fresh.ScanTime.After(scanTime))
}
