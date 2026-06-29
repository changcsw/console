package crypto

import (
	"encoding/base64"
	"testing"
)

func TestPasswordHashAndCompare(t *testing.T) {
	h := NewPasswordHasher(10)
	hash, err := h.Hash("S3cret_pass")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "S3cret_pass" {
		t.Fatal("hash must not equal plaintext")
	}
	if err := h.Compare(hash, "S3cret_pass"); err != nil {
		t.Fatalf("compare should match: %v", err)
	}
	if err := h.Compare(hash, "wrong"); err == nil {
		t.Fatal("compare should fail for wrong password")
	}
}

func TestPasswordHasherClampsCost(t *testing.T) {
	// 越界 cost 回落到默认，仍可用
	h := NewPasswordHasher(999)
	hash, err := h.Hash("abc12345")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := h.Compare(hash, "abc12345"); err != nil {
		t.Fatalf("compare: %v", err)
	}
}

func TestAESGCMRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	a, err := NewAESGCM(key)
	if err != nil || a == nil {
		t.Fatalf("new aesgcm: %v", err)
	}
	ct, err := a.Encrypt("feishu-token")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ct == "feishu-token" {
		t.Fatal("ciphertext must differ from plaintext")
	}
	pt, err := a.Decrypt(ct)
	if err != nil || pt != "feishu-token" {
		t.Fatalf("decrypt: %q %v", pt, err)
	}
}

func TestAESGCMNilWhenNoKey(t *testing.T) {
	a, err := NewAESGCM(nil)
	if err != nil {
		t.Fatalf("nil key should not error: %v", err)
	}
	if a != nil {
		t.Fatal("expected nil cipher for empty key")
	}
}

func TestAESGCMTamperDetected(t *testing.T) {
	a, _ := NewAESGCM([]byte("0123456789abcdef0123456789abcdef"))
	ct, err := a.Encrypt("feishu-token")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(ct)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// 翻转最后一个字节（密文区）→ GCM 认证标签校验失败。
	raw[len(raw)-1] ^= 0xFF
	tampered := base64.StdEncoding.EncodeToString(raw)
	if _, err := a.Decrypt(tampered); err == nil {
		t.Fatal("tampered ciphertext must fail authentication")
	}
}

func TestAESGCMDecryptRejectsGarbage(t *testing.T) {
	a, _ := NewAESGCM([]byte("0123456789abcdef"))
	if _, err := a.Decrypt("!!!not-base64!!!"); err == nil {
		t.Fatal("non-base64 must error")
	}
	// 比 nonce 还短的密文
	short := base64.StdEncoding.EncodeToString([]byte("x"))
	if _, err := a.Decrypt(short); err == nil {
		t.Fatal("too-short ciphertext must error")
	}
}

func TestAESGCMNilCipherErrors(t *testing.T) {
	var a *AESGCM // 未配置密钥的占位
	if _, err := a.Encrypt("x"); err == nil {
		t.Fatal("nil cipher Encrypt must error")
	}
	if _, err := a.Decrypt("x"); err == nil {
		t.Fatal("nil cipher Decrypt must error")
	}
}

func TestAESGCMUniqueNoncePerEncrypt(t *testing.T) {
	a, _ := NewAESGCM([]byte("0123456789abcdef0123456789abcdef"))
	c1, _ := a.Encrypt("same")
	c2, _ := a.Encrypt("same")
	if c1 == c2 {
		t.Fatal("identical plaintext must yield different ciphertext (random nonce)")
	}
}

func TestDecodeKey(t *testing.T) {
	if DecodeKey("") != nil {
		t.Fatal("empty string must decode to nil")
	}
	// 含非 base64 字符（'-'）的 16 字节字符串 → 回退原始字节。
	rawKey := "secret-key-16byt" // 16 chars, '-' 非标准 base64 字母
	raw := DecodeKey(rawKey)
	if len(raw) != 16 || string(raw) != rawKey {
		t.Fatalf("expected 16-byte raw key fallback, got %d %q", len(raw), string(raw))
	}
	// 合法 base64 且解码为 32 字节 → 用解码结果。
	b64 := base64.StdEncoding.EncodeToString(make([]byte, 32))
	if got := DecodeKey(b64); len(got) != 32 {
		t.Fatalf("expected 32-byte decoded key, got %d", len(got))
	}
}

func TestPasswordEmptyAndMismatch(t *testing.T) {
	h := NewPasswordHasher(10)
	// 空密码可哈希，但与非空不匹配（防空口令绕过）。
	hash, err := h.Hash("")
	if err != nil {
		t.Fatalf("hash empty: %v", err)
	}
	if err := h.Compare(hash, "anything"); err == nil {
		t.Fatal("empty-password hash must not match non-empty")
	}
	if err := h.Compare(hash, ""); err != nil {
		t.Fatalf("empty must match its own hash: %v", err)
	}
}

func TestPasswordHashNonDeterministic(t *testing.T) {
	h := NewPasswordHasher(10)
	h1, _ := h.Hash("S3cret_pass")
	h2, _ := h.Hash("S3cret_pass")
	if h1 == h2 {
		t.Fatal("bcrypt must use random salt → different hashes for same input")
	}
	// 两个不同哈希都能校验通过同一明文
	if err := h.Compare(h1, "S3cret_pass"); err != nil {
		t.Fatalf("h1 compare: %v", err)
	}
	if err := h.Compare(h2, "S3cret_pass"); err != nil {
		t.Fatalf("h2 compare: %v", err)
	}
}

func TestPasswordCompareRejectsGarbageHash(t *testing.T) {
	h := NewPasswordHasher(10)
	if err := h.Compare("not-a-bcrypt-hash", "x"); err == nil {
		t.Fatal("invalid hash format must error")
	}
}
