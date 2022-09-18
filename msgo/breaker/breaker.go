package breaker

import (
	"github.com/afex/hystrix-go/hystrix"
)

var DefaultHystrix = hystrix.CommandConfig{
	Timeout:                3000, //超时时间ms
	MaxConcurrentRequests:  1,    //最大并发量
	SleepWindow:            5000, //熔断器被打开后，SleepWindow的时间就是控制过多久后去尝试服务是否可用了。
	RequestVolumeThreshold: 10,   //一个统计窗口，10秒内请求数量。达到这个请求数量后才去判断是否要开启熔断
	ErrorPercentThreshold:  30,   //错误百分比，请求数量大于等于 RequestVolumeThreshold 并且错误率到达这个百分比后就会启动熔断
}
