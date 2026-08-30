package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
)

const apiTokenPrefix = "pst_"

// GenerateToken 生成管理 API Token。
// raw 形如 pst_<64位hex>（仅生成时展示一次，不落原文）；
// 返回值之二为 sha256(raw) hex，作为配置中唯一的存储形态。
func GenerateToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate api token: %w", err)
	}
	raw := apiTokenPrefix + hex.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// HashToken 计算 Token 的 sha256 hex（配置存储形态）
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// VerifyToken 常数时间比较 raw Token 与存储的 sha256 hex。
// 任一哈希命中即返回 true；空输入恒 false。
func VerifyToken(raw string, sha256HexHashes []string) bool {
	if raw == "" || len(sha256HexHashes) == 0 {
		return false
	}
	given := HashToken(raw)
	givenBytes := []byte(given)
	for _, h := range sha256HexHashes {
		if h == "" {
			continue
		}
		if subtle.ConstantTimeCompare(givenBytes, []byte(h)) == 1 {
			return true
		}
	}
	return false
}
