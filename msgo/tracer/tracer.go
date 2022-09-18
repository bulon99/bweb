package tracer

import (
	"fmt"
	"github.com/bulon99/msgo"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"io"
	"net/http"
)

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
