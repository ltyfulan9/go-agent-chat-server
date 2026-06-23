package api

import (
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type PageData struct {
	Items    interface{} `json:"items"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int64       `json:"total"`
}

func ParsePageQuery(c *app.RequestContext) (int, int) {
	page := parsePositiveInt(c.Query("page"), DefaultPage)
	pageSize := parsePositiveInt(c.Query("page_size"), DefaultPageSize)

	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	return page, pageSize
}

func parsePositiveInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}

	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}

	return n
}
