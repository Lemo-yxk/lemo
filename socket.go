package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// type Message struct {
// 	Fd    uint32
// 	Type  int
// 	Event string
// }

type IM struct {
	Fd    uint32
	Type  int
	Event string
	Msg   interface{}
}

type BM struct {
	Fd    uint32
	Type  int
	Event string
	Msg   []byte
}

type M map[string]interface{}

type WebSocketServerFunction func(conn *Connection, message *BM, context interface{})

// PingMessage PING
const PingMessage int = websocket.PingMessage

// PongMessage PONG
const PongMessage int = websocket.PongMessage

// TextMessage 文本
const TextMessage int = websocket.TextMessage

// BinaryMessage 二进制
const BinaryMessage int = websocket.BinaryMessage

const Json = 1
const ProtoBuf = 2

// Connection Connection
type Connection struct {
	Fd       uint32
	Socket   *websocket.Conn
	Handler  *Socket
	Response http.ResponseWriter
	Request  *http.Request
	push     chan *BM
	back     chan error
}

// Socket conn
type Socket struct {
	Fd          uint32
	Connections map[uint32]*Connection
	OnClose     func(conn *Connection)
	OnMessage   func(conn *Connection, message *BM)
	OnOpen      func(conn *Connection)
	OnError     func(err error)

	HeartBeatTimeout  int
	HeartBeatInterval int
	HandshakeTimeout  int
	ReadBufferSize    int
	WriteBufferSize   int
	WaitQueueSize     int
	CheckOrigin       func(r *http.Request) bool

	Before func() error
	After  func() error

	WebSocketRouter map[string]WebSocketServerFunction

	TsProto int
}

func (conn *Connection) IP() (string, string, error) {

	if ip := conn.Request.Header.Get("X-Real-IP"); ip != "" {
		return net.SplitHostPort(ip)
	}

	if ip := conn.Request.Header.Get("X-Forwarded-For"); ip != "" {
		return net.SplitHostPort(ip)
	}

	return net.SplitHostPort(conn.Request.RemoteAddr)
}

func (conn *Connection) Emit(event string, message *IM) error {
	return conn.Handler.Emit(event, message)
}

func (conn *Connection) EmitAll(event string, message *IM) {
	conn.Handler.EmitAll(event, message)
}

// Push 发送消息
func (socket *Socket) Push(fd uint32, messageType int, message []byte) error {

	if _, ok := socket.Connections[fd]; !ok {
		return fmt.Errorf("client %d is close", fd)
	}

	// 默认为文本
	if messageType == 0 {
		messageType = TextMessage
	}

	socket.Connections[fd].push <- &BM{fd, messageType, "", message}

	return <-socket.Connections[fd].back
}

// Push Json 发送消息
func (socket *Socket) Json(message *IM) error {

	messageJson, err := json.Marshal(message.Msg)
	if err != nil {
		return fmt.Errorf("message error: %v", err)
	}

	return socket.Push(message.Fd, message.Type, messageJson)
}

func (socket *Socket) EmitAll(event string, message *IM) {
	for _, conn := range socket.Connections {
		message.Fd = conn.Fd
		_ = socket.Emit(event, message)
	}
}

func (socket *Socket) Emit(event string, message *IM) error {

	if message.Type == BinaryMessage {
		if j, b := message.Msg.([]byte); b {
			return socket.Push(message.Fd, message.Type, j)
		}

		return fmt.Errorf("message type is bin that message must be []byte")
	}

	switch socket.TsProto {
	case Json:
		return socket.jsonEmit(message.Fd, message.Type, event, message.Msg)
	case ProtoBuf:
		return socket.protoBufEmit(message.Fd, message.Type, event, message.Msg)
	}

	return fmt.Errorf("unknown ts ptoto")

}

func (socket *Socket) protoBufEmit(fd uint32, messageType int, event string, message interface{}) error {
	return nil
}

func (socket *Socket) jsonEmit(fd uint32, messageType int, event string, message interface{}) error {

	var data = M{"event": event, "data": message}

	if j, b := message.([]byte); b {
		data["data"] = string(j)
	}

	return socket.Json(&IM{Fd: fd, Type: messageType, Msg: data})

}

func (socket *Socket) ProtoBuf(fd uint32, messageType int, message M) error {
	return nil
}

func (socket *Socket) addConnect(conn *Connection) {

	// +1
	socket.Fd++

	// 赋值
	conn.Fd = socket.Fd

	// 如果不存在 则存储
	if _, ok := socket.Connections[conn.Fd]; !ok {
		socket.Connections[conn.Fd] = conn
	} else {

		// 否则查找最大值
		var maxFd uint32 = 0

		for fd, _ := range socket.Connections {
			if fd > maxFd {
				maxFd = fd
			}
		}

		// +1
		maxFd++

		// 溢出
		if maxFd == 0 {
			maxFd++
		}

		socket.Connections[maxFd] = conn

	}

	// 触发OPEN事件
	socket.OnOpen(conn)
}
func (socket *Socket) delConnect(conn *Connection) {
	delete(socket.Connections, conn.Fd)
	socket.OnClose(conn)
}

// WebSocket 默认设置
func WebSocket(socket *Socket) http.HandlerFunc {

	if socket.TsProto == 0 {
		socket.TsProto = Json
	}

	if socket.HeartBeatTimeout == 0 {
		socket.HeartBeatTimeout = 30
	}

	if socket.HeartBeatInterval == 0 {
		socket.HeartBeatInterval = 20
	}

	if socket.HandshakeTimeout == 0 {
		socket.HandshakeTimeout = 2
	}

	if socket.ReadBufferSize == 0 {
		socket.ReadBufferSize = 2 * 1024 * 1024
	}

	if socket.WriteBufferSize == 0 {
		socket.WriteBufferSize = 2 * 1024 * 1024
	}

	if socket.WaitQueueSize == 0 {
		socket.WaitQueueSize = 1024
	}

	if socket.CheckOrigin == nil {
		socket.CheckOrigin = func(r *http.Request) bool {
			return true
		}
	}

	if socket.OnOpen == nil {
		socket.OnOpen = func(conn *Connection) {
			log.Println(conn.Fd, "is open at", time.Now())
		}
	}

	if socket.OnClose == nil {
		socket.OnClose = func(conn *Connection) {
			log.Println(conn.Fd, "is close at", time.Now())
		}
	}

	if socket.OnError == nil {
		socket.OnError = func(err error) {
			log.Println(err)
		}
	}

	upgrade := websocket.Upgrader{
		HandshakeTimeout: time.Duration(socket.HandshakeTimeout) * time.Second,
		ReadBufferSize:   socket.ReadBufferSize,
		WriteBufferSize:  socket.WriteBufferSize,
		CheckOrigin:      socket.CheckOrigin,
	}

	socket.Connections = make(map[uint32]*Connection)

	// 连接
	var connOpen = make(chan *Connection, socket.WaitQueueSize)

	// 关闭
	var connClose = make(chan *Connection, socket.WaitQueueSize)

	go func() {
		for {
			select {
			case conn := <-connOpen:
				socket.addConnect(conn)
			case conn := <-connClose:
				socket.delConnect(conn)
			}
		}
	}()

	var handler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {

		// 升级协议
		conn, err := upgrade.Upgrade(w, r, nil)

		// 错误处理
		if err != nil {
			socket.OnError(err)
			return
		}

		// 设置PING处理函数
		conn.SetPingHandler(func(status string) error {
			return conn.SetReadDeadline(time.Now().Add(time.Duration(socket.HeartBeatTimeout) * time.Second))
		})

		connection := Connection{
			Fd:       0,
			Socket:   conn,
			Handler:  socket,
			Response: w,
			Request:  r,
			push:     make(chan *BM, 1024),
			back:     make(chan error, 1024),
		}

		// 打开连接 记录
		connOpen <- &connection

		// 关闭连接 清理
		defer func() {
			_ = conn.Close()
			connClose <- &connection
		}()

		go func() {
			for {
				select {
				case message := <-connection.push:
					connection.back <- socket.Connections[message.Fd].Socket.WriteMessage(message.Type, message.Msg)
				}
			}
		}()

		// 收到消息 处理 单一连接收发不冲突 但是不能并发写入
		for {

			// 重置心跳
			_ = conn.SetReadDeadline(time.Now().Add(time.Duration(socket.HeartBeatTimeout) * time.Second))
			messageType, message, err := conn.ReadMessage()

			// 关闭连接
			if err != nil {
				// log.Println(err)
				break
			}

			go func() {
				// 处理消息
				if socket.Before != nil {
					if err := socket.Before(); err != nil {
						return
					}
				}

				if socket.OnMessage != nil {
					socket.OnMessage(&connection, &BM{Fd: connection.Fd, Type: messageType, Msg: message})
				}

				if socket.WebSocketRouter != nil {
					socket.router(&connection, &BM{Fd: connection.Fd, Type: messageType, Msg: message})
				}

				if socket.After != nil {
					_ = socket.After()
				}
			}()

		}

	}

	return handler
}
