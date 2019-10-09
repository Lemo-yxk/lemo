package main

import (
	"log"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/protobuf/proto"

	"github.com/Lemo-yxk/lemo"
	"github.com/Lemo-yxk/lemo/logger"
	awesomepackage "github.com/Lemo-yxk/lemo/protobuf-origin"
)

func init() {
	log.SetFlags(log.Llongfile | log.Ldate | log.Ltime)
}

func main() {

	// lemo.Socket()

	go Server()

	time.Sleep(time.Second)

	go Client()

	// 创建信号
	signalChan := make(chan os.Signal, 1)
	// 通知
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	// 阻塞
	<-signalChan

}

func Server() {
	var server = &lemo.Server{Host: "0.0.0.0", Port: 12345}

	var socketHandler = &lemo.WebSocketServer{Path: "/"}

	// var socketBefore = []lemo.WebSocketBefore{
	// 	func(conn *lemo.Connection, msg *lemo.MessagePackage) (lemo.Context, func() *lemo.Error) {
	// 		return "hello111111111", nil
	// 	},
	// }

	socketHandler.SetRouter("/:hello", func(conn *lemo.Connection, receive *lemo.Receive) func() *lemo.Error {

		var awesome = &awesomepackage.AwesomeMessage{}
		err := proto.Unmarshal(receive.Message.Message, awesome)

		if err != nil {
			return lemo.NewError(err)
		}

		logger.Log(receive.Message.Event, receive.Message.MessageType, receive.Message.ProtoType == lemo.ProtoBuf, awesome.AwesomeField, awesome.AwesomeKey)
		// _ = conn.JsonEmit(conn.Fd, lemo.JsonPackage{Event: "/haha", Message: "roland 这个傻吊"})
		_ = conn.Json(conn.Fd, lemo.M{"key": "roland", "type": "people"})
		return nil
	})

	socketHandler.OnMessage = func(conn *lemo.Connection, messageType int, msg []byte) {
		if len(msg) == 0 {
			return
		}

		// var awesome = &awesomepackage.AwesomeMessage{}
		// err := proto.Unmarshal(msg[9:], awesome)
		//
		// if err != nil {
		// 	logger.Log("marshaling error: ", err)
		// 	return
		// }

		logger.Log(msg)
	}

	socketHandler.OnClose = func(fd uint32) {
		logger.Log(fd, "is close")
	}

	socketHandler.OnError = func(err func() *lemo.Error) {
		logger.Log(err())
	}

	socketHandler.OnOpen = func(conn *lemo.Connection) {
		logger.Log(conn.Fd, "is open")
	}

	var httpHandler = &lemo.Http{}

	var before = []lemo.HttpBefore{
		func(t *lemo.Stream) (lemo.Context, func() *lemo.Error) {
			_ = t.End("before")
			return nil, nil
		},
	}

	var after = []lemo.HttpAfter{
		func(t *lemo.Stream) func() *lemo.Error {
			_ = t.End("after")
			return nil
		},
	}

	httpHandler.Get("/debug/pprof/", pprof.Index)
	httpHandler.Get("/debug/pprof/:tip", pprof.Index)
	httpHandler.Get("/debug/pprof/cmdline", pprof.Cmdline)
	httpHandler.Get("/debug/pprof/profile", pprof.Profile)
	httpHandler.Get("/debug/pprof/symbol", pprof.Symbol)
	httpHandler.Get("/debug/pprof/trace", pprof.Trace)

	httpHandler.Group("/:hello", func() {
		httpHandler.Get("/:12", before, after, func(t *lemo.Stream) func() *lemo.Error {

			var params = t.Params.ByName("hello")

			err := t.Json(lemo.M{"hello": params})

			return lemo.NewError(err)
		})
	})

	httpHandler.OnError = func(ef func() *lemo.Error) {
		var err = ef()
		if err == nil {
			return
		}
		logger.Log(err)
	}

	server.Start(socketHandler, httpHandler)
}

func Client() {
	var client = &lemo.WebSocketClient{Host: "127.0.0.1", Port: 12345, Path: "/", HandshakeTimeout: 10, AutoHeartBeat: true}

	client.SetRouter("/haha", func(c *lemo.WebSocketClient, receive *lemo.Receive) func() *lemo.Error {

		logger.Log(receive.Message.Event, receive.Message.MessageType, receive.Message.ProtoType == lemo.Json, string(receive.Message.Message))

		return nil
	})

	client.OnOpen = func(c *lemo.WebSocketClient) {
		logger.Log("open")

		var data = &awesomepackage.AwesomeMessage{AwesomeField: "尼玛的", AwesomeKey: "他妈的"}

		logger.Log(c.ProtoBufEmit(lemo.ProtoBufPackage{Event: "/xixi", Message: data}))

	}

	client.OnMessage = func(c *lemo.WebSocketClient, messageType int, msg []byte) {
		logger.Log(string(msg))
	}

	client.OnError = func(err func() *lemo.Error) {
		logger.Err(err)
	}

	client.OnClose = func(c *lemo.WebSocketClient) {
		logger.Log("close")
	}

	client.Connect()
}
