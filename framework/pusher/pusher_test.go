package pusher

import (
	"framework/protocol"
	"framework/stream"
	"testing"
)

type pusherTestClient struct{}

func (pusherTestClient) Run() error                 { return nil }
func (pusherTestClient) SendMsg(string, []byte) error { return nil }
func (pusherTestClient) Close() error               { return nil }

func TestPushRejectsInvalidInputAndPayload(t *testing.T) {
	pusher := &Pusher{client: pusherTestClient{}, pushChan: make(chan *stream.PushMessage, 2)}
	message := &stream.Msg{Body: &protocol.Message{ID: 1}}

	pusher.Push(message, nil, map[string]string{"ok": "yes"}, "RoomMessagePush")
	if len(pusher.pushChan) != 1 {
		t.Fatalf("valid push queue length = %d, want 1", len(pusher.pushChan))
	}
	pusher.Push(message, nil, func() {}, "RoomMessagePush")
	pusher.Push(nil, nil, map[string]string{"ignored": "yes"}, "RoomMessagePush")
	if len(pusher.pushChan) != 1 {
		t.Fatalf("invalid push inputs changed queue length to %d", len(pusher.pushChan))
	}
}
