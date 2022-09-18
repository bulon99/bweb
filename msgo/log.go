package msgo //请求日志

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const ( //颜色
	greenBg   = "\033[97;42m"
	whiteBg   = "\033[90;47m"
	yellowBg  = "\033[90;43m"
	redBg     = "\033[97;41m"
	blueBg    = "\033[97;44m"
	magentaBg = "\033[97;45m"
	cyanBg    = "\033[97;46m"
	green     = "\033[32m"
	white     = "\033[37m"
	yellow    = "\033[33m"
	red       = "\033[31m"
	blue      = "\033[34m"
	magenta   = "\033[35m"
	cyan      = "\033[36m"
	reset     = "\033[0m"
)

type LoggerFormatter func(params LogFormatterParams) string

type LoggerConfig struct {
	Formatter      LoggerFormatter
	Out            io.Writer
	IsDisplayColor bool //控制是否展示彩色日志
}

type LogFormatterParams struct {
	Request        *http.Request
	TimeStamp      time.Time
	StatusCode     int
	Latency        time.Duration
	ClientIP       net.IP
	Method         string
	Path           string
	IsDisplayColor bool //控制是否展示彩色日志
}

func (p *LogFormatterParams) StatusCodeColor() string {
	code := p.StatusCode
	switch code {
	case http.StatusOK: //200为绿色
		return green
	default:
		return red //其他为红色
	}
}

func (p *LogFormatterParams) ResetColor() string {
	return reset
}

var defaultWriter io.Writer = os.Stdout //默认为标准输出

var defaultLogFormatter = func(params LogFormatterParams) string { //默认日志输出格式
	var statusCodeColor = params.StatusCodeColor()
	var resetColor = params.ResetColor()
	if params.Latency > time.Minute {
		params.Latency = params.Latency.Truncate(time.Second)
	}
	if params.IsDisplayColor {
		return fmt.Sprintf(" %s[msgo]%s  %s%v%s | %s%3d%s | %s%13v%s | %15s | %s%-7s%s %s%#v%s\n",
			yellow, resetColor, blue, params.TimeStamp.Format("2006/01/02 15:04:05"), resetColor,
			statusCodeColor, params.StatusCode, resetColor,
			red, params.Latency, resetColor,
			params.ClientIP,
			magenta, params.Method, resetColor,
			cyan, params.Path, resetColor,
		)
	}
	return fmt.Sprintf(" [msgo]  %v | %3d | %13v | %15s | %-7s %#v\n", //-7表示左对齐占7个位置，不加-则是右对齐
		params.TimeStamp.Format("2006/01/02 15:04:05"),
		params.StatusCode,
		params.Latency, params.ClientIP, params.Method, params.Path,
	)
}

var DefaultLoggerConfig = &LoggerConfig{
	Formatter:      defaultLogFormatter,
	Out:            defaultWriter,
	IsDisplayColor: true,
}

func LoggerWithConfig(conf LoggerConfig, next HandlerFunc) HandlerFunc {
	return func(ctx *Context) {
		param := LogFormatterParams{
			Request:        ctx.R,
			IsDisplayColor: conf.IsDisplayColor,
		}
		// Start timer
		start := time.Now()
		path := ctx.R.URL.Path
		raw := ctx.R.URL.RawQuery
		//执行业务
		next(ctx)
		// stop timer
		stop := time.Now()
		latency := stop.Sub(start)
		ip, _, _ := net.SplitHostPort(strings.TrimSpace(ctx.R.RemoteAddr))
		clientIP := net.ParseIP(ip)
		method := ctx.R.Method
		statusCode := ctx.StatusCode

		if raw != "" {
			path = path + "?" + raw
		}

		param.ClientIP = clientIP
		param.TimeStamp = stop
		param.Latency = latency
		param.StatusCode = statusCode
		param.Method = method
		param.Path = path
		fmt.Fprint(conf.Out, conf.Formatter(param))
	}
}

func Logging(next HandlerFunc) HandlerFunc {
	return LoggerWithConfig(*DefaultLoggerConfig, next)
}
