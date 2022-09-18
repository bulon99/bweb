package rpc

//基于http实现的rpc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

type HttpConfig struct {
	Protocol string
	Host     string
	Port     int
}

func (c HttpConfig) Prefix() string {
	if c.Protocol == "" {
		c.Protocol = HTTP
	}
	switch c.Protocol {
	case HTTP:
		return fmt.Sprintf("http://%s:%d", c.Host, c.Port)
	case HTTPS:
		return fmt.Sprintf("https://%s:%d", c.Host, c.Port)
	}
	return ""
}

const (
	HTTP     = "http"
	HTTPS    = "https"
	GET      = "GET"
	PostForm = "POST_FORM"
	PostJson = "POST_JSON"
)

type MsService interface {
	Env() HttpConfig
}

type MsHttpClient struct {
	client     http.Client
	serviceMap map[string]MsService //存储注册的服务
}

func NewHttpClient() *MsHttpClient {
	//transport 请求分发，协程安全，连接池
	client := http.Client{
		Timeout: time.Duration(3) * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   5,
			MaxConnsPerHost:       100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	return &MsHttpClient{client: client, serviceMap: make(map[string]MsService)}
}

func (c *MsHttpClient) responseHandle(request *http.Request) ([]byte, error) {
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		info := fmt.Sprintf("response status is %d", response.StatusCode)
		return nil, errors.New(info)
	}
	reader := bufio.NewReader(response.Body)
	defer response.Body.Close()
	var buf = make([]byte, 127)
	var body []byte
	for {
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if err == io.EOF || n == 0 { //读取完了
			break
		}
		body = append(body, buf[:n]...)
		if n < len(buf) {
			break //读完了
		}
	}
	return body, nil
}

func (c *MsHttpClient) Get(url string, args map[string]any, tracerInject func(header http.Header)) ([]byte, error) {
	if args != nil && len(args) > 0 {
		url = url + "?" + c.toValues(args)
	}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if tracerInject != nil {
		tracerInject(request.Header)
	}
	return c.responseHandle(request)
}

func (c *MsHttpClient) PostForm(url string, args map[string]any, tracerInject func(header http.Header)) ([]byte, error) {
	request, err := http.NewRequest(http.MethodPost, url, strings.NewReader(c.toValues(args)))
	if err != nil {
		return nil, err
	}
	if tracerInject != nil {
		tracerInject(request.Header)
	}
	return c.responseHandle(request)
}

func (c *MsHttpClient) PostJson(url string, args map[string]any, tracerInject func(header http.Header)) ([]byte, error) {
	marshal, _ := json.Marshal(args)
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(marshal))
	if err != nil {
		return nil, err
	}
	if tracerInject != nil {
		tracerInject(request.Header)
	}
	return c.responseHandle(request)
}

//其他update，delete实现类似

//生成url参数
func (c *MsHttpClient) toValues(args map[string]any) string {
	if args != nil && len(args) > 0 {
		params := url.Values{}
		for k, v := range args {
			params.Set(k, fmt.Sprintf("%v", v))
		}
		return params.Encode()
	}
	return ""
}

func (c *MsHttpClient) RegisterHttpService(name string, service MsService) {
	c.serviceMap[name] = service
}

func (c *MsHttpClient) Do(service string, method string, tracerInject func(header http.Header)) MsService { //返回包装好的MsService
	msService, ok := c.serviceMap[service]
	if !ok {
		panic(errors.New("service not found"))
	}
	t := reflect.TypeOf(msService)
	v := reflect.ValueOf(msService)
	if t.Kind() != reflect.Pointer {
		panic(errors.New("service not pointer"))
	}
	tVar := t.Elem()
	vVar := v.Elem()
	fieldIndex := -1
	for i := 0; i < tVar.NumField(); i++ {
		name := tVar.Field(i).Name //寻找method在结构体中的位置
		if name == method {
			fieldIndex = i
			break
		}
	}
	if fieldIndex == -1 {
		panic(errors.New("method not found"))
	}
	tag := tVar.Field(fieldIndex).Tag
	rpcInfo := tag.Get("msrpc")
	if rpcInfo == "" {
		panic(errors.New("not msrpc tag"))
	}
	split := strings.Split(rpcInfo, ",")
	if len(split) != 2 {
		panic(errors.New("tag msrpc not valid"))
	}
	methodType := split[0]
	path := split[1]
	httpConfig := msService.Env()
	f := func(args map[string]any) ([]byte, error) {
		if methodType == GET {
			return c.Get(httpConfig.Prefix()+path, args, tracerInject)
		}
		if methodType == PostJson {
			return c.PostJson(httpConfig.Prefix()+path, args, tracerInject)
		}
		if methodType == PostForm {
			return c.PostForm(httpConfig.Prefix()+path, args, tracerInject)
		}
		return nil, errors.New("no match method type")
	}
	fValue := reflect.ValueOf(f)
	vVar.Field(fieldIndex).Set(fValue) //将f赋值给method
	return msService
}
