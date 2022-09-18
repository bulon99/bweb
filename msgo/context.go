package msgo

import (
	"errors"
	"github.com/bulon99/msgo/binding"
	msLog "github.com/bulon99/msgo/log"
	"github.com/bulon99/msgo/render"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"sync"
	"unicode"
)

const defaultMaxMemory int64 = 32 << 20

type Context struct { //封装上下文
	W                     http.ResponseWriter
	R                     *http.Request
	engine                *Engine
	queryCache            url.Values
	formCache             url.Values
	DisallowUnknownFields bool
	IsValidate            bool
	StatusCode            int
	Logger                *msLog.Logger
	Keys                  map[string]any //用于在上下文之间传值
	mu                    sync.RWMutex
	sameSite              http.SameSite
}

func (c *Context) GetHeader(key string) string {
	return c.R.Header.Get(key)
}

func (c *Context) GetCookie(name string) string {
	cookie, err := c.R.Cookie(name)
	if err != nil {
		return ""
	}
	if cookie != nil {
		return cookie.Value
	}
	return ""
}

func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}
	http.SetCookie(c.W, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		SameSite: c.sameSite,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	if c.Keys == nil {
		c.Keys = make(map[string]any)
	}

	c.Keys[key] = value
	c.mu.Unlock()
}

func (c *Context) Get(key string) (value any, exists bool) {
	c.mu.RLock()
	value, exists = c.Keys[key]
	c.mu.RUnlock()
	return
}

func (c *Context) SetBasicAuth(username, password string) {
	c.R.Header.Set("Authorization", "Basic "+BasicAuth(username, password))
}

//query相关
func (c *Context) DefaultQuery(key, defaultValue string) string { //获取不到参数返回默认值
	array, ok := c.GetQueryArray(key)
	if !ok {
		return defaultValue
	}
	return array[0]
}

func (c *Context) GetQuery(key string) string { //获取参数第一个值
	c.initQueryCache()
	return c.queryCache.Get(key)
}

func (c *Context) GetQueryArray(key string) (values []string, ok bool) { //获取参数多个值
	c.initQueryCache()
	values, ok = c.queryCache[key]
	return
}

func (c *Context) initQueryCache() {
	if c.R != nil {
		c.queryCache = c.R.URL.Query()
	} else {
		c.queryCache = url.Values{}
	}

}

//postform相关
func (c *Context) GetPostForm(key string) (string, bool) {
	if values, ok := c.GetPostFormArray(key); ok {
		return values[0], ok
	}
	return "", false
}

func (c *Context) GetPostFormArray(key string) (values []string, ok bool) {
	c.initFormCache()
	values, ok = c.formCache[key]
	return
}

func (c *Context) initFormCache() {
	c.formCache = make(url.Values)
	req := c.R
	if err := req.ParseMultipartForm(defaultMaxMemory); err != nil {
		if !errors.Is(err, http.ErrNotMultipart) { //http.ErrNotMultipart表示传表单时未传文件，忽略
			log.Println(err)
		}
	}
	c.formCache = c.R.PostForm
}

//postfile相关
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	req := c.R
	if err := req.ParseMultipartForm(defaultMaxMemory); err != nil {
		return nil, err
	}
	file, header, err := req.FormFile(name) //FormFile返回key对应的第一个文件*multipart.FileHeader
	if err != nil {
		return nil, err
	}
	err = file.Close()
	if err != nil {
		return nil, err
	}
	return header, nil
}

func (c *Context) FormFiles(name string) ([]*multipart.FileHeader, error) {
	multipart, err := c.MultipartForm()
	if err != nil {
		return nil, err
	}
	return multipart.File[name], nil
}

func (c *Context) SaveUploadedFile(file *multipart.FileHeader, dst string) error { //将post的文件保存到某个路径下
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.R.ParseMultipartForm(defaultMaxMemory)
	return c.R.MultipartForm, err
}

//html相关
func (c *Context) HTML(status int, html string) error {
	c.W.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.W.WriteHeader(status)
	_, err := c.W.Write([]byte(html))
	return err
}

func (c *Context) HTMLTemplate(name string, data any, filename ...string) error {
	c.W.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.New(name)
	t, err := t.ParseFiles(filename...)
	if err != nil {
		return err
	}
	err = t.Execute(c.W, data)
	return err
}

func (c *Context) HTMLTemplateGlob(name string, data any, pattern string) error {
	c.W.Header().Set("Content-Type", "text/html; charset=utf-8")
	t := template.New(name)
	t, err := t.ParseGlob(pattern)
	if err != nil {
		return err
	}
	err = t.Execute(c.W, data)
	return err
}

//HTMLTemplate  HTMLTemplateGlob 每次请求都要加载模板，可以提前将模板加载到内存
//使用预加载的模板
func (c *Context) Template(name string, data any) error {
	c.W.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := c.engine.HTMLRender.Template.ExecuteTemplate(c.W, name, data)
	return err
}

func (c *Context) Render(statusCode int, r render.Render) error {
	err := r.Render(c.W)
	c.StatusCode = statusCode
	//c.W.WriteHeader(statusCode) 去掉不然会导致后面w.Header().Set()失效
	return err
}

//json
func (c *Context) JSON(status int, data any) error {
	return c.Render(status, render.JSON{Data: data})
}

//xml
func (c *Context) XML(status int, data any) error {
	return c.Render(status, render.XML{Data: data})
}

//重定向
func (c *Context) Redirect(status int, location string) {
	c.Render(status, render.Redirect{
		Code:     status,
		Request:  c.R,
		Location: location,
	})
}

//text
func (c *Context) String(status int, format string, values ...any) (err error) {
	err = c.Render(status, render.String{
		Format: format,
		Data:   values,
	})
	return
}

//文件下载
func (c *Context) File(fileName string) {
	http.ServeFile(c.W, c.R, fileName)
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

//文件下载，自定义名称
func (c *Context) FileAttachment(filepath, filename string) {
	if isASCII(filename) {
		c.W.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	} else {
		c.W.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.QueryEscape(filename))
	}
	http.ServeFile(c.W, c.R, filepath)
}

//filepath是相对文件系统的路径
func (c *Context) FileFromFS(filepath string, fs http.FileSystem) {
	defer func(old string) {
		c.R.URL.Path = old
	}(c.R.URL.Path)

	c.R.URL.Path = filepath

	http.FileServer(fs).ServeHTTP(c.W, c.R)
}

//处理json参数
func (c *Context) BindJson(obj any) error {
	jsonBinding := binding.JSON
	jsonBinding.DisallowUnknownFields = c.DisallowUnknownFields
	jsonBinding.IsValidate = c.IsValidate
	return c.MustBindWith(obj, jsonBinding)
}

//处理xml参数
func (c *Context) BindXML(obj any) error {
	return c.MustBindWith(obj, binding.XML)
}

func (c *Context) MustBindWith(obj any, b binding.Binding) error {
	//如果发生错误，返回400状态码 参数错误
	if err := c.ShouldBindWith(obj, b); err != nil {
		c.W.WriteHeader(http.StatusBadRequest)
		return err
	}
	return nil
}

func (c *Context) ShouldBindWith(obj any, b binding.Binding) error {
	return b.Bind(c.R, obj)
}

func (c *Context) Fail(code int, msg string) {
	c.String(code, msg) //使用text格式返回错误数据
}
