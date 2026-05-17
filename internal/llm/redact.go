package llm

import "strings"

// RedactAuthHeader 对类似 Authorization 的头部进行脱敏以记录日志
func RedactAuthHeader(s string) string {
	if s == "" {
		return ""
	}
	if len(s) < 20 {
		return "***"
	}
	return s[:8] + "***" + s[len(s)-4:]
}

// RedactAPIKey 对 API 密钥进行脱敏处理
func RedactAPIKey(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-2:]
}
