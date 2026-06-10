package controllers

import (
	"archive/zip"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"prerender-shield/internal/config"
	"prerender-shield/internal/logging"
)

// GetStaticFiles 获取静态资源文件列表
func (c *SitesController) GetStaticFiles(ctx *gin.Context) {
	id := ctx.Param("id")
	path := ctx.Query("path")

	if c.configManager == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Configuration manager not initialized"})
		return
	}
	currentConfig := c.configManager.GetConfig()

	site := findSiteByID(currentConfig.Sites, id)
	if site == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Site not found"})
		return
	}

	siteStaticDir := filepath.Join(c.cfg.Dirs.StaticDir, site.ID)
	filePath := filepath.Join(siteStaticDir, path)

	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		if strings.HasSuffix(path, "/") {
			ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": []gin.H{}})
			return
		}
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "File not found"})
		return
	}

	if !fileInfo.IsDir() {
		ctx.File(filePath)
		return
	}

	files, err := os.ReadDir(filePath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to read directory"})
		return
	}

	var fileList []gin.H
	for _, file := range files {
		fInfo, _ := file.Info()
		fp := "/" + file.Name()
		if path != "" && path != "/" {
			fp = filepath.Join(path, file.Name())
		}
		fileList = append(fileList, gin.H{
			"key":   file.Name(),
			"name":  file.Name(),
			"type":  map[bool]string{true: "dir"}[file.IsDir()],
			"size":  fInfo.Size(),
			"isDir": file.IsDir(),
			"path":  fp,
		})
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": fileList})
}

// UploadStaticFile 上传静态资源文件
func (c *SitesController) UploadStaticFile(ctx *gin.Context) {
	id := ctx.Param("id")
	path := ctx.PostForm("path")

	site, errMsg, code := c.resolveSite(id)
	if errMsg != "" {
		ctx.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	siteStaticDir := filepath.Join(c.cfg.Dirs.StaticDir, site.ID)

	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Failed to get file"})
		return
	}

	filePath := filepath.Join(siteStaticDir, path, file.Filename)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create directory"})
		return
	}
	if err := ctx.SaveUploadedFile(file, filePath); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to save file"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "File uploaded successfully"})
}

// ExtractFile 解压ZIP文件
func (c *SitesController) ExtractFile(ctx *gin.Context) {
	id := ctx.Param("id")
	fileName := ctx.PostForm("filename")
	path := ctx.PostForm("path")

	site, errMsg, code := c.resolveSite(id)
	if errMsg != "" {
		ctx.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	siteStaticDir := filepath.Join(c.cfg.Dirs.StaticDir, site.ID)
	cleanPath := strings.TrimPrefix(path, "/")
	if cleanPath == "" {
		cleanPath = "."
	}
	filePath := filepath.Join(siteStaticDir, cleanPath, fileName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		ctx.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "File not found"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(fileName), ".zip") {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Only ZIP files are supported"})
		return
	}

	destDir := filepath.Join(siteStaticDir, cleanPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to create directory"})
		return
	}
	if err := ExtractZIP(filePath, destDir); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to extract ZIP"})
		return
	}

	c.indexExtractedFiles(site, destDir)
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "File extracted successfully"})
}

// DeleteStaticFile 删除静态资源文件
func (c *SitesController) DeleteStaticFile(ctx *gin.Context) {
	id := ctx.Param("id")
	path := ctx.Query("path")

	site, errMsg, code := c.resolveSite(id)
	if errMsg != "" {
		ctx.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	siteStaticDir := filepath.Join(c.cfg.Dirs.StaticDir, site.ID)

	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid path"})
		return
	}

	filePath := filepath.Join(siteStaticDir, cleanPath)
	absFP, _ := filepath.Abs(filePath)
	absSD, _ := filepath.Abs(siteStaticDir)
	if !strings.HasPrefix(absFP, absSD) {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Forbidden path"})
		return
	}

	if err := os.RemoveAll(filePath); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "Failed to delete file"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "File deleted successfully"})
}

// BatchDeleteStaticFiles 批量删除静态资源文件
func (c *SitesController) BatchDeleteStaticFiles(ctx *gin.Context) {
	id := ctx.Param("id")
	var req struct {
		Paths []string `json:"paths" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "Invalid request"})
		return
	}

	site, errMsg, code := c.resolveSite(id)
	if errMsg != "" {
		ctx.JSON(code, gin.H{"code": code, "message": errMsg})
		return
	}

	siteStaticDir := filepath.Join(c.cfg.Dirs.StaticDir, site.ID)
	var failedPaths []string
	deletedCount := 0

	for _, p := range req.Paths {
		relPath := strings.TrimPrefix(filepath.Clean(p), "/")
		if strings.Contains(relPath, "..") {
			failedPaths = append(failedPaths, p)
			continue
		}
		fp := filepath.Join(siteStaticDir, relPath)
		absFP, _ := filepath.Abs(fp)
		absSD, _ := filepath.Abs(siteStaticDir)
		if !strings.HasPrefix(absFP, absSD) {
			failedPaths = append(failedPaths, p)
			continue
		}
		if err := os.RemoveAll(fp); err != nil && !os.IsNotExist(err) {
			failedPaths = append(failedPaths, p)
		} else {
			deletedCount++
		}
	}

	if len(failedPaths) > 0 {
		ctx.JSON(http.StatusPartialContent, gin.H{"code": 206, "message": "Some files failed", "data": gin.H{"deleted": deletedCount, "failed": failedPaths}})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "Files deleted successfully", "data": gin.H{"deleted": deletedCount}})
}

// resolveSite 查找站点，复用代码
func (c *SitesController) resolveSite(id string) (*config.SiteConfig, string, int) {
	if c.configManager == nil {
		return nil, "Configuration manager not initialized", http.StatusInternalServerError
	}
	currentConfig := c.configManager.GetConfig()
	site := findSiteByID(currentConfig.Sites, id)
	if site == nil {
		return nil, "Site not found", http.StatusNotFound
	}
	return site, "", 0
}

func findSiteByID(sites []config.SiteConfig, id string) *config.SiteConfig {
	for i, s := range sites {
		if s.ID == id {
			return &sites[i]
		}
	}
	return nil
}

// indexExtractedFiles 将解压后的HTML文件URL存入Redis
func (c *SitesController) indexExtractedFiles(site *config.SiteConfig, destDir string) {
	if c.redisClient == nil || len(site.Domains) == 0 {
		return
	}

	var htmlFiles []string
	filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".html") {
			if relPath, err := filepath.Rel(destDir, path); err == nil {
				url := fmt.Sprintf("http://%s:%d/%s", site.Domains[0], site.Port, strings.ReplaceAll(relPath, "\\", "/"))
				htmlFiles = append(htmlFiles, url)
			}
		}
		return nil
	})

	for _, url := range htmlFiles {
		if err := c.redisClient.AddURL(site.ID, url); err != nil {
			logging.DefaultLogger.Info("Failed to add URL to Redis: %v", err)
		}
	}
	if len(htmlFiles) > 0 {
		c.redisClient.SetSiteStats(site.ID, map[string]interface{}{"url_count": len(htmlFiles)})
	}
}

// isPortAvailable 检查端口是否可用
func isPortAvailable(port int) bool {
	reservedPorts := map[int]bool{
		21: true, 22: true, 23: true, 25: true, 53: true,
		80: true, 110: true, 143: true, 443: true, 465: true,
		587: true, 993: true, 995: true, 3306: true, 5432: true,
		6379: true, 8080: true, 9000: true, 9090: true, 15672: true, 27017: true,
	}
	if reservedPorts[port] {
		return false
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// ExtractZIP 解压ZIP文件
func ExtractZIP(filePath, destDir string) error {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		destFilePath := filepath.Join(destDir, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(destFilePath, file.Mode())
			continue
		}
		os.MkdirAll(filepath.Dir(destFilePath), 0755)
		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		zipFile, err := file.Open()
		if err != nil {
			destFile.Close()
			return err
		}
		io.Copy(destFile, zipFile)
		zipFile.Close()
		destFile.Close()
	}
	return nil
}
