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

type Claims struct { //是token里面真正存的用户信息
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Exp      int64  `json:"exp"` //过期时间
}

type header struct {
	Alg string `json:"alg"` //签名算法，这里是HS256
	Typ string `json:"typ"` //typ类型，这里是JWT
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

	headerPart, err := encodeJSON(h) //转成JSON，把JSON转成Base64URL字符串
	if err != nil {
		return "", err
	}

	payloadPart, err := encodeJSON(claims) //转成JSON，把JSON转成Base64URL字符串
	if err != nil {
		return "", err
	}

	signingInput := headerPart + "." + payloadPart //xxxxx.yyyyy
	signature := sign(signingInput, secret)        //通过HMAC-SHA256算法和secret对签名输入进行签名，得到签名字符串

	return signingInput + "." + signature, nil //拼出完整token
}

func ParseToken(token, secret string) (Claims, error) { //token：客户端传来的JWT，secret服务端自己的签名密钥
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
	if err := decodeJSON(parts[0], &h); err != nil { //header还原成json，alg+typ结构体
		return Claims{}, fmt.Errorf("decode jwt header failed: %w", err)
	}
	if h.Alg != "HS256" || h.Typ != "JWT" { //检查支不支持JWT
		return Claims{}, errors.New("unsupported token header")
	}

	var claims Claims //这一步解析第二段Payload，得到用户信息和过期时间等
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
