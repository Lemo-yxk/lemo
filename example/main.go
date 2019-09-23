package main

import (
	"log"

	"github.com/Lemo-yxk/ws"
)

func init() {
	log.SetFlags(log.Llongfile | log.Ldate | log.Ltime)
}

func main() {

	var server = &ws.Server{Host: "127.0.0.1", Port: 12345, Path: "/Game-Robot"}

	var socketHandler = &ws.Socket{}

	socketHandler.SetRouter("hello1", func(conn *ws.Connection, ftd *ws.Fte, msg []byte) {
		log.Println(ftd.Fd)
	})

	socketHandler.OnClose = func(conn *ws.Connection) {
		log.Println(conn.Fd, "is close")
	}

	socketHandler.OnError = func(err error) {
		log.Println(err)
	}

	socketHandler.OnOpen = func(conn *ws.Connection) {
		log.Println(conn.Fd, "is open")
	}

	var httpHandler = &ws.Http{}

	var before = []ws.Before{
		func(t *ws.Stream) (i interface{}, e error) {
			_ = t.End("before")
			return nil, nil
		},
	}

	var after = []ws.After{
		func(t *ws.Stream) (e error) {
			_ = t.End("after")
			return nil
		},
	}

	httpHandler.Group("/hello", func() {
		httpHandler.Post("/", before, after, func(t *ws.Stream) {
			_ = t.End("hello")
		})
	})

	server.Start(socketHandler, httpHandler)

}
