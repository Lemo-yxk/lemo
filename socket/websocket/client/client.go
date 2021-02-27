package client

import (
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/json-iterator/go"

	"github.com/lemoyxk/kitty"
	"github.com/lemoyxk/kitty/socket"

	websocket2 "github.com/lemoyxk/kitty/socket/websocket"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
)

type Client struct {
	Name   string
	Scheme string
	Host   string
	Path   string

	Conn              *websocket.Conn
	Response          *http.Response
	AutoHeartBeat     bool
	HeartBeatTimeout  time.Duration
	HeartBeatInterval time.Duration
	HeartBeat         func(c *Client) error
	Reconnect         bool
	ReconnectInterval time.Duration
	WriteBufferSize   int
	ReadBufferSize    int
	HandshakeTimeout  time.Duration

	OnOpen    func(client *Client)
	OnClose   func(client *Client)
	OnMessage func(client *Client, messageType int, msg []byte)
	OnError   func(err error)
	OnSuccess func()
	OnUnknown func(conn *Client, message []byte, next Middle)

	PingHandler func(client *Client) func(appData string) error
	PongHandler func(client *Client) func(appData string) error

	Protocol websocket2.Protocol

	mux    sync.RWMutex
	router *Router
	middle []func(Middle) Middle
}

type Middle func(client *Client, stream *socket.Stream)

func (c *Client) LocalAddr() net.Addr {
	return c.Conn.LocalAddr()
}

func (c *Client) RemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

func (c *Client) Use(middle ...func(Middle) Middle) {
	c.middle = append(c.middle, middle...)
}

func (c *Client) Emit(pack socket.Pack) error {
	return c.Push(c.Protocol.Encode(socket.BinData, pack.ID, []byte(pack.Event), pack.Data))
}

func (c *Client) JsonEmit(pack socket.JsonPack) error {
	data, err := jsoniter.Marshal(pack.Data)
	if err != nil {
		return err
	}
	return c.Push(c.Protocol.Encode(socket.BinData, pack.ID, []byte(pack.Event), data))
}

func (c *Client) ProtoBufEmit(pack socket.ProtoBufPack) error {
	data, err := proto.Marshal(pack.Data)
	if err != nil {
		return err
	}
	return c.Push(c.Protocol.Encode(socket.BinData, pack.ID, []byte(pack.Event), data))
}

func (c *Client) Push(message []byte) error {
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.Conn.WriteMessage(int(socket.BinData), message)
}

func (c *Client) Close() error {
	return c.Conn.Close()
}

func (c *Client) reconnecting() {
	if c.Reconnect == true {
		time.Sleep(c.ReconnectInterval)
		c.Connect()
	}
}

// Connect 连接服务器
func (c *Client) Connect() {

	if c.Path == "" {
		c.Path = "/"
	}

	if c.Host == "" {
		panic("Host must set")
	}

	if c.OnOpen == nil {
		panic("OnOpen must set")
	}

	if c.OnClose == nil {
		panic("OnClose must set")
	}

	if c.OnError == nil {
		panic("OnError must set")
	}

	// 握手
	if c.HandshakeTimeout == 0 {
		c.HandshakeTimeout = 2 * time.Second
	}

	// 写入BUF大小
	if c.WriteBufferSize == 0 {
		c.WriteBufferSize = 1024
	}

	// 读出BUF大小
	if c.ReadBufferSize == 0 {
		c.ReadBufferSize = 1024
	}

	// 定时心跳间隔
	if c.HeartBeatInterval == 0 {
		c.HeartBeatInterval = 15 * time.Second
	}

	if c.HeartBeatTimeout == 0 {
		c.HeartBeatTimeout = 30 * time.Second
	}

	// 自动重连间隔
	if c.ReconnectInterval == 0 {
		c.ReconnectInterval = 1 * time.Second
	}

	if c.Protocol == nil {
		c.Protocol = &websocket2.DefaultProtocol{}
	}

	// heartbeat function
	if c.HeartBeat == nil {
		c.HeartBeat = func(client *Client) error {
			return client.Push(client.Protocol.Encode(socket.PingData, 0, nil, nil))
		}
	}

	if c.PingHandler == nil {
		c.PingHandler = func(connection *Client) func(appData string) error {
			return func(appData string) error {
				return nil
			}
		}
	}

	if c.PongHandler == nil {
		c.PongHandler = func(connection *Client) func(appData string) error {
			return func(appData string) error {
				return nil
			}
		}
	}

	var dialer = websocket.Dialer{
		HandshakeTimeout: c.HandshakeTimeout,
		WriteBufferSize:  c.WriteBufferSize,
		ReadBufferSize:   c.ReadBufferSize,
	}

	// 连接服务器
	handler, response, err := dialer.Dial(c.Scheme+"://"+c.Host+c.Path, nil)
	if err != nil {
		c.OnError(err)
		c.reconnecting()
		return
	}

	// 设置PING处理函数
	handler.SetPingHandler(c.PingHandler(c))

	// 设置PONG处理函数
	handler.SetPongHandler(c.PongHandler(c))

	c.Response = response

	c.Conn = handler

	// start success
	if c.OnSuccess != nil {
		c.OnSuccess()
	}

	// 连接成功
	c.OnOpen(c)

	// 定时器 心跳
	ticker := time.NewTicker(c.HeartBeatInterval)

	// 如果有心跳设置
	if c.AutoHeartBeat != true {
		ticker.Stop()
	}

	go func() {
		for range ticker.C {
			if err := c.HeartBeat(c); err != nil {
				c.OnError(err)
				_ = c.Close()
				break
			}
		}
	}()

	for {
		messageFrame, message, err := c.Conn.ReadMessage()
		// close error
		if err != nil {
			break
		}

		err = c.decodeMessage(messageFrame, message)

		if err != nil {
			c.OnError(err)
			break
		}
	}

	// 关闭定时器
	ticker.Stop()
	// 关闭连接
	_ = c.Close()
	// 触发回调
	c.OnClose(c)
	// 触发重连设置
	c.reconnecting()
}

func (c *Client) decodeMessage(messageFrame int, message []byte) error {
	// unpack
	messageType, id, route, body := c.Protocol.Decode(message)

	if c.OnMessage != nil {
		c.OnMessage(c, messageFrame, message)
	}

	if messageType == socket.Unknown {
		if c.OnUnknown != nil {
			c.OnUnknown(c, message, c.middleware)
		}
		return nil
	}

	// Ping
	if messageType == socket.PingData {
		return c.PingHandler(c)("")
	}

	// Pong
	if messageType == socket.PongData {
		return c.PongHandler(c)("")
	}

	// on router
	c.middleware(c, &socket.Stream{Pack: socket.Pack{Event: string(route), Data: body, ID: id}})

	return nil
}

func (c *Client) middleware(conn *Client, stream *socket.Stream) {
	var next Middle = c.handler
	for i := len(c.middle) - 1; i >= 0; i-- {
		next = c.middle[i](next)
	}
	next(conn, stream)
}

func (c *Client) handler(conn *Client, stream *socket.Stream) {

	if c.router == nil {
		if c.OnError != nil {
			c.OnError(errors.New(stream.Event + " " + "404 not found"))
		}
		return
	}

	var n, formatPath = c.router.getRoute(stream.Event)
	if n == nil {
		if c.OnError != nil {
			c.OnError(errors.New(stream.Event + " " + "404 not found"))
		}
		return
	}

	var nodeData = n.Data.(*node)

	stream.Params = kitty.Params{Keys: n.Keys, Values: n.ParseParams(formatPath)}

	for i := 0; i < len(nodeData.Before); i++ {
		if err := nodeData.Before[i](conn, stream); err != nil {
			if c.OnError != nil {
				c.OnError(err)
			}
			return
		}
	}

	err := nodeData.Function(conn, stream)
	if err != nil {
		if c.OnError != nil {
			c.OnError(err)
		}
		return
	}

	for i := 0; i < len(nodeData.After); i++ {
		if err := nodeData.After[i](conn, stream); err != nil {
			if c.OnError != nil {
				c.OnError(err)
			}
			return
		}
	}

}

func (c *Client) SetRouter(router *Router) *Client {
	c.router = router
	return c
}

func (c *Client) GetRouter() *Router {
	return c.router
}
