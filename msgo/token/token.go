package token

import (
	"errors"
	"github.com/bulon99/msgo"
	"github.com/dgrijalva/jwt-go"
	"net/http"
	"time"
)

const JWTToken = "jwt_token"

type JwtHandler struct {
	//算法
	Alg string
	//登录认证
	Authenticator func(ctx *msgo.Context) (map[string]any, error) //自定义的处理逻辑
	//过期时间 秒
	TimeOut        time.Duration
	RefreshTimeOut time.Duration
	//时间函数 从此时开始计算过期
	TimeFuc func() time.Time
	//私钥
	PrivateKey []byte
	//key
	Key []byte
	//获取refreshToken的键
	RefreshKey string
	//save cookie
	SendCookie     bool //是否返回cookie
	CookieName     string
	CookieMaxAge   int
	CookieDomain   string
	SecureCookie   bool
	CookieHTTPOnly bool
	Header         string
	AuthHandler    func(ctx *msgo.Context, err error)
}

type JWTResponse struct {
	Token        string
	RefreshToken string
}

//登录处理，登录成功后，使用jwt生成token
func (j *JwtHandler) LoginHandler(ctx *msgo.Context) (*JWTResponse, error) {
	data, err := j.Authenticator(ctx)
	if err != nil {
		return nil, err
	}
	if j.Alg == "" {
		j.Alg = "HS256"
	}
	signingMethod := jwt.GetSigningMethod(j.Alg)
	token := jwt.New(signingMethod)
	claims := token.Claims.(jwt.MapClaims) //token第二段
	if data != nil {
		for key, value := range data {
			claims[key] = value
		}
	}
	if j.TimeFuc == nil { //若未设置起始时间，以当前时间为准
		j.TimeFuc = func() time.Time {
			return time.Now()
		}
	}
	expire := j.TimeFuc().Add(j.TimeOut)
	claims["exp"] = expire.Unix()
	claims["iat"] = j.TimeFuc().Unix()
	var tokenString string
	var errToken error
	if j.usingPublicKeyAlgo() {
		tokenString, errToken = token.SignedString(j.PrivateKey)
	} else {
		tokenString, errToken = token.SignedString(j.Key)
	}
	if errToken != nil {
		return nil, errToken
	}
	jr := &JWTResponse{
		Token: tokenString,
	}
	refreshToken, err := j.refreshToken(token)
	if err != nil {
		return nil, err
	}
	jr.RefreshToken = refreshToken
	if j.SendCookie {
		if j.CookieName == "" {
			j.CookieName = JWTToken
		}
		if j.CookieMaxAge == 0 {
			j.CookieMaxAge = int(expire.Unix() - j.TimeFuc().Unix()) //token存活时间
		}
		maxAge := j.CookieMaxAge
		ctx.SetCookie(j.CookieName, tokenString, maxAge, "/", j.CookieDomain, j.SecureCookie, j.CookieHTTPOnly)
	}
	return jr, nil
}

func (j *JwtHandler) refreshToken(token *jwt.Token) (string, error) { //返回一个过期时间更长的RefreshToken，用于刷新Token
	claims := token.Claims.(jwt.MapClaims)
	expire := j.TimeFuc().Add(j.RefreshTimeOut) //设置refreshToken过期时间
	claims["exp"] = expire.Unix()
	var tokenString string
	var errToken error
	if j.usingPublicKeyAlgo() {
		tokenString, errToken = token.SignedString(j.PrivateKey)
	} else {
		tokenString, errToken = token.SignedString(j.Key)
	}
	if errToken != nil {
		return "", errToken
	}
	return tokenString, nil
}

//退出登录
func (j *JwtHandler) LogoutHandler(ctx *msgo.Context) error {
	//清除cookie即可
	if j.SendCookie {
		if j.CookieName == "" {
			j.CookieName = JWTToken
		}
		ctx.SetCookie(j.CookieName, "", -1, "/", j.CookieDomain, j.SecureCookie, j.CookieHTTPOnly)
		return nil
	}
	return nil
}

func (j *JwtHandler) usingPublicKeyAlgo() bool {
	switch j.Alg {
	case "RS256", "RS512", "RS384":
		return true
	}
	return false
}

//刷新Token
func (j *JwtHandler) RefreshHandler(ctx *msgo.Context) (*JWTResponse, error) {
	//检测refresh token是否过期
	rToken, ok := ctx.Get(j.RefreshKey)
	if !ok {
		return nil, errors.New("refresh token is null")
	}
	if j.Alg == "" {
		j.Alg = "HS256"
	}
	t, err := jwt.Parse(rToken.(string), func(token *jwt.Token) (interface{}, error) {
		if j.usingPublicKeyAlgo() {
			return j.PrivateKey, nil
		} else {
			return j.Key, nil
		}
	})
	if err != nil { //refresh token不合法
		return nil, err
	}
	claims := t.Claims.(jwt.MapClaims)
	if j.TimeFuc == nil { //若未设置起始时间，以当前时间为准
		j.TimeFuc = func() time.Time {
			return time.Now()
		}
	}
	expire := j.TimeFuc().Add(j.TimeOut)
	claims["exp"] = expire.Unix()
	claims["iat"] = j.TimeFuc().Unix()
	var tokenString string
	var errToken error
	if j.usingPublicKeyAlgo() {
		tokenString, errToken = t.SignedString(j.PrivateKey)
	} else {
		tokenString, errToken = t.SignedString(j.Key)
	}
	if errToken != nil {
		return nil, errToken
	}
	jr := &JWTResponse{
		Token: tokenString,
	}
	refreshToken, err := j.refreshToken(t)
	if err != nil {
		return nil, err
	}
	jr.RefreshToken = refreshToken
	if j.SendCookie {
		if j.CookieName == "" {
			j.CookieName = JWTToken
		}
		if j.CookieMaxAge == 0 {
			j.CookieMaxAge = int(expire.Unix() - j.TimeFuc().Unix()) //token存活时间
		}
		maxAge := j.CookieMaxAge
		ctx.SetCookie(j.CookieName, tokenString, maxAge, "/", j.CookieDomain, j.SecureCookie, j.CookieHTTPOnly)
	}
	return jr, nil
}

//jwt认证中间件
func (j *JwtHandler) AuthInterceptor(next msgo.HandlerFunc) msgo.HandlerFunc {
	return func(ctx *msgo.Context) {
		if j.Header == "" {
			j.Header = "Authorization"
		}
		token := ctx.R.Header.Get(j.Header)
		if token == "" {
			if j.SendCookie { //若headers和cookie中都没有token
				token = ctx.GetCookie(j.CookieName)
				if token == "" {
					j.AuthErrorHandler(ctx, nil)
					return
				}
			}
		}
		if token == "" {
			j.AuthErrorHandler(ctx, errors.New("token is null"))
			return
		}
		t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			if j.usingPublicKeyAlgo() {
				return j.PrivateKey, nil
			}
			return j.Key, nil
		})
		if err != nil {
			j.AuthErrorHandler(ctx, err)
			return
		}
		ctx.Set("jwt_claims", t.Claims.(jwt.MapClaims))
		next(ctx)
	}
}

func (j *JwtHandler) AuthErrorHandler(ctx *msgo.Context, err error) {
	if j.AuthHandler == nil {
		ctx.W.WriteHeader(http.StatusUnauthorized)
	} else {
		j.AuthHandler(ctx, nil)
	}
}
