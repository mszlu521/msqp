package protocol

import (
	"bytes"
	"common/logs"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

var (
	routes = make(map[string]uint16) // 路由信息映射为uint16
	codes  = make(map[uint16]string) // uint16映射为路由信息
)

type PackageType byte
type MessageType byte

const (
	None         PackageType = 0x00
	Handshake    PackageType = 0x01 // Handshake represents a handshake: request(client) <====> handshake response(server)
	HandshakeAck PackageType = 0x02 // HandshakeAck represents a handshake ack from client to server
	Heartbeat    PackageType = 0x03 // Heartbeat represents a heartbeat
	Data         PackageType = 0x04 // settings represents a common data packet
	Kick         PackageType = 0x05 // Kick represents a kick off packet
)
const (
	Request  MessageType = 0x00 // ----000-
	Notify   MessageType = 0x01 // ----001-
	Response MessageType = 0x02 // ----010-
	Push     MessageType = 0x03 // ----011-
)

// 掩码定义用来操作flag(1byte)
const (
	RouteCompressMask = 0x01 // 启用路由压缩 00000001
	MsgHeadLength     = 0x02 // 消息头的长度 00000010
	TypeMask          = 0x07 // 获取消息类型 00000111
	GZIPMask          = 0x10 // data compressed gzip mark
	ErrorMask         = 0x20 // 响应错误标识 00100000
)
const (
	HeaderLen     = 4 // 1byte package type 3byte body len
	MaxPacketSize = 1 << 24
)
const (
	msgFlagBytes = 1
)

type Packet struct {
	Type PackageType
	Len  uint32
	Body any
}

func Decode(payload []byte) (*Packet, error) {
	if len(payload) < HeaderLen {
		return nil, errors.New("data len invalid")
	}
	p := &Packet{}
	p.Type = PackageType(payload[0])
	p.Len = uint32(BytesToInt(payload[1:HeaderLen]))
	if p.Type == Handshake {
		var body HandshakeBody
		err := json.Unmarshal(payload[HeaderLen:], &body)
		if err != nil {
			return nil, err
		}
		if body.Sys.Dict != nil {
			SetDictionary(body.Sys.Dict)
		}
		p.Body = body
	}
	if p.Type == Data {
		m, err := MessageDecode(payload[HeaderLen:])
		if err != nil {
			return nil, err
		}
		p.Body = m
	}
	return p, nil
}

func SetDictionary(dict map[string]uint16) {
	if dict == nil {
		return
	}

	for route, code := range dict {
		r := strings.TrimSpace(route) //去掉开头结尾的空格
		// duplication check
		if _, ok := routes[r]; ok {
			logs.Error("duplicated route(route: %s, code: %d)", r, code)
			return
		}

		if _, ok := codes[code]; ok {
			logs.Error("duplicated route(route: %s, code: %d)", r, code)
			return
		}

		// update map, using last value when key duplicated
		routes[r] = code
		codes[code] = r
	}
}
func MessageEncode(m *Message) ([]byte, error) {
	code, compressed := routes[m.Route]
	buf := make([]byte, 0)
	buf = encodeMsgFlag(m.Type, compressed, buf)
	if msgHasId(m.Type) {
		buf = encodeMsgId(m, buf)
	}
	if msgHasRoute(m.Type) {
		buf = encodeMsgRoute(code, compressed, m.Route, buf)
	}
	if m.Data != nil {
		buf = append(buf, m.Data...)
	}
	return buf, nil
}

func encodeMsgRoute(code uint16, compressed bool, route string, buf []byte) []byte {
	if compressed {
		buf = append(buf, byte((code>>8)&0xFF))
		buf = append(buf, byte(code&0xFF))
	} else {
		buf = append(buf, byte(len(route)))
		buf = append(buf, []byte(route)...)
	}
	return buf
}

func encodeMsgId(m *Message, buf []byte) []byte {
	id := m.ID
	for {
		b := byte(id % 128)
		id >>= 7
		if id != 0 {
			buf = append(buf, b+128)
		} else {
			buf = append(buf, b)
			break
		}
	}
	return buf
}

func encodeMsgFlag(t MessageType, compressed bool, buf []byte) []byte {

	flag := byte(t) << 1
	if compressed {
		flag |= RouteCompressMask
	}
	return append(buf, flag)
}

func msgHasRoute(t MessageType) bool {
	return t == Request || t == Notify || t == Push
}

func caculateMsgIdBytes(id uint) int {
	length := 0
	for id > 0 {
		length += 1
		id <<= 7
	}
	return length
}

func msgHasId(messageType MessageType) bool {
	return messageType == Request || messageType == Response
}

//func MessageEncode(m *Message) ([]byte, error) {
//	if m.SessionType < Request || m.SessionType > Push {
//		return nil, errors.New("invalid stream type")
//	}
//	buf := make([]byte, 0)
//	flag := byte(m.SessionType) << 1
//	code, compressed := routes[m.Route]
//	if compressed {
//		flag |= RouteCompressMask
//	}
//	buf = append(buf, flag)
//	if m.SessionType == Request || m.SessionType == Response {
//		n := m.ID
//		// variant length encode
//		for {
//			b := byte(n % 128)
//			n >>= 7
//			if n != 0 {
//				buf = append(buf, b+128)
//			} else {
//				buf = append(buf, b)
//				break
//			}
//		}
//	}
//	if routable(m.SessionType) {
//		if compressed {
//			buf = append(buf, byte((code>>8)&0xFF))
//			buf = append(buf, byte(code&0xFF))
//		} else {
//			buf = append(buf, byte(len(m.Route)))
//			buf = append(buf, []byte(m.Route)...)
//		}
//	}
//
//	buf = append(buf, m.Data...)
//	fmt.Println(buf)
//	return buf, nil
//}

func routable(t MessageType) bool {
	return t == Request || t == Notify || t == Push
}

// MessageDecode https://github.com/NetEase/pomelo/wiki/%E5%8D%8F%E8%AE%AE%E6%A0%BC%E5%BC%8F
// ------------------------------------------
// |   flag   |  stream id  |       route        |
// |----------|--------|--------------------|
// | 1 byte   |0-5bytes|0-256bytes|
// ------------------------------------------
// flag占用message头的第一个byte
// 现在只用到了其中的4个bit，这四个bit包括两部分，占用3个bit的message type字段和占用1个bit的route标识
// stream type用来标识消息类型,范围为0～7，现在消息共有四类，request，notify，response，push，值的范围是0～3
// 最后一位的route表示route是否压缩，影响route字段的长度
// 不同类型的消息，对应不同消息头，消息类型通过flag字段的第2-4位来确定，其对应关系
// ------------------------------------------
// |   type   |  flag  |       other        |
// |----------|--------|--------------------|
// | request  |----000-|<stream id>|<route>|
// | notify   |----001-|<route>             |
// | response |----010-|<stream id>        |
// | push     |----011-|<route>             |
// ------------------------------------------
func MessageDecode(body []byte) (Message, error) {
	m := Message{}
	flag := body[0]
	m.Type = MessageType((flag >> 1) & TypeMask)
	if m.Type < Request || m.Type > Push {
		return m, errors.New("invalid stream type")
	}
	offset := 1
	dataLen := len(body)
	if m.Type == Request || m.Type == Response {
		id := uint(0)
		// little end byte order
		// variant length encode
		for i := offset; i < dataLen; i++ {
			b := body[i]
			id += uint(b&0x7F) << uint(7*(i-offset))
			if b < 128 {
				offset = i + 1
				break
			}
		}
		m.ID = id
	}
	if offset > dataLen {
		return m, errors.New("invalid stream")
	}
	m.Error = flag&ErrorMask == ErrorMask
	if m.Type == Request || m.Type == Notify || m.Type == Push {
		//route解析
		if flag&RouteCompressMask == 1 {
			m.routeCompressed = true
			code := binary.BigEndian.Uint16(body[offset:(offset + 2)])
			route, found := GetRoute(code)
			if !found {
				return m, errors.New("route info not found in dictionary")
			}
			m.Route = route
			offset += 2

		} else {
			m.routeCompressed = false
			rl := body[offset]
			offset++
			m.Route = string(body[offset:(offset + int(rl))])
			offset += int(rl)
		}
	}
	if offset > dataLen {
		return m, errors.New("invalid stream")
	}
	m.Data = body[offset:]
	var err error
	if flag&GZIPMask == GZIPMask {
		m.Data, err = InflateData(m.Data)
		if err != nil {
			return m, err
		}
	}
	return m, nil
}

func GetRoute(code uint16) (route string, found bool) {
	route, found = codes[code]
	return route, found
}

func InflateData(data []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func Encode(packageType PackageType, body []byte) ([]byte, error) {
	if packageType == None {
		return nil, errors.New("encode unsupported packageType")
	}
	if len(body) > MaxPacketSize {
		return nil, errors.New("encode body size too big")
	}
	buf := make([]byte, len(body)+HeaderLen)
	//1. 类型
	buf[0] = byte(packageType)
	//2. 长度
	copy(buf[1:HeaderLen], IntToBytes(len(body)))
	//3.body
	copy(buf[HeaderLen:], body)
	return buf, nil
}
func (p *Packet) HandshakeBody() *HandshakeBody {
	if p.Type == Handshake {
		body := p.Body.(HandshakeBody)
		return &body
	}
	return nil
}
func (p *Packet) MessageBody() *Message {
	if p.Type == Data {
		body := p.Body.(Message)
		return &body
	}
	return nil
}

type HandshakeBody struct {
	Sys Sys `json:"sys"`
}

type Sys struct {
	Type         string            `json:"type"`
	Version      string            `json:"version"`
	ProtoVersion uint8             `json:"protoVersion"`
	Heartbeat    uint8             `json:"heartbeat"`
	Dict         map[string]uint16 `json:"dict"`
	Serializer   string            `json:"serializer"`
}

type HandshakeResponse struct {
	Code uint16 `json:"code"`
	Sys  Sys    `json:"sys"`
}

type Message struct {
	Type            MessageType // stream type 4中消息类型 request response notify push
	ID              uint        // unique id, zero while notify mode 消息id（request response）
	Route           string      // route for locating service 消息路由
	Data            []byte      // payload  消息体的原始数据
	routeCompressed bool        // is route Compressed 是否启用路由压缩
	Error           bool        // response error
}
