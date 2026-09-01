package securetoken

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
)

func RandomHex(bytes int) (string, error) {
	value, err := random(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func RandomURL(bytes int) (string, error) {
	value, err := random(bytes)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func Hash(value string) []byte {
	result := sha256.Sum256([]byte(value))
	return result[:]
}

func EqualHash(value string, expected []byte) bool {
	return subtle.ConstantTimeCompare(Hash(value), expected) == 1
}

func random(bytes int) ([]byte, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return nil, err
	}
	return value, nil
}
