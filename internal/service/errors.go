package service

import "errors"

var ErrTooManyRequests = errors.New("too many llm requests")
