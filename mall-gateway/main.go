package main

import (
	"github.com/bulon99/msgo"
	"github.com/bulon99/msgo/gateway"
	"net/http"
)

func main() {
	engine := msgo.Default()
	engine.OpenGateway = true
	var configs []gateway.GWConfig
	configs = append(configs, gateway.GWConfig{
		Name: "order",
		Path: "/order/**",
		Host: "127.0.0.1", //这里的host和port可通过集成注册中心来获取（未实现）
		Port: 9003,
		Header: func(req *http.Request) {
			req.Header.Set("from", "gateway")
		},
		ServiceName: "orderCenter",
	}, gateway.GWConfig{
		Name: "goods",
		Path: "/goods/**",
		Host: "127.0.0.1", //这里的host和port可通过集成注册中心来获取（未实现）
		Port: 9002,
		Header: func(req *http.Request) {
			req.Header.Set("from", "gateway")
		},
		ServiceName: "goodsCenter",
	})
	engine.SetGatewayConfig(configs)
	engine.Run("127.0.0.1:80") //通过80端口转发
}
