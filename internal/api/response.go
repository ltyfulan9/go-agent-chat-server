package api

import "go-agent-chat-server/internal/ecode"

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
	return Response{
		Code:    ecode.CodeOK,
		Message: "ok",
		Data:    data,
	}
}

func Error(code int, message string) Response {
	return Response{
		Code:    code,
		Message: message,
	}
}
