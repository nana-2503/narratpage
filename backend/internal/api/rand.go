package api

import "crypto/rand"

// randRead 填充随机字节。抽出来便于集中处理 panic 语义。
func randRead(b []byte) (int, error) { return rand.Read(b) }
