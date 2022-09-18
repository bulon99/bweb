package msgo

import (
	"fmt"
	"github.com/bulon99/msgo/gateway"
	msLog "github.com/bulon99/msgo/log"
	"github.com/bulon99/msgo/render"
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

const ANY = "ANY"

type HandlerFunc func(ctx *Context)

//中间件函数，传入一个HandlerFunc，在这个HandlerFunc前或者后加上要实现的代码作为新的HandlerFunc返回，从而实现前或者后中间件
type MiddlewareFunc func(handleFunc HandlerFunc) HandlerFunc

//路由组
type routerGroup struct {
	name               string
	handlerFuncMap     map[string]map[string]HandlerFunc      //map["/index"]map["GET"]HandlerFunc
	middlewaresFuncMap map[string]map[string][]MiddlewareFunc //map["/index"]map["GET"][]MiddlewareFunc
	treeNode           *treeNode
	middlewares        []MiddlewareFunc //组中间件
}

func (r *routerGroup) Use(middlewareFunc ...MiddlewareFunc) { //添加中间件
	r.middlewares = append(r.middlewares, middlewareFunc...)
}

func (r *routerGroup) methodHandle(name string, method string, h HandlerFunc, ctx *Context) {
	//组中间件
	if r.middlewares != nil {
		for _, middlewareFunc := range r.middlewares {
			h = middlewareFunc(h)
		}
	}
	//路由方法级别的中间件
	middlewareFuncs := r.middlewaresFuncMap[name][method]
	if middlewareFuncs != nil {
		for _, middlewareFunc := range middlewareFuncs {
			h = middlewareFunc(h)
		}
	}
	h(ctx)
}

func (r *routerGroup) handle(name string, method string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	_, ok := r.handlerFuncMap[name]
	if !ok {
		r.handlerFuncMap[name] = make(map[string]HandlerFunc)
		r.middlewaresFuncMap[name] = make(map[string][]MiddlewareFunc)
	}
	_, ok = r.handlerFuncMap[name][method]
	if ok {
		panic("路由重复")
	}
	r.handlerFuncMap[name][method] = handlerFunc
	r.middlewaresFuncMap[name][method] = append(r.middlewaresFuncMap[name][method], middlewareFunc...)
	r.treeNode.Put(name)
}

func (r *routerGroup) Any(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, ANY, handlerFunc, middlewareFunc...)
}

func (r *routerGroup) Get(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, http.MethodGet, handlerFunc, middlewareFunc...)
}

func (r *routerGroup) Post(name string, handlerFunc HandlerFunc, middlewareFunc ...MiddlewareFunc) {
	r.handle(name, http.MethodPost, handlerFunc, middlewareFunc...)
}

//其他方法head put patch Delete

//路由
type router struct {
	groups []*routerGroup
	engine *Engine
}

func (r *router) Group(name string) *routerGroup {
	g := &routerGroup{
		name:               name,
		handlerFuncMap:     make(map[string]map[string]HandlerFunc),
		middlewaresFuncMap: make(map[string]map[string][]MiddlewareFunc),
		treeNode:           &treeNode{name: "/", children: make([]*treeNode, 0)},
	}
	g.Use(r.engine.Middles...)
	r.groups = append(r.groups, g)
	return g
}

//引擎
type Engine struct {
	*router
	funcMap          template.FuncMap
	HTMLRender       render.HTMLRender
	pool             sync.Pool //保存和复用临时对象，减少内存分配，降低 GC 压力。解决频繁创建context
	Logger           *msLog.Logger
	Middles          []MiddlewareFunc
	gatewayConfigs   []gateway.GWConfig //网关配置
	OpenGateway      bool               //是否开启网关
	gatewayTreeNode  *gateway.TreeNode
	gatewayConfigMap map[string]gateway.GWConfig
}

func New() *Engine {
	engine := &Engine{
		router:           &router{},
		gatewayTreeNode:  &gateway.TreeNode{Name: "/", Children: make([]*gateway.TreeNode, 0)},
		gatewayConfigMap: make(map[string]gateway.GWConfig),
	}
	engine.pool.New = func() any {
		return engine.allocateContext()
	}
	return engine
}

func Default() *Engine {
	engine := New()
	engine.Logger = msLog.Default()
	engine.Use(Logging, Recovery)
	engine.router.engine = engine
	return engine
}

func (e *Engine) Handler() http.Handler { //返回本身
	return e
}

func (e *Engine) Use(middles ...MiddlewareFunc) {
	e.Middles = append(e.Middles, middles...)
}

func (e *Engine) allocateContext() any {
	return &Context{engine: e}
}

func (e *Engine) SetGatewayConfig(configs []gateway.GWConfig) {
	e.gatewayConfigs = configs
	//把这个路径存储起来，访问的时候，去匹配里面的路由，如果匹配拿出结果
	for _, v := range e.gatewayConfigs {
		e.gatewayTreeNode.Put(v.Path, v.Name)
		e.gatewayConfigMap[v.Name] = v
	}
}

func (e *Engine) SetFuncMap(funcMap template.FuncMap) { //设置模板函数映射
	e.funcMap = funcMap
}

func (e *Engine) LoadTemplate(pattern string) { //加载模板
	t := template.Must(template.New("").Funcs(e.funcMap).ParseGlob(pattern))
	e.SetHtmlTemplate(t)
}

func (e *Engine) SetHtmlTemplate(t *template.Template) {
	e.HTMLRender = render.HTMLRender{
		Template: t,
	}
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := e.pool.Get().(*Context)
	ctx.W = w
	ctx.R = r
	ctx.Logger = e.Logger
	e.httpRequestHandle(ctx, w, r)
	e.pool.Put(ctx)
}

func (e *Engine) httpRequestHandle(ctx *Context, w http.ResponseWriter, r *http.Request) {
	if e.OpenGateway { //已开启路由网关
		//请求过来具体转发到哪
		path := r.URL.Path
		node := e.gatewayTreeNode.Get(path)
		if node == nil {
			ctx.W.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(ctx.W, ctx.R.RequestURI+" not found")
			return
		}
		gwConfig := e.gatewayConfigMap[node.GwName]
		gwConfig.Header(r) //网关设置header
		target, err := url.Parse(fmt.Sprintf("http://%s:%d%s", gwConfig.Host, gwConfig.Port, path))
		if err != nil {
			ctx.W.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(ctx.W, err.Error())
			return
		}
		//网关业务处理
		director := func(req *http.Request) {
			req.Host = target.Host
			req.URL.Host = target.Host
			req.URL.Path = target.Path
			req.URL.Scheme = target.Scheme
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "")
			}
		}
		response := func(response *http.Response) error {
			log.Println("响应修改")
			return nil
		}
		handler := func(writer http.ResponseWriter, request *http.Request, err error) {
			log.Println("错误处理")
		}
		proxy := httputil.ReverseProxy{Director: director, ModifyResponse: response, ErrorHandler: handler}
		proxy.ServeHTTP(w, r)
		return
	}
	method := r.Method
	for _, group := range e.router.groups {
		routerName := SubStringLast(r.URL.Path, "/"+group.name) //r.URL.Path与r.RequestURI区别在于后者会包含query参数（如果有）
		node := group.treeNode.Get(routerName)
		if node != nil && node.isEnd {
			//路由匹配上了
			handle, ok := group.handlerFuncMap[node.routerName][ANY]
			if ok {
				group.methodHandle(node.routerName, ANY, handle, ctx)
				return
			}
			handle, ok = group.handlerFuncMap[node.routerName][method]
			if ok {
				group.methodHandle(node.routerName, method, handle, ctx)
				return
			}
			//路径匹配方法不匹配，显示405
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "%s %s not allowed \n", r.RequestURI, method)
			return
		}
	}
	//当所有路由组中都未匹配到路径返回404
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "%s not found\n", r.RequestURI)
}

func (e *Engine) Run(addr string) {
	http.Handle("/", e)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func (e *Engine) RunTLS(addr, certFile, keyFile string) { //https支持
	err := http.ListenAndServeTLS(addr, certFile, keyFile, e.Handler())
	if err != nil {
		log.Fatal(err)
	}
}
