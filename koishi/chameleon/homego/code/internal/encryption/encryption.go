package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

const (
	envArticleIdKey = "ARTICLE_ID_KEY"
)

var (
	articleIdGCM cipher.AEAD
)

func init() {
	rawKey := os.Getenv(envArticleIdKey)
	if rawKey == "" {
		panic(fmt.Sprintf("%s environment variable is not set", envArticleIdKey))
	}

	keyHash := sha256.Sum256([]byte(rawKey))
	key := keyHash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(fmt.Sprintf("failed to create cipher: %v", err))
	}
	articleIdGCM, err = cipher.NewGCM(block)
	if err != nil {
		panic(fmt.Sprintf("failed to create GCM: %v", err))
	}
}

func CipherEncode(plain string) string {
	indexBytes := []byte(plain)

	nonce := make([]byte, articleIdGCM.NonceSize())
	if _, err := io.ReadFull(cryptorand.Reader, nonce); err != nil {
		panic(err)
	}

	ciphertext := articleIdGCM.Seal(nonce, nonce, indexBytes, nil)
	return base64.URLEncoding.EncodeToString(ciphertext)
}

func CipherDecode(encrypted string) (string, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("invalid string: %v", err)
	}

	if len(ciphertext) < articleIdGCM.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:articleIdGCM.NonceSize()]
	ciphertext = ciphertext[articleIdGCM.NonceSize():]

	plaintext, err := articleIdGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %v", err)
	}

	return string(plaintext), nil
}
