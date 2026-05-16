package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID      string `json:"sub"`
	WorkspaceID string `json:"wid"`
	Role        string `json:"role"`
	jwt.RegisteredClaims
}

func Sign(secret []byte, userID, workspaceID, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	c := Claims{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Role:        role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(secret)
}

// SignSSEToken issues a short-lived token for SSE EventSource connections.
// The token has minimal scope: it only allows subscribing to a specific session's events.
func SignSSEToken(secret []byte, userID, workspaceID, sessionID string) (string, error) {
	now := time.Now()
	c := Claims{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Role:        "viewer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
			Subject:   sessionID,
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(secret)
}

func Parse(secret []byte, token string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}
