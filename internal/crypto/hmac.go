package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// HMACSign 计算给定 payload 的 HMAC-SHA256，返回十六进制编码结果。
func HMACSign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// HMACVerify 检查 payload 的 HMAC-SHA256 签名是否与预期的十六进制值匹配。
func HMACVerify(payload []byte, signatureHex, secret string) bool {
	expected, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	actual := mac.Sum(nil)
	return hmac.Equal(actual, expected)
}

// FormatHMACSignature 返回给定 payload 和 secret 的头部值 "sha256=<hex>"。
func FormatHMACSignature(payload []byte, secret string) string {
	return fmt.Sprintf("sha256=%s", HMACSign(payload, secret))
}
