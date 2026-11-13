package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOverviewOwnerGroupsAndExactFiltering(t *testing.T) {
	root := cleanupFixtureDir(t)
	files := map[string]string{
		"project/.env":         "fixture",
		"project/package.json": "{}",
		"project/node_modules/electron/package.json":        "{}",
		"project/node_modules/electron/electron.exe":        "electron",
		"project/.venv/pkg/__pycache__/foo.cpython-312.pyc": "pyc",
		"project/child/package.json":                        "{}",
		"project/child/node_modules/test/child.js":          "js",
		".workbuddy/binaries/package.json":                  "{}",
		".workbuddy/binaries/node_modules/test/tool.exe":    "tool",
		"unknown/loose.txt":                                 "unowned",
	}
	for path, contents := range files {
		fullPath := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o700))
		require.NoError(t, os.WriteFile(fullPath, []byte(contents), 0o600))
	}
	ui := newTestUI()
	scan(t, ui, root)
	handler := ui.routes()
	request := func(query string) overviewResult {
		t.Helper()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/overview?"+query, nil))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var result overviewResult
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
		return result
	}

	result := request("grade=AB&group=owner")
	require.Equal(t, 3, result.OwnerCount)
	require.Len(t, result.Owners, 3)
	parent := result.Owners[0]
	require.Equal(t, filepath.Join(root, "project"), parent.Path)
	require.Equal(t, "project", parent.Kind)
	require.Contains(t, parent.Evidence, "package.json")
	require.Equal(t, 3, parent.Count)
	require.Positive(t, parent.ABytes)
	require.Positive(t, parent.BBytes)
	require.Equal(t, parent.Bytes, parent.ABytes+parent.BBytes)
	require.Equal(t, result.MatchedBytes, parent.Bytes+result.Owners[1].Bytes+result.Owners[2].Bytes)

	exact := request("grade=AB&group=owner&owner=" + url.QueryEscape(parent.Path))
	require.Equal(t, parent.Count, exact.MatchedCount)
	require.Len(t, exact.Files, parent.Count)
	for _, file := range exact.Files {
		require.Equal(t, parent.Path, file.Owner.Path)
		require.NotEmpty(t, file.Owner.Impact)
		require.NotEmpty(t, file.Owner.Recovery)
	}
	require.Equal(t, result.OwnerCount, exact.OwnerCount)
	require.Empty(t, request("grade=AB&group=owner&owner="+url.QueryEscape(filepath.Join(root, "pro"))).Files)
	require.Equal(t, []int{1, 2, 1, 1}, []int{
		exact.Grades[0].Count, exact.Grades[1].Count, exact.Grades[2].Count, exact.Grades[3].Count,
	})
	require.Equal(t, parent.ABytes, exact.Grades[0].Bytes)
	require.Equal(t, parent.BBytes, exact.Grades[1].Bytes)

	softwarePath := filepath.Join(root, ".workbuddy")
	software := request("grade=AB&group=owner&owner=" + url.QueryEscape(softwarePath))
	require.Len(t, software.Files, 1)
	require.Equal(t, "software", software.Files[0].Owner.Kind)
	require.Contains(t, software.Files[0].Owner.Impact, "无法启动")

	all := request("group=owner")
	var mixed ownerGroup
	for _, group := range all.Owners {
		if group.Path == parent.Path {
			mixed = group
			break
		}
	}
	require.Equal(t, "包含受保护文件，请逐项查看分级。", mixed.Impact)
	require.Contains(t, mixed.Recovery, "按文件核对")
	protected := request("group=owner&grade=D&owner=" + url.QueryEscape(parent.Path))
	require.Len(t, protected.Files, 1)
	require.Equal(t, "这些文件受保护。", protected.Files[0].Owner.Impact)
	fallback := filepath.Join(root, "unknown")
	unassigned := request("group=owner&owner=" + url.QueryEscape(fallback))
	require.Len(t, unassigned.Files, 1)
	require.Equal(t, "loose.txt", unassigned.Files[0].Name)
	require.Equal(t, "folder", unassigned.Files[0].Owner.Kind)
	require.Equal(t, fallback, unassigned.Files[0].Owner.Path)
	require.Contains(t, unassigned.Files[0].Owner.Evidence, "直接上级目录")
	require.Greater(t, all.OwnerCount, result.OwnerCount)

	dependencyPath := filepath.Join(root, "project", "node_modules", "electron")
	narrowed := request("grade=AB&group=owner&path=" + url.QueryEscape(dependencyPath))
	require.Len(t, narrowed.Files, 2)
	require.Equal(t, parent.Path, narrowed.Files[0].Owner.Path)
	resolver := make(fileOwnerResolver)
	node, err := ui.findNode(narrowed.Files[0].Path)
	require.NoError(t, err)
	owner, err := json.Marshal(resolver.describe(node, narrowed.Files[0].fileSafety, ui.cleanupResult()))
	require.NoError(t, err)
	expected, err := json.Marshal(narrowed.Files[0].Owner)
	require.NoError(t, err)
	require.JSONEq(t, string(expected), string(owner))

	folder := request("grade=AB&group=owner&category=installer&folder=" + url.QueryEscape(parent.Path))
	require.Equal(t, 1, folder.OwnerCount)
	require.Equal(t, 1, folder.MatchedCount)
	require.Equal(t, "electron.exe", folder.Files[0].Name)
	require.Equal(t, []int{0, 1, 0, 0}, []int{
		folder.Grades[0].Count, folder.Grades[1].Count, folder.Grades[2].Count, folder.Grades[3].Count,
	})

	dir := request("grade=B&group=directory&folder=" + url.QueryEscape(parent.Path))
	require.Equal(t, 3, dir.MatchedCount)
	require.Equal(t, []int{1, 3, 2, 1}, []int{
		dir.Grades[0].Count, dir.Grades[1].Count, dir.Grades[2].Count, dir.Grades[3].Count,
	})
}
