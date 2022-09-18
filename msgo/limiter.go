package msgo

import (
	"context"
	"golang.org/x/time/rate"
	"net/http"
	"time"
)

//限流中间件
func Limiter(limit, cap int) MiddlewareFunc {
	li := rate.NewLimiter(rate.Limit(limit), cap)
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) {
			con, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := li.WaitN(con, 1)
			if err != nil {
				ctx.String(http.StatusForbidden, "限流了")
				return
			}
			next(ctx)
		}
	}
}
