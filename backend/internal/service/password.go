package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func GeneratePasswordHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(bcryptPrehash(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(password, stored string) bool {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return false
	}

	if strings.HasPrefix(stored, "bcrypt$") {
		hash := stored[len("bcrypt$"):]
		return bcrypt.CompareHashAndPassword([]byte(hash), bcryptPrehash(password)) == nil
	}

	parts := strings.SplitN(stored, "$", 3)
	if len(parts) != 3 || parts[0] != "sha256" {
		return false
	}
	expected := sha256.Sum256([]byte(parts[1] + password))
	expectedHex := hex.EncodeToString(expected[:])
	return subtle.ConstantTimeCompare([]byte(expectedHex), []byte(parts[2])) == 1
}

func bcryptPrehash(password string) []byte {
	sum := sha256.Sum256([]byte(password))
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(sum)))
	base64.StdEncoding.Encode(dst, sum[:])
	return dst
}
