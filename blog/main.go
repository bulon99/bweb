package main

import (
	"fmt"
	"github.com/bulon99/msgo"
	msLog "github.com/bulon99/msgo/log"
	"github.com/bulon99/msgo/mspool"
	"github.com/bulon99/msgo/token"
	"log"
	"net/http"
	"sync"
	"time"
)

func M(next msgo.HandlerFunc) msgo.HandlerFunc {
	return func(ctx *msgo.Context) {
		fmt.Println("----------")
		next(ctx)
		fmt.Println("++++++++++")
	}
}

type User struct {
	Name      string   `xml:"name" json:"name"`
	Age       int      `xml:"age" json:"age" validate:"required,max=50,min=18"` //引入validate验证，对xml参数和json参数都有效，缺陷在于如果传字段的零值，则认为是未传入的
	Addresses []string `json:"addresses"`
	Email     string   `json:"email" msgo:"required"` //msgo:"required"表示该字段必须传
}

func main() {
	//http.HandleFunc("/hello", func(writer http.ResponseWriter, request *http.Request) {
	//	fmt.Fprintln(writer, "hello bulon.com")
	//})
	//err := http.ListenAndServe(":8111", nil)
	//if err != nil {
	//	log.Fatal(err)
	//}
	engine := msgo.Default()

	//auth := &msgo.Accounts{
	//	Users: make(map[string]string),
	//}
	//auth.Users["bulon"] = "0213" //Authorization    Basic YnVsb246MDIxMw==
	//engine.Use(auth.BasicAuth) //使用basic认证中间件

	//jh := &token.JwtHandler{Key: []byte("0213")}
	//engine.Use(jh.AuthInterceptor) //jwt验证中间件

	g := engine.Group("user")
	//g.Use(middle)  //对路由组添加中间件
	//msgo.DefaultLoggerConfig.IsDisplayColor = false //禁用彩色请求日志
	//engine.Logger.Formatter = &msLog.JsonFormatter{TimeDisplay: true} //使用json日志
	//engine.Logger.Level = msLog.LevelInfo                             //设置日志级别，默认是LevelDebug
	//engine.Logger.SetLogPath("./logfile") //将日志按等级输出到不同文件

	g.Get("/hello", func(ctx *msgo.Context) {
		fmt.Fprintln(ctx.W, "get bulon.com/user/hello")
	}, M) //对该请求添加单独的中间件
	g.Post("/hello", func(ctx *msgo.Context) {
		ctx.Logger.Debug("debug日志")
		ctx.Logger.WithFields(msLog.Fields{
			"name": "bulon",
			"id":   1000,
		}).Debug("debug日志")
		ctx.Logger.Info("info日志")
		ctx.Logger.Error("error日志")
		fmt.Fprintln(ctx.W, "post bulon.com/user/hello")
	})

	var zero = 0
	g.Any("/error", func(ctx *msgo.Context) {
		fmt.Println(2 / zero) //测试错误
		fmt.Fprintln(ctx.W, "not return")
	})
	g.Any("/any", func(ctx *msgo.Context) {
		fmt.Fprintln(ctx.W, "any bulon.com/user/any")
	})
	g.Get("/get/:id", func(ctx *msgo.Context) {
		fmt.Fprintln(ctx.W, ctx.R.RequestURI, "catched by /get/:id")
	})
	g.Get("/hello/*/get", func(ctx *msgo.Context) {
		fmt.Fprintln(ctx.W, ctx.R.RequestURI, "catched by /hello/*/get")
	})

	//html, 模板加载
	g.Get("/html", func(ctx *msgo.Context) {
		ctx.HTML(http.StatusOK, "<h1>golang之旅</h1>")
	})
	g.Get("/html_template", func(ctx *msgo.Context) {
		user := &User{"bulong", 23, nil, ""}
		err := ctx.HTMLTemplate("login.html", user, "tpl/login.html", "tpl/header.html")
		if err != nil {
			log.Println(err)
		}
	})
	g.Get("/html_template_glob", func(ctx *msgo.Context) {
		user := &User{"bulong", 23, nil, ""}
		err := ctx.HTMLTemplateGlob("login.html", user, "tpl/*.html")
		if err != nil {
			log.Println(err)
		}
	})
	g.Get("/template", func(ctx *msgo.Context) {
		user := &User{"bulong", 23, nil, ""}
		err := ctx.Template("login.html", user)
		if err != nil {
			log.Println(err)
		}
	})

	//返回json数据
	g.Get("/info", func(ctx *msgo.Context) {
		_ = ctx.JSON(http.StatusOK, &User{
			Name: "go微服务框架",
			Age:  23,
		})
	})

	//返回xml数据
	g.Get("/xml", func(ctx *msgo.Context) {
		user := &User{
			Name: "go微服务框架",
		}
		_ = ctx.XML(http.StatusOK, user)
	})

	//重定向
	g.Get("/redirect", func(ctx *msgo.Context) {
		ctx.Redirect(http.StatusFound, "/user/hello")
	})

	//返回text
	g.Get("/text", func(ctx *msgo.Context) {
		ctx.String(http.StatusOK, "%s 是由 %s 制作 \n", "goweb框架", "go微服务框架")
	})

	//下载文件
	g.Get("/excel", func(ctx *msgo.Context) { //下载后的文件是excel.xlsx
		ctx.File("file/work.xlsx")
	})

	//下载文件，自定义名称
	g.Get("/excelName", func(ctx *msgo.Context) {
		ctx.FileAttachment("file/work.xlsx", "aa.xlsx") //下载后的文件是aa.xlsx
	})

	//文件系统下载
	g.Get("/fs", func(ctx *msgo.Context) {
		ctx.FileFromFS("work.xlsx", http.Dir("file"))
	})

	//query参数
	g.Get("/add", func(ctx *msgo.Context) {
		id := ctx.GetQuery("id")
		ids, _ := ctx.GetQueryArray("id") //?id=123&id=567
		fmt.Println("id:", id, ids)
	})

	//postform参数
	g.Post("/form", func(ctx *msgo.Context) {
		name, _ := ctx.GetPostForm("name")
		names, _ := ctx.GetPostFormArray("name")
		fmt.Println(name, names)
	})

	//postfile
	g.Post("/file", func(ctx *msgo.Context) {
		name, _ := ctx.GetPostForm("name")
		fmt.Println(name)
		//上传一个文件（一个key对应一个文件）
		file, err := ctx.FormFile("file") //解析文件对应的key
		if err != nil {
			log.Println(err)
		}
		err = ctx.SaveUploadedFile(file, "upload/"+file.Filename)
		if err != nil {
			log.Println(err)
		}
		//上传多个文件(一个key对应多个文件)
		fileHeaders, err := ctx.FormFiles("file0")
		if err != nil {
			log.Println(err)
		}
		for _, fileHeader := range fileHeaders {
			err = ctx.SaveUploadedFile(fileHeader, "upload/"+fileHeader.Filename)
			if err != nil {
				log.Println(err)
			}
		}
	})

	//json参数
	g.Post("/jsonParam", func(ctx *msgo.Context) {
		//user := User{}
		user := make([]User, 0)
		//ctx.DisallowUnknownFields = true //不允许参数中传入了结构体中没有的字段
		//ctx.IsValidate = true         //不允许参数中缺少了结构体中必需的字段，这两个设置只能开启其中一个，因为decoder.Decoder只能调用一次
		err := ctx.BindJson(&user)
		if err == nil {
			ctx.JSON(http.StatusOK, user)
		} else {
			log.Println(err)
		}
	})

	//xml参数
	g.Post("/xmlParam", func(ctx *msgo.Context) {
		user := &User{}
		err := ctx.BindXML(user)
		if err == nil {
			ctx.JSON(http.StatusOK, user)
		} else {
			log.Println(err)
		}
	})

	//协程池
	p, _ := mspool.NewPool(5)
	g.Post("/pool", func(ctx *msgo.Context) {
		currentTime := time.Now().UnixMilli()
		var wg sync.WaitGroup
		wg.Add(5)
		p.Submit(func() {
			fmt.Println(1111)
			time.Sleep(3 * time.Second)
			wg.Done()
			panic("协程池中的异常") //测试协程池中的异常捕获
		})
		p.Submit(func() {
			fmt.Println(2222)
			time.Sleep(3 * time.Second)
			wg.Done()
		})
		p.Submit(func() {
			fmt.Println(3333)
			time.Sleep(3 * time.Second)
			wg.Done()
		})
		p.Submit(func() {
			fmt.Println(4444)
			time.Sleep(3 * time.Second)
			wg.Done()
		})
		p.Submit(func() {
			fmt.Println(5555)
			time.Sleep(3 * time.Second)
			wg.Done()
		})
		wg.Wait()
		fmt.Printf("耗时 %vms\n", time.Now().UnixMilli()-currentTime)
		ctx.JSON(http.StatusOK, "success")
	})

	//JWT 登录验证过后返回token
	g.Get("/login", func(ctx *msgo.Context) {

		jwt := &token.JwtHandler{}
		jwt.Key = []byte("0213")
		jwt.SendCookie = true
		jwt.TimeOut = 10 * time.Minute
		jwt.RefreshTimeOut = 20 * time.Minute
		jwt.Authenticator = func(ctx *msgo.Context) (map[string]any, error) {
			data := make(map[string]any)
			data["userId"] = 1
			return data, nil
		}
		token, err := jwt.LoginHandler(ctx)
		if err != nil {
			log.Println(err)
			ctx.JSON(http.StatusOK, err.Error())
			return
		}
		ctx.JSON(http.StatusOK, token)
	})

	//在token未过期时刷新token
	g.Get("/refresh", func(ctx *msgo.Context) {
		jwt := &token.JwtHandler{}
		jwt.Key = []byte("0213")
		jwt.SendCookie = true
		jwt.TimeOut = 10 * time.Minute
		jwt.RefreshTimeOut = 20 * time.Minute
		jwt.RefreshKey = "refresh_token"
		ctx.Set(jwt.RefreshKey, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2NjIxOTU4MzcsImlhdCI6MTY2MjE5NDYzNywidXNlcklkIjoxfQ.g9fcc8AciiTlowclZAacd1yKi0lXeAiCkRlk8K173OU")
		token, err := jwt.RefreshHandler(ctx)
		if err != nil {
			log.Println(err)
			ctx.JSON(http.StatusOK, err.Error())
			return
		}
		ctx.JSON(http.StatusOK, token)
	})

	//预加载模板
	//engine.SetFuncMap()
	engine.LoadTemplate("tpl/*html")
	//启动服务
	//engine.Run("localhost:8111")
	engine.RunTLS("localhost:8118", "key/server.pem", "key/server.key") //https://127.0.0.1:8118/user/hello 必须通过https访问
}
