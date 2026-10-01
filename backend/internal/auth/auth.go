// Package auth 提供 JWT 签发校验、密码哈希与 RBAC 权限中间件。
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Claims JWT 载荷。
type Claims struct {
	Sub      int64  `json:"sub"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// ErrInvalidToken 表示 token 缺失、过期或签名不匹配。
var ErrInvalidToken = errors.New("invalid token")

// Sign 签发 token。jti 用于关联 sessions 表，实现「登出所有设备」。
func Sign(secret string, expiry time.Duration, userID int64, username, role string) (string, string, error) {
	jti := randomID()
	claims := Claims{
		Sub:      userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	return s, jti, err
}

// Parse 校验并解析 token。
func Parse(secret, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		// 固定算法校验：防止 alg=none 或 RS256 降级攻击
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// HashPassword bcrypt 哈希。
func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), 10)
	return string(h), err
}

// CheckPassword 校验密码。
func CheckPassword(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
