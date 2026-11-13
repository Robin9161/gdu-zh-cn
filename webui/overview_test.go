package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOverviewDirectoryGroupsUseFilteredDescendantUsage(t *testing.T) {
	root := t.TempDir()
	deps := filepath.Join(root, "dependencies")
	fixtures := []struct {
		path  string
		grade cleanupGrade
		usage int64
	}{
		{filepath.Join(deps, "first", "index.js"), gradeB, 70},
		{filepath.Join(deps, "second", "index.js"), gradeB, 40},
		{filepath.Join(root, "cache", "example.pyc"), gradeA, 25},
		{filepath.Join(root, "dependencies-other", "example.pyc"), gradeA, 15},
		{filepath.Join(root, "backup", "state.db"), gradeD, 1000},
		{filepath.Join(root, "loose.pyc"), gradeA, 5},
	}
	data := fileInventory{}
	for i, fixture := range fixtures {
		data.files = append(data.files, overviewFile{
			Path: fixture.path, Usage: fixture.usage, Size: 1, fileSafety: fileSafety{Grade: fixture.grade},
		})
		data.order[1] = append(data.order[1], i)
	}
	filter := overviewFilter{folder: root, groupByDirectory: true, grade: gradeAB, metric: 1}
	grouped := data.filter(context.Background(), filter)
	require.Equal(t, []overviewCategory{
		{ID: "dependencies", Bytes: 110, Count: 2},
		{ID: "cache", Bytes: 25, Count: 1},
		{ID: "dependencies-other", Bytes: 15, Count: 1},
	}, grouped.Directories)
	require.Equal(t, 5, grouped.MatchedCount)

	filter.folder = deps
	detail := data.filter(context.Background(), filter)
	require.Equal(t, 2, detail.MatchedCount)
	require.Len(t, detail.Files, 2)
	require.Equal(t, fixtures[0].path, detail.Files[0].Path)
	require.Equal(t, fixtures[1].path, detail.Files[1].Path)
}

func TestOverviewFiltersAllFilesAndPagesWithoutLosingTotals(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	require.NoError(t, os.Mkdir(nested, 0o700))
	modified := time.Now().Add(-180 * 24 * time.Hour)
	for i := 0; i < overviewPage+2; i++ {
		path := filepath.Join(nested, fmt.Sprintf("clip-%02d.MOV", i))
		require.NoError(t, os.WriteFile(path, make([]byte, 1024), 0o600))
		require.NoError(t, os.Chtimes(path, modified, modified))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "readme.txt"), []byte("abc"), 0o600))
	ui := newTestUI()
	scan(t, ui, root)
	routes := ui.routes()
	get := func(query string) overviewResult {
		t.Helper()
		recorder := httptest.NewRecorder()
		routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/overview?"+query, nil))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var result overviewResult
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
		return result
	}

	all := get("metric=size")
	require.Equal(t, overviewPage+3, all.FileCount)
	require.EqualValues(t, (overviewPage+2)*1024+3, all.FileBytes)
	require.Equal(t, overviewCategory{ID: "video", Bytes: (overviewPage + 2) * 1024, Count: overviewPage + 2}, all.Categories[0])

	videos := get("metric=size&category=video")
	require.Len(t, videos.Points, 1)
	require.Equal(t, overviewPage+2, videos.Points[0].Count)
	require.Len(t, videos.Files, overviewPage)
	require.Equal(t, overviewPage+2, videos.MatchedCount)
	require.Equal(t, "clip-00.MOV", videos.Files[0].Name)

	point := videos.Points[0]
	filtered := get(fmt.Sprintf("metric=size&box=%d,%d,%d,%d&offset=%d", point.X, point.X, point.Y, point.Y, overviewPage))
	require.Equal(t, overviewPage+2, filtered.MatchedCount)
	require.EqualValues(t, (overviewPage+2)*1024, filtered.MatchedBytes)
	require.Len(t, filtered.Files, 2)
	require.Equal(t, "video", filtered.Files[0].Category)

	empty := get("metric=size&category=document&box=95,95,47,47")
	require.Empty(t, empty.Files)
	require.Zero(t, empty.MatchedCount)
}

func TestOverviewSafetyGradesCombineWithCategory(t *testing.T) {
	root := cleanupFixtureDir(t)
	files := map[string]cleanupGrade{
		"__pycache__/example.cpython-312.pyc": gradeA,
		"node_modules/example/index.js":       gradeB,
		"unverified/node_modules/original.js": gradeC,
		"project.json":                        gradeC,
		"package.json":                        gradeC,
		".ssh/settings.json":                  gradeD,
		"state.sqlite-wal":                    gradeD,
		".env.local":                          gradeD,
	}
	for path := range files {
		fullPath := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o700))
		require.NoError(t, os.WriteFile(fullPath, []byte("fixture"), 0o600))
	}

	ui := newTestUI()
	scan(t, ui, root)
	routes := ui.routes()
	get := func(query string) overviewResult {
		t.Helper()
		recorder := httptest.NewRecorder()
		routes.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/overview?"+query, nil))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		var result overviewResult
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
		return result
	}
	all := get("metric=size")
	require.Len(t, all.Files, len(files))
	for _, file := range all.Files {
		rel, err := filepath.Rel(root, file.Path)
		require.NoError(t, err)
		require.Equal(t, files[filepath.ToSlash(rel)], file.Grade, file.Path)
		require.NotEmpty(t, file.Reason, file.Path)
	}

	protectedCode := get("metric=size&category=code&grade=D")
	require.Equal(t, 1, protectedCode.MatchedCount)
	require.EqualValues(t, len("fixture"), protectedCode.MatchedBytes)
	require.Equal(t, "settings.json", protectedCode.Files[0].Name)
	require.Equal(t, []int{0, 1, 3, 1}, []int{
		protectedCode.Grades[0].Count, protectedCode.Grades[1].Count,
		protectedCode.Grades[2].Count, protectedCode.Grades[3].Count,
	})
	require.Empty(t, get("category=code&grade=A").Files)
	both := get("metric=size&grade=AB")
	require.Equal(t, 2, both.MatchedCount)
	require.EqualValues(t, 2*len("fixture"), both.MatchedBytes)
	require.ElementsMatch(t, []cleanupGrade{gradeA, gradeB}, []cleanupGrade{both.Files[0].Grade, both.Files[1].Grade})
}

// Use the checkout rather than the system temp directory, which Windows
// normally places under the protected AppData directory.
func cleanupFixtureDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "gdu-cleanup-test-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(dir)) })
	root, err := filepath.Abs(dir)
	require.NoError(t, err)
	if protectedReason(pathParts(filepath.Join(root, "fixture")), "other") != "" {
		t.Skip("the test checkout is inside a protected directory")
	}
	return root
}
