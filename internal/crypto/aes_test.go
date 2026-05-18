package crypto

import (
	"encoding/hex"
	"strings"
	"testing"
)

// testKeyHex 返回一个 64 字符（32 字节）的十六进制密钥用于测试
func testKeyHex() string {
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}

// TestEncryptDecrypt_RoundTrip 验证加密后解密能还原原始明文
func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := testKeyHex()
	plaintext := "hello world, this is a test message"

	cipherHex, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if cipherHex == "" {
		t.Fatal("expected non-empty ciphertext")
	}

	decrypted, err := Decrypt(cipherHex, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("Decrypt(Encrypt(%q)) = %q, want %q", plaintext, decrypted, plaintext)
	}
}

// TestEncryptDecrypt_EmptyString 验证空字符串的加解密一致性
func TestEncryptDecrypt_EmptyString(t *testing.T) {
	key := testKeyHex()

	cipherHex, err := Encrypt("", key)
	if err != nil {
		t.Fatalf("Encrypt empty string failed: %v", err)
	}

	decrypted, err := Decrypt(cipherHex, key)
	if err != nil {
		t.Fatalf("Decrypt empty string failed: %v", err)
	}
	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

// TestEncryptDecrypt_Unicode 验证 Unicode 文本的加解密一致性
func TestEncryptDecrypt_Unicode(t *testing.T) {
	key := testKeyHex()
	plaintext := "你好，世界！🎉"

	cipherHex, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt unicode failed: %v", err)
	}

	decrypted, err := Decrypt(cipherHex, key)
	if err != nil {
		t.Fatalf("Decrypt unicode failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("Decrypt(Encrypt(%q)) = %q, want %q", plaintext, decrypted, plaintext)
	}
}

// TestEncryptDecrypt_LongText 验证长文本的加解密一致性
func TestEncryptDecrypt_LongText(t *testing.T) {
	key := testKeyHex()
	plaintext := strings.Repeat("A", 10000)

	cipherHex, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt long text failed: %v", err)
	}

	decrypted, err := Decrypt(cipherHex, key)
	if err != nil {
		t.Fatalf("Decrypt long text failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("Decrypt(Encrypt(long text)) mismatch, len(decrypted)=%d, len(plaintext)=%d", len(decrypted), len(plaintext))
	}
}

// TestKeyLength_TooShort 验证过短的密钥返回错误
func TestKeyLength_TooShort(t *testing.T) {
	shortKey := hex.EncodeToString([]byte("short"))
	_, err := Encrypt("test", shortKey)
	if err == nil {
		t.Fatal("expected error for short key, got nil")
	}
	if !strings.Contains(err.Error(), "key must be 32 bytes") {
		t.Errorf("expected error about key length, got: %v", err)
	}
}

// TestKeyLength_TooLong 验证过长的密钥返回错误
func TestKeyLength_TooLong(t *testing.T) {
	longKey := strings.Repeat("ab", 33) // 66 hex chars = 33 bytes
	_, err := Encrypt("test", longKey)
	if err == nil {
		t.Fatal("expected error for long key, got nil")
	}
	if !strings.Contains(err.Error(), "key must be 32 bytes") {
		t.Errorf("expected error about key length, got: %v", err)
	}
}

// TestKeyLength_Exactly32 验证正好 32 字节密钥可以正常工作
func TestKeyLength_Exactly32(t *testing.T) {
	key := testKeyHex()
	cipherHex, err := Encrypt("test", key)
	if err != nil {
		t.Fatalf("Encrypt with valid 32-byte key failed: %v", err)
	}
	if cipherHex == "" {
		t.Fatal("expected non-empty ciphertext")
	}
}

// TestInvalidKeyHex 验证无效的十六进制密钥返回错误
func TestInvalidKeyHex(t *testing.T) {
	_, err := Encrypt("test", "not-a-hex-string!!!")
	if err == nil {
		t.Fatal("expected error for invalid hex key, got nil")
	}
}

// TestTamperedCiphertext 验证篡改密文后解密失败
func TestTamperedCiphertext(t *testing.T) {
	key := testKeyHex()

	cipherHex, err := Encrypt("sensitive data", key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// 篡改密文：修改中间一个字符
	cipherBytes := []byte(cipherHex)
	mid := len(cipherBytes) / 2
	if cipherBytes[mid] == 'a' {
		cipherBytes[mid] = 'b'
	} else {
		cipherBytes[mid] = 'a'
	}
	tamperedHex := string(cipherBytes)

	_, err = Decrypt(tamperedHex, key)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext, got nil")
	}
}

// TestTamperedNonce 验证篡改 nonce 后解密失败
func TestTamperedNonce(t *testing.T) {
	key := testKeyHex()

	cipherHex, err := Encrypt("sensitive data", key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// 篡改 nonce（前 24 个 hex 字符 = 12 字节 nonce）
	cipherBytes := []byte(cipherHex)
	if cipherBytes[5] == 'f' {
		cipherBytes[5] = 'e'
	} else {
		cipherBytes[5] = 'f'
	}
	tamperedHex := string(cipherBytes)

	_, err = Decrypt(tamperedHex, key)
	if err == nil {
		t.Fatal("expected error for tampered nonce, got nil")
	}
}

// TestWrongKey 验证使用不同密钥解密失败
func TestWrongKey(t *testing.T) {
	key1 := testKeyHex()
	key2 := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	cipherHex, err := Encrypt("secret message", key1)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(cipherHex, key2)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key, got nil")
	}
}

// TestUniqueCiphertext 验证相同明文每次加密结果不同（nonce 随机性）
func TestUniqueCiphertext(t *testing.T) {
	key := testKeyHex()
	plaintext := "same message every time"

	c1, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("first Encrypt failed: %v", err)
	}

	c2, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("second Encrypt failed: %v", err)
	}

	if c1 == c2 {
		t.Error("expected different ciphertext for same plaintext (nonce should be random)")
	}
}

// TestDecrypt_ShortCiphertext 验证过短的密文返回错误
func TestDecrypt_ShortCiphertext(t *testing.T) {
	key := testKeyHex()

	_, err := Decrypt("aabb", key)
	if err == nil {
		t.Fatal("expected error for short ciphertext, got nil")
	}
}

// TestDecrypt_InvalidHex 验证无效的十六进制密文返回错误
func TestDecrypt_InvalidHex(t *testing.T) {
	key := testKeyHex()

	_, err := Decrypt("!!!invalid-hex!!!", key)
	if err == nil {
		t.Fatal("expected error for invalid hex ciphertext, got nil")
	}
}
