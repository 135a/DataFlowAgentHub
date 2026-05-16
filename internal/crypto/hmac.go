package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// HMACSign computes HMAC-SHA256 of the given payload and returns the hex-encoded result.
func HMACSign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// HMACVerify checks whether the HMAC-SHA256 signature of payload matches the expected hex-encoded value.
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

// FormatHMACSignature returns the header value "sha256=<hex>" for a given payload and secret.
func FormatHMACSignature(payload []byte, secret string) string {
	return fmt.Sprintf("sha256=%s", HMACSign(payload, secret))
}
