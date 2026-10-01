package api

import (
	"crypto/sha256"
	"encoding/hex"
)

// shortHash 生成 16 位十六进制短哈希，用于 IP 去标识化存储。
func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:16]
}
