package api

import "github.com/cloudwego/hertz/pkg/app"

const CurrentUserIDKey = "current_user_id"

func CurrentUserID(c *app.RequestContext) string {
	value, ok := c.Get(CurrentUserIDKey)
	if !ok {
		return ""
	}

	userID, ok := value.(string)
	if !ok {
		return ""
	}

	return userID
}
