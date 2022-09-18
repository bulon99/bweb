package render

import (
	"net/http"
)

type Render interface {
	Render(w http.ResponseWriter) error
	WriteContentType(w http.ResponseWriter)
}

func writeContentType(w http.ResponseWriter, value string) {
	w.Header().Set("Content-Type", value)
	//w.WriteHeader()和w.Header().Set()同时使用时，w.WriteHeader()方法必须在所有w.Header().Set（）之后出现
}
