package auth

// A minimal, dependency-free signed token (HMAC-SHA256 over a JSON claims
// payload, base64url-encoded — structurally the same idea as a JWT, kept
// hand-rolled here so the module has no extra third-party token library).

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bruhjeshhh/delivery-tracker/internal/models"
)

type Claims struct {
	UserID int64           `json:"uid"`
	Role   models.UserRole `json:"role"`
	Exp    int64           `json:"exp"`
}

var ErrInvalidToken = errors.New("invalid or expired token")

func enc(b []byte) string          { return base64.RawURLEncoding.EncodeToString(b) }
func dec(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

func sign(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return enc(mac.Sum(nil))
}

func GenerateToken(secret string, userID int64, role models.UserRole, ttl time.Duration) (string, error) {
	claims := Claims{UserID: userID, Role: role, Exp: time.Now().Add(ttl).Unix()}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := enc(payloadBytes)
	sig := sign(secret, payload)
	return fmt.Sprintf("%s.%s", payload, sig), nil
}

func ParseToken(secret, token string) (*Claims, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}
	payload, sig := parts[0], parts[1]
	expectedSig := sign(secret, payload)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, ErrInvalidToken
	}
	payloadBytes, err := dec(payload)
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if time.Now().Unix() > claims.Exp {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}
