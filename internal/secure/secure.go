// Package secure 提供 JWT 令牌与敏感字段的可逆加密（与原 Python 版兼容）。
package secure

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/fernet/fernet-go"
	"github.com/golang-jwt/jwt/v5"

	"easysub/internal/config"
)

// ---------- JWT（HS256，payload: sub/type/iat/exp） ----------

func createToken(userID int, ttl time.Duration, typ string) string {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":  strconv.Itoa(userID),
		"type": typ,
		"iat":  now.Unix(),
		"exp":  now.Add(ttl).Unix(),
	}
	s, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.C.JWTSecret))
	return s
}

func AccessToken(userID int) string {
	return createToken(userID, config.C.AccessTokenTTL, "access")
}

func RefreshToken(userID int) string {
	return createToken(userID, config.C.RefreshTokenTTL, "refresh")
}

// DecodeToken 校验并返回用户 ID；类型不符或无效返回 0。
func DecodeToken(token, expected string) int {
	t, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(config.C.JWTSecret), nil
	})
	if err != nil || !t.Valid {
		return 0
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != expected {
		return 0
	}
	sub, _ := claims["sub"].(string)
	id, _ := strconv.Atoi(sub)
	return id
}

// RandomToken 生成 URL 安全的随机串（等价 secrets.token_urlsafe）。
func RandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ---------- 敏感字段加密（Fernet，密钥由 JWT_SECRET 派生；"enc:" 前缀） ----------

const prefix = "enc:"

func key() *fernet.Key {
	sum := sha256.Sum256([]byte(config.C.JWTSecret))
	var k fernet.Key
	copy(k[:], sum[:])
	return &k
}

// Encrypt 加密；空值或已加密值原样返回。
func Encrypt(v string) string {
	if v == "" || strings.HasPrefix(v, prefix) {
		return v
	}
	tok, err := fernet.EncryptAndSign([]byte(v), key())
	if err != nil {
		return v
	}
	return prefix + string(tok)
}

// Decrypt 解密；明文（无前缀）原样返回，解密失败返回空串。
func Decrypt(v string) string {
	if !strings.HasPrefix(v, prefix) {
		return v
	}
	msg := fernet.VerifyAndDecrypt([]byte(v[len(prefix):]), 0, []*fernet.Key{key()})
	if msg == nil {
		return ""
	}
	return string(msg)
}
