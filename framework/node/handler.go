package node

import "framework/remote"

type HandlerFunc func(session *remote.Session, msg []byte) any
type LogicHandler map[string]HandlerFunc
