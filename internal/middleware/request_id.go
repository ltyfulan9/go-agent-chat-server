package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

const RequestIDKey = "request_id"

func RequestID() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		requestID := string(c.Request.Header.Peek("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}

		c.Set(RequestIDKey, requestID)
		c.Response.Header.Set("X-Request-ID", requestID) //设置到响应头中
		c.Next(ctx)
	}
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b[:]))
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
