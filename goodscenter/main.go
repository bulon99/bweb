package main

import (
	"fmt"
	"github.com/bulon99/goodscenter/model"
	"github.com/bulon99/msgo"
	trace "github.com/bulon99/msgo/tracer"
	"net/http"
)

func main() {
	//http实现rpc服务端
	engine := msgo.Default()
	group := engine.Group("goods")
	group.Any("/find", func(ctx *msgo.Context) {
		fmt.Println("from:", ctx.GetHeader("from")) //从网关来的请求header中才有from字段
		goods := &model.Goods{Id: 1000, Name: "战66"}
		ctx.JSON(http.StatusOK, &model.Result{Code: 200, Msg: "Success", Data: goods})
	})

	//链路追踪
	group.Any("/tracer", func(ctx *msgo.Context) {
		fmt.Println("from:", ctx.GetHeader("from")) //从网关来的请求header中才有from字段
		goods := &model.Goods{Id: 1000, Name: "链路追踪商品"}
		ctx.JSON(http.StatusOK, &model.Result{Code: 200, Msg: "Success", Data: goods})
	}, trace.Tracer("goodsCenter"))
	engine.Run("localhost:9002")

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	//grpc服务端
	//server, _ := rpc.NewGrpcServer("localhost:9002")
	//server.Register(func(g *grpc.Server) {
	//	api.RegisterGoodsApiServer(g, &api.GoodsRpcServices{})
	//}, "goods", "localhost", 9002, "") //nacos etcd 两种注册服务方式
	//err := server.Run()
	//log.Println(err)
}
