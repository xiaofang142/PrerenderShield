package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIVersion API版本信息
type APIVersion struct {
	Version     string
	Deprecated  bool
	SunsetDate  string
	Description string
}

// 支持的API版本
var supportedVersions = map[string]*APIVersion{
	"v1": {
		Version:     "v1",
		Deprecated:  false,
		Description: "稳定版本",
	},
	"v2": {
		Version:     "v2",
		Deprecated:  false,
		Description: "最新版本",
	},
}

// VersionMiddleware API版本中间件
func VersionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从URL路径中提取版本号
		path := c.Request.URL.Path
		version := extractVersion(path)

		if version != "" {
			// 检查版本是否支持
			if apiVersion, exists := supportedVersions[version]; exists {
				// 设置版本信息到上下文
				c.Set("api_version", version)

				// 如果版本已弃用，添加警告头
				if apiVersion.Deprecated {
					c.Header("X-API-Deprecated", "true")
					c.Header("X-API-Sunset", apiVersion.SunsetDate)
					c.Header("X-API-Upgrade-To", "v2")
				}

				// 添加版本信息头
				c.Header("X-API-Version", version)
			} else {
				// 不支持的版本
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    400,
					"message": "不支持的API版本: " + version,
					"data": gin.H{
						"supported_versions": getSupportedVersionList(),
					},
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// extractVersion 从URL路径中提取版本号
func extractVersion(path string) string {
	// 匹配 /api/v1/... 或 /api/v2/... 模式
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "api" && i+1 < len(parts) {
			nextPart := parts[i+1]
			if strings.HasPrefix(nextPart, "v") {
				return nextPart
			}
		}
	}
	return ""
}

// getSupportedVersionList 获取支持的版本列表
func getSupportedVersionList() []string {
	versions := make([]string, 0, len(supportedVersions))
	for v := range supportedVersions {
		versions = append(versions, v)
	}
	return versions
}

// GetAPIVersion 获取API版本信息
func GetAPIVersion(version string) *APIVersion {
	return supportedVersions[version]
}

// IsVersionSupported 检查版本是否支持
func IsVersionSupported(version string) bool {
	_, exists := supportedVersions[version]
	return exists
}

// IsVersionDeprecated 检查版本是否已弃用
func IsVersionDeprecated(version string) bool {
	if apiVersion, exists := supportedVersions[version]; exists {
		return apiVersion.Deprecated
	}
	return false
}
