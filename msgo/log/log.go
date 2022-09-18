package log //用户自己定义的日志

import (
	"fmt"
	"io"
	"os"
	"path"
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

//级别
type LoggerLevel int

func (level LoggerLevel) Level() string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelError:
		return "ERROR"
	default:
		return ""
	}
}

const (
	LevelDebug LoggerLevel = iota
	LevelInfo
	LevelError
)

//日志
type Logger struct {
	Formatter    LoggingFormatter
	Level        LoggerLevel
	Outs         []*LoggerWriter
	LoggerFields Fields
	logPath      string
}

type LoggerWriter struct {
	Level LoggerLevel
	Out   io.Writer
}

type LoggingFormatter interface { //日志格式化接口
	Formatter(param *LoggingFormatterParam) string
}

type LoggingFormatterParam struct {
	Color        bool
	Level        LoggerLevel
	Msg          any
	LoggerFields Fields
}

type Fields map[string]any //字段信息

type LoggerFormatter struct {
	Color        bool
	Level        LoggerLevel
	LoggerFields Fields
}

func New() *Logger {
	return &Logger{}
}

func Default() *Logger {
	logger := New()
	logger.Level = LevelDebug //默认是debug级别
	logger.Outs = append(logger.Outs, &LoggerWriter{Level: LevelDebug, Out: os.Stdout})
	logger.Formatter = &TextFormatter{}
	return logger
}

func (l *Logger) Info(msg any) {
	l.Print(LevelInfo, msg)
}

func (l *Logger) Debug(msg any) {
	l.Print(LevelDebug, msg)
}

func (l *Logger) Error(msg any) {
	l.Print(LevelError, msg)
}

func (l *Logger) WithFields(fields Fields) *Logger { //添加字段
	return &Logger{
		Formatter:    l.Formatter,
		Outs:         l.Outs,
		Level:        l.Level,
		LoggerFields: fields,
	}
}

func (l *Logger) Print(level LoggerLevel, msg any) {
	if l.Level > level {
		//级别不满足 不打印日志
		return
	}
	param := &LoggingFormatterParam{
		Level:        level,
		Msg:          msg,
		LoggerFields: l.LoggerFields,
	}
	for _, out := range l.Outs {
		if out.Out == os.Stdout { //标准输出
			param.Color = true //只在控制台输出颜色，只对text格式日志生效
			str := l.Formatter.Formatter(param)
			fmt.Fprint(out.Out, str)
		} else if out.Level == -1 || level == out.Level { //日志输出到对应文件
			param.Color = false
			str := l.Formatter.Formatter(param)
			fmt.Fprint(out.Out, str)
		}
	}
}

func (l *Logger) SetLogPath(logPath string) { //将不同等级的日志输出到不同文件
	l.logPath = logPath
	l.Outs = append(l.Outs, &LoggerWriter{Level: -1, Out: FileWriter(path.Join(logPath, "all.log"))})
	l.Outs = append(l.Outs, &LoggerWriter{Level: LevelDebug, Out: FileWriter(path.Join(logPath, "debug.log"))})
	l.Outs = append(l.Outs, &LoggerWriter{Level: LevelInfo, Out: FileWriter(path.Join(logPath, "info.log"))})
	l.Outs = append(l.Outs, &LoggerWriter{Level: LevelError, Out: FileWriter(path.Join(logPath, "error.log"))})
}

func FileWriter(name string) io.Writer {
	w, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	return w
}
