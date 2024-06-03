package stream

import "framework/protocol"

type Msg struct {
	Cid         string
	Body        *protocol.Message
	Src         string
	Dst         string
	Router      string
	Uid         string
	ConnectorId string
	SessionData *SessionData
	SessionType SessionType // 0 normal 1 session
	PushUser    []string
}
type DataType int

const (
	Single DataType = iota
	All
)

type SessionData struct {
	SingleData map[string]any //只保存当前cid
	AllData    map[string]any //所有cid 都需要保存
}
type SessionType int

const (
	Normal SessionType = iota
	Session
)
