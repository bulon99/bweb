package service

import "github.com/bulon99/msgo/rpc"

type GoodsService struct {
	Find      func(args map[string]any) ([]byte, error) `msrpc:"GET,/goods/find"` //标签用于实例化该方法
	FindTrace func(args map[string]any) ([]byte, error) `msrpc:"GET,/goods/tracer"`
	//Find func(args map[string]any) ([]byte, error) `msrpc:"POST_JSON,/goods/find"`
	//Find func(args map[string]any) ([]byte, error) `msrpc:"POST_FORM,/goods/find"`
}

func (*GoodsService) Env() rpc.HttpConfig {
	return rpc.HttpConfig{
		Protocol: rpc.HTTP,
		Host:     "localhost",
		Port:     9002,
	}
}
