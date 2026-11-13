package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalRescanValidatesPathAndReplacesRoot(t *testing.T) {
	ui := newTestUI()
	ui.SetRecycleOnly(true)
	old := makeTree(t)
	next := makeTree(t)
	scan(t, ui, old)
	revision := ui.buildStatus().ScanRevision
	ui.recyclePlan = &recyclePlan{ID: "previous-scan"}

	routes := ui.routes()
	request := func(method, route, path, token string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]string{"path": path})
		require.NoError(t, err)
		req := httptest.NewRequest(method, "http://127.0.0.1"+route, bytes.NewReader(body))
		req.RemoteAddr = "127.0.0.1:5678"
		req.Header.Set(actionHeader, token)
		recorder := httptest.NewRecorder()
		routes.ServeHTTP(recorder, req)
		return recorder
	}
	require.Equal(t, http.StatusForbidden, request("POST", "/api/v1/scan", next, "").Code)
	require.Equal(t, http.StatusBadRequest, request("POST", "/api/v1/scan", "relative", ui.actionToken).Code)
	require.Equal(t, http.StatusBadRequest, request("POST", "/api/v1/scan", filepath.Join(next, "small.txt"), ui.actionToken).Code)
	require.Equal(t, revision, ui.buildStatus().ScanRevision)
	require.NotNil(t, ui.recyclePlan)

	res := request("POST", "/api/v1/scan", next, ui.actionToken)
	require.Equal(t, http.StatusAccepted, res.Code, res.Body.String())
	waitClassified(t, ui)
	expected, err := filepath.EvalSymlinks(next)
	require.NoError(t, err)
	require.Equal(t, expected, ui.buildStatus().RootPath)
	require.Equal(t, revision+1, ui.buildStatus().ScanRevision)
	require.Nil(t, ui.recyclePlan)
	_, err = ui.findNode(filepath.Join(old, "small.txt"))
	require.Error(t, err)
	_, err = ui.findNode(filepath.Join(expected, "small.txt"))
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(next, "small.txt"))

	drive := filepath.VolumeName(os.TempDir()) + "\\"
	list := request("GET", "/api/v1/scan/drives", "", ui.actionToken)
	require.Equal(t, http.StatusOK, list.Code)
	require.Contains(t, list.Body.String(), drive[:2])
}
