package tracer

import (
	"context"
	"fmt"
	"github.com/bulon99/msgo"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"io"
	"net/http"
)

////////////////////////////////////////////////////////////////////////////////////
//http链路追踪相关函数

var DefaultSamplerConfig = &jaegercfg.SamplerConfig{
	Type:  jaeger.SamplerTypeConst,
	Param: 1,
}

var DefaultReporterConfig = &jaegercfg.ReporterConfig{
	LogSpans: true,
	// 按实际情况替换你的 ip
	CollectorEndpoint: "http://192.168.0.190:14268/api/traces",
}

func CreateTracer(servieName string) (opentracing.Tracer, io.Closer, error) { //创建tracer
	var cfg = jaegercfg.Configuration{
		ServiceName: servieName,
		Sampler:     DefaultSamplerConfig,
		Reporter:    DefaultReporterConfig,
	}
	jLogger := jaegerlog.StdLogger
	tracer, closer, err := cfg.NewTracer(jaegercfg.Logger(jLogger))
	return tracer, closer, err
}

func CreateTracerHeader(serviceName string, header http.Header) (opentracing.Tracer, opentracing.SpanContext, io.Closer, error) {
	var cfg = jaegercfg.Configuration{
		ServiceName: serviceName,
		Sampler:     DefaultSamplerConfig,
		Reporter:    DefaultReporterConfig,
	}
	jLogger := jaegerlog.StdLogger
	tracer, closer, err := cfg.NewTracer(jaegercfg.Logger(jLogger))
	// 继承别的进程传递过来的上下文
	spanContext, _ := tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(header))
	return tracer, spanContext, closer, err
}

func TracerInjectHttp(spanContext opentracing.SpanContext, tracer opentracing.Tracer) func(http.Header) {
	return func(header http.Header) {
		err := tracer.Inject(spanContext, opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(header))
		if err != nil {
			fmt.Println("tracer.Inject   ", err)
		}
	}
}

//中间件
func Tracer(serviceName string) msgo.MiddlewareFunc {
	return func(next msgo.HandlerFunc) msgo.HandlerFunc {
		return func(ctx *msgo.Context) {
			// 使用 opentracing.GlobalTracer() 获取全局 Tracer
			tracer, spanContext, closer, _ := CreateTracerHeader(serviceName, ctx.R.Header)
			defer closer.Close()
			// 生成依赖关系，并新建一个 span
			// 这里很重要，因为生成了  References []SpanReference 依赖关系
			startSpan := tracer.StartSpan(ctx.R.URL.Path, ext.RPCServerOption(spanContext))
			//startSpan := tracer.StartSpan(ctx.R.URL.Path, opentracing.ChildOf(spanContext)) //这样也可以，与上面的区别是，上面的有span.kind=server标签
			defer startSpan.Finish()
			// 记录 tag
			// 记录请求 Url
			ext.HTTPUrl.Set(startSpan, ctx.R.URL.Path)
			// Http Method
			ext.HTTPMethod.Set(startSpan, ctx.R.Method)
			// 记录组件名称
			ext.Component.Set(startSpan, "Msgo-Http")
			// 在 header 中加上当前进程的上下文信息
			ctx.R = ctx.R.WithContext(opentracing.ContextWithSpan(ctx.R.Context(), startSpan))
			next(ctx)
			// 继续设置 tag
			ext.HTTPStatusCode.Set(startSpan, uint16(ctx.StatusCode))
		}
	}
}

/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//grpc链路追踪相关函数

//grpc客户端
func ClientDialOption() grpc.DialOption {
	return grpc.WithUnaryInterceptor(jaegerGrpcClientInterceptor)
}

type TextMapWriter struct {
	metadata.MD
}

//重写TextMapWriter的Set方法，我们需要将carrier中的数据写入到metadata中，这样grpc才会携带。
func (t TextMapWriter) Set(key, val string) {
	//key = strings.ToLower(key)
	t.MD[key] = append(t.MD[key], val)
}

//客户端拦截器
func jaegerGrpcClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
	tracer, closer, _ := CreateTracer("order_Center")
	defer closer.Close()
	span := tracer.StartSpan("/find_Trace")
	defer span.Finish()
	md, ok := metadata.FromIncomingContext(ctx) //从上下文中拿到metadata
	if !ok {
		md = metadata.New(nil) //若没有则新建一个
	} else {
		//如果对metadata进行修改，那么需要用拷贝的副本进行修改。（FromIncomingContext的注释）
		md = md.Copy()
	}
	//定义一个carrier，下面的Inject注入数据需要用到
	//carrier := opentracing.TextMapCarrier{}
	carrier := TextMapWriter{md}
	//将span的context信息注入到carrier中
	e := tracer.Inject(span.Context(), opentracing.TextMap, carrier)
	if e != nil {
		fmt.Println("tracer Inject err,", e)
	}
	//创建一个新的context，把metadata附带上
	ctx = metadata.NewOutgoingContext(ctx, md)

	return invoker(ctx, method, req, reply, cc, opts...)
}

//grpc服务端
func ServerOption() grpc.ServerOption {
	return grpc.UnaryInterceptor(jaegerGrpcServerInterceptor)
}

type TextMapReader struct {
	metadata.MD
}

//读取metadata中的span信息
func (t TextMapReader) ForeachKey(handler func(key, val string) error) error { //不能是指针
	for key, val := range t.MD {
		for _, v := range val {
			if err := handler(key, v); err != nil {
				return err
			}
		}
	}
	return nil
}

//服务端拦截器
func jaegerGrpcServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	//从context中获取metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		//如果对metadata进行修改，那么需要用拷贝的副本进行修改。（FromIncomingContext的注释）
		md = md.Copy()
	}
	carrier := TextMapReader{md}
	tracer, closer, _ := CreateTracer("goods_Center")
	defer closer.Close()
	spanContext, e := tracer.Extract(opentracing.TextMap, carrier)
	if e != nil {
		fmt.Println("Extract err:", e)
	}
	span := tracer.StartSpan(info.FullMethod, opentracing.ChildOf(spanContext))
	defer span.Finish()
	ctx = opentracing.ContextWithSpan(ctx, span)
	return handler(ctx, req)
}
