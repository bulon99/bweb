package msgo

import "strings"

func SubStringLast(str string, substr string) string { //从某个位置向后截取字符串
	index := strings.Index(str, substr)
	if index < 0 {
		return ""
	}
	return str[index+len(substr):]
}
