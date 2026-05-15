package llm

import "strings"

// RedactAuthHeader masks Authorization-like headers for logs.
func RedactAuthHeader(s string) string {
	if s == "" {
		return ""
	}
	if len(s) < 20 {
		return "***"
	}
	return s[:8] + "***" + s[len(s)-4:]
}

// RedactAPIKey masks API key material.
func RedactAPIKey(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-2:]
}
