package store

import "golang.org/x/crypto/bcrypt"

// HashPassword 使用 bcrypt（与原版 passlib bcrypt 哈希互通）。
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func VerifyPassword(pw, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
