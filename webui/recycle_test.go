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

	"github.com/stretchr/testify/require"
)

func TestRecycleRequiresReviewAndRejectsReplacedFile(t *testing.T) {
	root := cleanupFixtureDir(t)
	first := filepath.Join(root, "big.bin")
	second := filepath.Join(root, "small.txt")
	require.NoError(t, os.WriteFile(first, make([]byte, 4096), 0o600))
	require.NoError(t, os.WriteFile(second, []byte("fixture"), 0o600))
	protected := filepath.Join(root, "auth.json")
	require.NoError(t, os.WriteFile(protected, []byte("fixture"), 0o600))
	ui := newTestUI()
	ui.SetRecycleOnly(true)
	scan(t, ui, root)
	routes := ui.routes()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o600))

	var moved []string
	ui.recycleToBin = func(path string) error {
		moved = append(moved, path)
		return nil
	}

	request := func(method, route string, data any, token string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(data)
		require.NoError(t, err)
		req := httptest.NewRequest(method, "http://127.0.0.1"+route, bytes.NewReader(body))
		req.RemoteAddr = "127.0.0.1:5678"
		req.Header.Set(actionHeader, token)
		recorder := httptest.NewRecorder()
		routes.ServeHTTP(recorder, req)
		return recorder
	}

	preview := "/api/v1/recycle/preview"
	confirm := "/api/v1/recycle/confirm"
	require.Equal(t, http.StatusForbidden, request("POST", preview, nil, "").Code)
	require.Equal(t, http.StatusForbidden, request("DELETE", "/api/v1/nodes", nil, ui.actionToken).Code)
	require.Equal(t, http.StatusConflict, request("POST", confirm, map[string]string{"id": "unreviewed"}, ui.actionToken).Code)
	require.Empty(t, moved)

	recorder := request("POST", preview, map[string][]string{"paths": {first, first, second, outside, protected}}, ui.actionToken)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var plan recyclePlan
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &plan))
	require.Len(t, plan.Files, 4)
	require.Positive(t, plan.Usage)
	require.Equal(t, gradeC, plan.Files[0].Grade)
	require.NotEmpty(t, plan.Files[0].Reason)
	require.NotEmpty(t, plan.Files[2].Error)
	require.Equal(t, gradeD, plan.Files[3].Grade)
	require.NotEmpty(t, plan.Files[3].Reason)
	require.Contains(t, plan.Files[3].Error, "D 级")
	require.Zero(t, plan.Files[3].Usage)
	require.Empty(t, moved)
	// Exercise the confirm-time check independently of the preview rejection.
	node, err := ui.findNode(protected)
	require.NoError(t, err)
	protectedInfo, err := recycleFileInfo(node)
	require.NoError(t, err)
	require.ErrorContains(t, ui.recycleReviewedFile(recycleEntry{Path: protected, info: protectedInfo}), "D 级")
	require.Empty(t, moved)

	original, err := os.Stat(second)
	require.NoError(t, err)
	require.NoError(t, os.Rename(second, filepath.Join(root, "original.txt")))
	require.NoError(t, os.WriteFile(second, make([]byte, original.Size()), 0o600))
	require.NoError(t, os.Chtimes(second, original.ModTime(), original.ModTime()))

	payload := map[string]string{"id": plan.ID}
	recorder = request("POST", confirm, payload, ui.actionToken)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var results []recycleResult
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &results))
	require.Len(t, results, 4)
	require.Empty(t, results[0].Error)
	require.NotEmpty(t, results[1].Error)
	require.NotEmpty(t, results[2].Error)
	require.NotEmpty(t, results[3].Error)
	require.Equal(t, []string{first}, moved)
	require.Equal(t, http.StatusConflict, request("POST", confirm, payload, ui.actionToken).Code)
}
