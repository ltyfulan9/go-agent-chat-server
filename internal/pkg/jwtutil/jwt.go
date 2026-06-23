package jwtutil

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

func GenerateToken(userID, username, secret string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("jwt secret is required")
	}

	h := header{Alg: "HS256", Typ: "JWT"}
	claims := Claims{
		UserID:   userID,
		Username: username,
		Exp:      time.Now().Add(ttl).Unix(),
	}

	headerPart, err := encodeJSON(h)
	if err != nil {
		return "", err
	}

	payloadPart, err := encodeJSON(claims)
	if err != nil {
		return "", err
	}

	signingInput := headerPart + "." + payloadPart
	signature := sign(signingInput, secret)

	return signingInput + "." + signature, nil
}

func ParseToken(token, secret string) (Claims, error) {
	if secret == "" {
		return Claims{}, errors.New("jwt secret is required")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSignature := sign(signingInput, secret)
	if !hmac.Equal([]byte(expectedSignature), []byte(parts[2])) {
		return Claims{}, errors.New("invalid token signature")
	}

	var h header
	if err := decodeJSON(parts[0], &h); err != nil {
		return Claims{}, fmt.Errorf("decode jwt header failed: %w", err)
	}
	if h.Alg != "HS256" || h.Typ != "JWT" {
		return Claims{}, errors.New("unsupported token header")
	}

	var claims Claims
	if err := decodeJSON(parts[1], &claims); err != nil {
		return Claims{}, fmt.Errorf("decode jwt claims failed: %w", err)
	}
	if claims.UserID == "" {
		return Claims{}, errors.New("missing user_id")
	}
	if claims.Exp <= time.Now().Unix() {
		return Claims{}, errors.New("token expired")
	}

	return claims, nil
}

func encodeJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeJSON(value string, v interface{}) error {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func sign(signingInput, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
