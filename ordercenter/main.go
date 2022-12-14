package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/afex/hystrix-go/hystrix"
	"github.com/bulon99/goodscenter/api"
	"github.com/bulon99/goodscenter/model"
	"github.com/bulon99/msgo"
	"github.com/bulon99/msgo/breaker"
	"github.com/bulon99/msgo/rpc"
	trace "github.com/bulon99/msgo/tracer"
	"github.com/bulon99/ordercenter/service"
	"github.com/opentracing/opentracing-go/ext"
	"net/http"
)

func main() {
	engine := msgo.Default()
	group := engine.Group("order")

	//http实现rpc客户端
	client := rpc.NewHttpClient()
	client.RegisterHttpService("goods", &service.GoodsService{}) //注册服务
	group.Get("/find", func(ctx *msgo.Context) {
		//通过商品中心查询商品
		params := make(map[string]any)
		params["id"] = 1000
		params["name"] = "alen"
		//body, err := client.Get("http://localhost:9002/goods/find", params) //实现rpc
		//body, err := client.PostJson("http://localhost:9002/goods/find", params)
		//body, err := client.PostForm("http://localhost:9002/goods/find", params)
		body, err := client.Do("goods", "Find", nil).(*service.GoodsService).Find(params) //使用服务名和方法名，实现rpc
		if err != nil {
			panic(err)
		}
		v := &model.Result{}
		json.Unmarshal(body, v)
		ctx.JSON(200, v)
	}, msgo.Limiter(1, 1)) //使用限流中间件

	//引入hystrix熔断
	hystrix.ConfigureCommand("mycommand", breaker.DefaultHystrix)
	group.Get("/find_1", func(ctx *msgo.Context) {
		_ = hystrix.Do("mycommand", func() error {
			body, err := client.Do("goods", "Find", nil).(*service.GoodsService).Find(nil)
			if err != nil {
				return err
			}
			v := &model.Result{}
			_ = json.Unmarshal(body, v)
			_ = ctx.JSON(200, v)
			return nil
		}, func(err error) error {
			fmt.Println(err.Error()) //超过并发时， hystrix: max concurrency
			return nil
		})
	})

	//链路追踪
	group.Get("/findTrace", func(ctx *msgo.Context) {
		tracer, closer, _ := trace.CreateTracer("orderCenter")
		defer closer.Close()                   //请求响应完成后要关闭，否则jaeger会无记录
		span := tracer.StartSpan("/findTrace") //根据当前路径名创建span
		defer span.Finish()                    //请求响应完成后要关闭，否则jaeger会无记录
		//设置一些标签
		ext.SpanKindRPCClient.Set(span)
		ext.HTTPUrl.Set(span, "localhost:9003/order/findTrace")
		ext.HTTPMethod.Set(span, "GET")
		f := trace.TracerInjectHttp(span.Context(), tracer) //将spantext添加到header的操作封装为一个闭包函数
		body, err := client.Do("goods", "FindTrace", f).(*service.GoodsService).FindTrace(nil)
		if err != nil {
			panic(err)
		}
		v := &model.Result{}
		_ = json.Unmarshal(body, v)
		_ = ctx.JSON(200, v)
	})

	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	//grpc客户端
	//若下面的这部分没有注释，在goodscenter启动的是grpc服务

	//熔断
	config := rpc.DefaultGrpcClientConfig()
	config.Address = "localhost:9002"
	grpcClient, _ := rpc.NewGrpcClient(config)
	defer grpcClient.Conn.Close()
	goodsApiClient := api.NewGoodsApiClient(grpcClient.Conn)
	group.Get("/findGrpc", func(ctx *msgo.Context) {
		//goodsResponse, _ := goodsApiClient.Find(context.Background(), &api.GoodsRequest{}) //本地调用
		//ctx.JSON(http.StatusOK, goodsResponse)
		_ = hystrix.Do("mycommand", func() error { //hystrix熔断
			goodsResponse, err := goodsApiClient.Find(context.Background(), &api.GoodsRequest{}) //本地调用
			if err != nil {
				return err
			}
			ctx.JSON(http.StatusOK, goodsResponse)
			return nil
		}, func(err error) error {
			fmt.Println(err.Error()) //超过并发时， hystrix: max concurrency
			return nil
		})
	})

	//链路追踪
	config1 := rpc.DefaultGrpcClientConfig()
	config1.Address = "localhost:9002"
	config1.DialOptions = append(config1.DialOptions, trace.ClientDialOption()) //在客户端添加链路追踪拦截器
	grpcClient1, _ := rpc.NewGrpcClient(config1)
	defer grpcClient1.Conn.Close()
	goodsApiClient1 := api.NewGoodsApiClient(grpcClient1.Conn)
	group.Get("/find_Trace", func(ctx *msgo.Context) {
		goodsResponse, _ := goodsApiClient1.Find(context.Background(), &api.GoodsRequest{}) //本地调用
		ctx.JSON(http.StatusOK, goodsResponse)
	})

	engine.Run("localhost:9003")
}

//namingClient, err := register.CreateNacosClient() //从nacos获取goods服务地址
//if err != nil {
//	log.Println(err)
//}
//host, port, _ := register.GetInstance(namingClient, "goods") //通过服务名获取
//config.Address = fmt.Sprintf("%v:%d", host, port)
//
//cli, err := register.CreateEtcdCli(register.DefaultOption) //从etcd获取goods服务地址
//if err != nil {
//	fmt.Println(err)
//}
//addr, err := register.GetEtcdValue(cli, "goods")
//if err != nil {
//	fmt.Println(err)
//}
//config.Address = addr
