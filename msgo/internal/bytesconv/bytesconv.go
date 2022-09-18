package bytesconv

import "unsafe"

func StringToBytes(s string) []byte { //将string转为[]byte
	return *(*[]byte)(unsafe.Pointer( //使用unsafe.Pointer时的写法
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}
