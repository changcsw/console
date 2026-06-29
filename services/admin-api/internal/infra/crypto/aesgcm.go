package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// AESGCM 对飞书第三方令牌等密文做 AES-GCM 加解密（00 §6.1）。
// 密钥来自配置（hex/base64），长度须为 16/24/32 字节。
type AESGCM struct {
	gcm cipher.AEAD
}

// NewAESGCM 从原始密钥字节构造；空密钥返回 nil（调用方据此跳过加密，仅 develop 容忍）。
func NewAESGCM(key []byte) (*AESGCM, error) {
	if len(key) == 0 {
		return nil, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes key: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESGCM{gcm: gcm}, nil
}

// DecodeKey 解析配置中的密钥：优先 base64，失败再按原始字节。
func DecodeKey(s string) []byte {
	if s == "" {
		return nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && (len(b) == 16 || len(b) == 24 || len(b) == 32) {
		return b
	}
	return []byte(s)
}

// Encrypt 加密并返回 base64(nonce||ciphertext)。
func (a *AESGCM) Encrypt(plain string) (string, error) {
	if a == nil {
		return "", errors.New("aes-gcm not configured")
	}
	nonce := make([]byte, a.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := a.gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密 base64(nonce||ciphertext)。
func (a *AESGCM) Decrypt(encoded string) (string, error) {
	if a == nil {
		return "", errors.New("aes-gcm not configured")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	ns := a.gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plain, err := a.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
