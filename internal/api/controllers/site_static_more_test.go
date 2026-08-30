package controllers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"prerender-shield/internal/config"
)

// setupStaticController 构建带 configManager 与可选 redis 的站点控制器（静态资源端点）
func setupStaticController(t *testing.T, redisOpts ...func(*MockRedisClient)) (*SitesController, *gin.Engine, string) {
	t.Helper()

	staticRoot := t.TempDir()
	siteDir := filepath.Join(staticRoot, "ctl-static")
	require.NoError(t, os.MkdirAll(siteDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(siteDir, "index.html"), []byte("<html>home</html>"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(siteDir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(siteDir, "sub", "page.html"), []byte("<html>page</html>"), 0o644))

	cfg := &config.Config{
		Sites: []config.SiteConfig{{ID: "ctl-static", Name: "Static", Domains: []string{"static.example"}, Port: 8080}},
		Dirs:  config.DirsConfig{StaticDir: staticRoot},
	}
	mockCM := &MockConfigManager{config: cfg}

	mockRedis := &MockRedisClient{storedStats: map[string]map[string]interface{}{}}
	for _, opt := range redisOpts {
		opt(mockRedis)
	}

	controller := NewSitesController(mockCM, nil, &MockSiteHandler{}, mockRedis, nil, nil, nil, cfg)

	router := ginNewRouter()
	router.GET("/sites/:id/static", controller.GetStaticFiles)
	router.POST("/sites/:id/static/upload", controller.UploadStaticFile)
	router.POST("/sites/:id/static/extract", controller.ExtractFile)
	router.DELETE("/sites/:id/static", controller.DeleteStaticFile)
	router.POST("/sites/:id/static/batch-delete", controller.BatchDeleteStaticFiles)
	return controller, router, siteDir
}

// TestSitesController_Static_FullFlow 列表/子目录/单文件/不存在路径分支
func TestSitesController_Static_FullFlow(t *testing.T) {
	_, router, siteDir := setupStaticController(t)

	// 站点不存在 → 404
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ghost/static", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 根目录列表
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-static/static", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "index.html")

	// 子目录列表（path 参数）
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-static/static?path=sub", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "page.html")

	// 单文件 → ctx.File 内容直出
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-static/static?path=index.html", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "home")

	// 不存在目录（带尾斜杠）→ 200 空列表
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-static/static?path=missing/", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"data":[]`)

	// 不存在文件 → 404
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-static/static?path=missing.txt", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// ReadDir 失败：目录无读权限
	noRead := filepath.Join(siteDir, "locked")
	require.NoError(t, os.MkdirAll(noRead, 0o755))
	require.NoError(t, os.Chmod(noRead, 0o000))
	t.Cleanup(func() { _ = os.Chmod(noRead, 0o755) })
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/sites/ctl-static/static?path=locked", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestSitesController_UploadStaticFile_Branches 上传：无文件 / 目录创建失败 / 保存失败 / 成功
func TestSitesController_UploadStaticFile_Branches(t *testing.T) {
	_, router, siteDir := setupStaticController(t)

	// 无文件 → 400
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/sites/ctl-static/static/upload", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// path 的父级是已存在文件 → MkdirAll 失败 → 500
	require.NoError(t, os.WriteFile(filepath.Join(siteDir, "blocker"), []byte("f"), 0o644))
	body2 := &bytes.Buffer{}
	writer := multipart.NewWriter(body2)
	writer.WriteField("path", "/blocker/sub")
	part2, _ := writer.CreateFormFile("file", "a.txt")
	part2.Write([]byte("content"))
	writer.Close()
	w = httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/sites/ctl-static/static/upload", body2)
	req2.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req2)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// filename 与现有目录冲突 → SaveUploadedFile 失败 → 500
	body3 := &bytes.Buffer{}
	writer3 := multipart.NewWriter(body3)
	writer3.WriteField("path", "")
	part3, _ := writer3.CreateFormFile("file", "sub")
	part3.Write([]byte("content"))
	writer3.Close()
	w = httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/sites/ctl-static/static/upload", body3)
	req3.Header.Set("Content-Type", writer3.FormDataContentType())
	router.ServeHTTP(w, req3)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// 成功上传
	body4 := &bytes.Buffer{}
	writer4 := multipart.NewWriter(body4)
	writer4.WriteField("path", "")
	part4, _ := writer4.CreateFormFile("file", "uploaded.txt")
	part4.Write([]byte("uploaded"))
	writer4.Close()
	w = httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodPost, "/sites/ctl-static/static/upload", body4)
	req4.Header.Set("Content-Type", writer4.FormDataContentType())
	router.ServeHTTP(w, req4)
	require.Equal(t, http.StatusOK, w.Code)
	_, err := os.Stat(filepath.Join(siteDir, "uploaded.txt"))
	assert.NoError(t, err)
}

// TestSitesController_ExtractFile_Branches 解压：不存在/非 zip/损坏 zip/成功 + Redis 索引
func TestSitesController_ExtractFile_Branches(t *testing.T) {
	addURLErr := false
	_, router, siteDir := setupStaticController(t, func(m *MockRedisClient) {
		m.addURLFunc = func(siteID, url string) error {
			if addURLErr {
				return fmt.Errorf("redis down")
			}
			return nil
		}
	})

	// 文件不存在 → 404
	w := httptest.NewRecorder()
	req := extractForm(t, "ghost.zip", "")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 非 zip → 400
	require.NoError(t, os.WriteFile(filepath.Join(siteDir, "notzip.txt"), []byte("x"), 0o644))
	w = httptest.NewRecorder()
	req = extractForm(t, "notzip.txt", "")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 损坏的 zip → 500
	require.NoError(t, os.WriteFile(filepath.Join(siteDir, "broken.zip"), []byte("not a zip"), 0o644))
	w = httptest.NewRecorder()
	req = extractForm(t, "broken.zip", "")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// 成功解压（含 html → Redis 索引 + 统计）
	zipPath := filepath.Join(siteDir, "good.zip")
	createTestZip(t, zipPath, map[string]string{"inner/index.html": "<html>i</html>", "assets/app.js": "console.log(1)"})
	w = httptest.NewRecorder()
	req = extractForm(t, "good.zip", "")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	_, err := os.Stat(filepath.Join(siteDir, "inner", "index.html"))
	assert.NoError(t, err)
}

// TestSitesController_ExtractFile_IndexErrors AddURL 失败仅记日志
func TestSitesController_ExtractFile_IndexErrors(t *testing.T) {
	_, router, siteDir := setupStaticController(t, func(m *MockRedisClient) {
		m.addURLFunc = func(siteID, url string) error { return fmt.Errorf("redis down") }
	})

	zipPath := filepath.Join(siteDir, "idx.zip")
	createTestZip(t, zipPath, map[string]string{"page.html": "<html>p</html>"})

	w := httptest.NewRecorder()
	req := extractForm(t, "idx.zip", "")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

// TestSitesController_DeleteStaticFile_Branches 删除：..路径/成功/删除失败
func TestSitesController_DeleteStaticFile_Branches(t *testing.T) {
	_, router, siteDir := setupStaticController(t)

	// ".." 穿越 → 400
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/sites/ctl-static/static?path=../x", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 成功删除
	require.NoError(t, os.WriteFile(filepath.Join(siteDir, "del.txt"), []byte("x"), 0o644))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/sites/ctl-static/static?path=del.txt", nil))
	require.Equal(t, http.StatusOK, w.Code)

	// 删除失败：目标目录内文件无权限
	locked := filepath.Join(siteDir, "lockeddir")
	inner := filepath.Join(locked, "f.txt")
	require.NoError(t, os.MkdirAll(locked, 0o755))
	require.NoError(t, os.WriteFile(inner, []byte("x"), 0o644))
	require.NoError(t, os.Chmod(locked, 0o555))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/sites/ctl-static/static?path=lockeddir", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestSitesController_BatchDeleteStaticFiles_Branches 批量删除：..路径/部分失败/成功
func TestSitesController_BatchDeleteStaticFiles_Branches(t *testing.T) {
	_, router, siteDir := setupStaticController(t)

	// 站点不存在 → 404
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sites/ghost/static/batch-delete",
		jsonBody(t, map[string]interface{}{"paths": []string{"/a"}}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// ".." 路径 → 206 部分失败
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/sites/ctl-static/static/batch-delete",
		jsonBody(t, map[string]interface{}{"paths": []string{"a/../../evil"}}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusPartialContent, w.Code)
	assert.Contains(t, w.Body.String(), "Some files failed")

	// 删除失败（父目录无写权限）→ 206
	locked := filepath.Join(siteDir, "lockdir")
	require.NoError(t, os.MkdirAll(filepath.Join(locked, "c"), 0o755))
	require.NoError(t, os.Chmod(locked, 0o555))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/sites/ctl-static/static/batch-delete",
		jsonBody(t, map[string]interface{}{"paths": []string{"/lockdir"}}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusPartialContent, w.Code)

	// 全部成功
	require.NoError(t, os.WriteFile(filepath.Join(siteDir, "b1.txt"), []byte("1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(siteDir, "b2.txt"), []byte("2"), 0o644))
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/sites/ctl-static/static/batch-delete",
		jsonBody(t, map[string]interface{}{"paths": []string{"/b1.txt", "/b2.txt"}}))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"deleted":2`)
}

// extractForm 构造 ExtractFile 的 multipart 请求
func extractForm(t *testing.T, filename, path string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("filename", filename)
	writer.WriteField("path", path)
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/sites/ctl-static/static/extract", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// createTestZip 创建包含给定文件的 zip
func createTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		require.NoError(t, err)
		_, err = fw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
}
