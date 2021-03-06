/**
* @program: kitty
*
* @description:
*
* @author: lemo
*
* @create: 2021-02-12 23:59
**/

package udp

import (
	"encoding/binary"
	"errors"

	"github.com/lemoyxk/kitty/socket"
)

const HeadLen = 16

var OpenMessage = []byte{0x0, 0x0, 0x3, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
var CloseMessage = []byte{0x0, 0x0, 0x4, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
var PingMessage = []byte{0x0, 0x0, 0x9, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
var PongMessage = []byte{0x0, 0x0, 0xa, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}

type Protocol interface {
	Decode(message []byte) (messageType byte, id int64, route []byte, body []byte)
	Encode(messageType byte, id int64, route []byte, body []byte) []byte
	Reader() func(n int, buf []byte, fn func(bytes []byte)) error
}

type DefaultProtocol struct{}

func (p *DefaultProtocol) Decode(message []byte) (messageType byte, id int64, route []byte, body []byte) {
	if !isHeaderInvalid(message) {
		return 0, 0, nil, nil
	}

	if getLen(message) != len(message) {
		return 0, 0, nil, nil
	}

	return message[2],
		int64(binary.BigEndian.Uint64(message[8:HeadLen])),
		message[HeadLen : HeadLen+message[3]], message[HeadLen+message[3]:]
}

func (p *DefaultProtocol) Encode(messageType byte, id int64, route []byte, body []byte) []byte {
	switch messageType {
	case socket.Bin:
		return packBin(id, route, body)
	case socket.Ping:
		return PingMessage
	case socket.Pong:
		return PongMessage
	}
	return nil
}

func (p *DefaultProtocol) Reader() func(n int, buf []byte, fn func(bytes []byte)) error {

	var singleMessageLen = 0

	var message []byte

	return func(n int, buf []byte, fn func(bytes []byte)) error {

		message = append(message, buf[0:n]...)

		// read continue
		if len(message) < HeadLen {
			return nil
		}

		for {

			// jump out and read continue
			if len(message) < HeadLen {
				return nil
			}

			// just begin
			if singleMessageLen == 0 {

				// proto error
				if !isHeaderInvalid(message) {
					message = message[0:0]
					singleMessageLen = 0
					return errors.New("invalid header")
				}

				singleMessageLen = getLen(message)
			}

			// jump out and read continue
			if len(message) < singleMessageLen {
				return nil
			}

			// a complete message
			fn(message[0:singleMessageLen])

			// delete this message
			message = message[singleMessageLen:]

			// reset len
			singleMessageLen = 0
		}

	}
}

func isHeaderInvalid(message []byte) bool {

	if len(message) < HeadLen {
		return false
	}

	// keep
	if message[0] != 0 {
		return false
	}

	// keep
	if message[1] != 0 {
		return false
	}

	// message type
	if message[2] != socket.Bin &&
		message[2] != socket.Ping &&
		message[2] != socket.Pong &&
		message[2] != socket.Open &&
		message[2] != socket.Close {
		return false
	}

	return true
}

func getLen(message []byte) int {
	if len(message) < HeadLen {
		return 0
	}

	var rl = int(message[3])

	var bl = binary.BigEndian.Uint32(message[4:8])

	return rl + int(bl) + HeadLen
}

func packBin(id int64, route []byte, body []byte) []byte {

	var rl = len(route)

	var bl = len(body)

	// data struct
	var data = make([]byte, HeadLen+rl+bl)

	// 0 keep
	data[0] = 0

	// 1 keep
	data[1] = 0

	// 2 message type
	data[2] = socket.Bin

	// 3 route len
	data[3] = byte(rl)

	// 4 - 7 body len
	binary.BigEndian.PutUint32(data[4:8], uint32(bl))

	// 8 - 15 id
	binary.BigEndian.PutUint64(data[8:HeadLen], uint64(id))

	copy(data[HeadLen:HeadLen+rl], route)

	copy(data[HeadLen+rl:HeadLen+rl+bl], body)

	return data
}
