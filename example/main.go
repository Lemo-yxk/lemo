package main

import (
	"os"
	"time"

	"github.com/Lemo-yxk/lemo"
	"github.com/Lemo-yxk/lemo/logger"
	"github.com/Lemo-yxk/lemo/utils"
)

func main() {

	logger.SetFlag(logger.DEBUG | logger.LOG)

	logger.SetLogHook(func(status string, t time.Time, file string, line int, v ...interface{}) {
		println(status)
	})

	var httpClient = utils.NewHttpClient()

	res, err := httpClient.Post("http://161.117.178.174:12350/Proxy/User/login").Form(lemo.M{"account_name": 571413495, "password": 123456.0111}).Send()
	if err != nil {
		panic(err)
	}

	logger.Console(string(res))

	HttpServer()

	utils.ListenSignal(func(sig os.Signal) {
		logger.Console(sig)
	})

}

func HttpServer() {

	var server = lemo.Server{Host: "127.0.0.1", Port: 8666}

	var httpServer = lemo.HttpServer{}

	httpServer.OnError = func(err func() *lemo.Error) {
		logger.Console(err())
	}

	httpServer.Group("/hello").Handler(func(this *lemo.HttpServer) {
		this.Get("/world").Handler(func(t *lemo.Stream) func() *lemo.Error {
			logger.Console("ha")
			return lemo.NewError(t.End("hello world!"))
		})
	})

	httpServer.SetStaticPath("/dir", "./example/public")

	server.Start(nil, &httpServer)

}
