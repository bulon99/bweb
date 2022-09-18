package gateway

import "net/http"

type GWConfig struct {
	Name        string
	Path        string
	Host        string
	Port        int
	Header      func(req *http.Request) //设置header
	ServiceName string
}
