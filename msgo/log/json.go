package log

import (
	"encoding/json"
	"fmt"
	"time"
)

type JsonFormatter struct {
	TimeDisplay bool
}

func (f *JsonFormatter) Formatter(param *LoggingFormatterParam) string {
	now := time.Now()
	if param.LoggerFields == nil {
		param.LoggerFields = make(Fields)
	}
	if f.TimeDisplay {
		timeNow := now.Format("2006/01/02 15:04:05")
		param.LoggerFields["log_time"] = timeNow
	}

	param.LoggerFields["msg"] = param.Msg
	param.LoggerFields["log_level"] = param.Level.Level()
	marshal, _ := json.Marshal(param.LoggerFields)
	return fmt.Sprint(string(marshal)) + "\n"
}
