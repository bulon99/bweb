package msgo

import (
	"encoding/base64"
	"net/http"
)

type Accounts struct {
	UnAuthHandler func(ctx *Context)
	Users         map[string]string
	Realm         string
}

func (a *Accounts) BasicAuth(next HandlerFunc) HandlerFunc {
	return func(ctx *Context) {
		//判断请求中是否有Authorization的Header
		username, password, ok := ctx.R.BasicAuth()
		if !ok {
			a.UnAuthHandlers(ctx)
			return
		}
		pw, ok := a.Users[username]
		if !ok {
			a.UnAuthHandlers(ctx)
			return
		}
		if pw != password {
			a.UnAuthHandlers(ctx)
			return
		}
		ctx.Set("user", username)
		next(ctx)
	}
}

func (a *Accounts) UnAuthHandlers(ctx *Context) { //若验证失败，使用该方法处理
	if a.UnAuthHandler != nil {
		a.UnAuthHandler(ctx)
	} else { //未设置该方法，返回401
		ctx.W.Header().Set("WWW-Authenticate", a.Realm) //Digest认证
		ctx.W.WriteHeader(http.StatusUnauthorized)
	}
}

func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
