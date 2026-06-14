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

		c.Set(RequestIDKey, requestID) //存到请求上下文里，方便后续日志等使用
		c.Response.Header.Set("X-Request-ID", requestID)
		c.Next(ctx) //RequestID中间件处理完了，继续执行后面的中间件或者真正的handler
	}
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b[:]))
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
