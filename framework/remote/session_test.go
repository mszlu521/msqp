package remote

import (
	"encoding/json"
	"framework/stream"
	"testing"
	"time"
)

type sessionTestClient struct {
	sent chan *stream.SessionData
}

func (c *sessionTestClient) Run() error   { return nil }
func (c *sessionTestClient) Close() error { return nil }
func (c *sessionTestClient) SendMsg(_ string, data []byte) error {
	var message stream.Msg
	if err := json.Unmarshal(data, &message); err != nil {
		return err
	}
	c.sent <- message.SessionData
	return nil
}

func TestSessionPutDoesNotShareMutableSessionSnapshot(t *testing.T) {
	client := &sessionTestClient{sent: make(chan *stream.SessionData, 2)}
	message := &stream.Msg{Src: "src", Dst: "dst", Uid: "uid", Cid: "cid"}
	session := NewSession(client, message)
	session.Put("roomId", "first", stream.Single)
	session.Put("roomId", "second", stream.Single)

	select {
	case snapshot := <-client.sent:
		if snapshot.SingleData["roomId"] != "first" {
			t.Fatalf("first snapshot = %#v, want first value", snapshot.SingleData)
		}
	case <-time.After(time.Second):
		t.Fatal("session snapshot was not queued")
	}
}

func TestSessionGetDataReturnsSnapshot(t *testing.T) {
	session := NewSession(nil, &stream.Msg{Uid: "uid"})
	session.Put("roomId", "room-1", stream.Single)

	snapshot := session.GetData()
	snapshot.SingleData["roomId"] = "tampered"
	if value, _ := session.Get("roomId"); value != "room-1" {
		t.Fatalf("session data was exposed: %v", value)
	}
}

func TestNilSessionAccessorsAreSafe(t *testing.T) {
	var session *Session
	if session.GetUid() != "" || session.GetDst() != "" || session.GetMsg() != nil {
		t.Fatal("nil session accessor returned unexpected data")
	}
	if value, ok := session.Get("key"); ok || value != nil {
		t.Fatalf("nil session Get() = %v, %v", value, ok)
	}
}
