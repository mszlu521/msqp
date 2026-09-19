package net

import (
	"encoding/json"
	"framework/protocol"
	"framework/stream"
	"sync"
	"testing"
	"time"
)

type orderedPushConnection struct {
	mu       sync.Mutex
	session  *Session
	payloads []string
}

func (c *orderedPushConnection) Close() {}

func (c *orderedPushConnection) SendMessage(data []byte) error {
	packet, err := protocol.Decode(data)
	if err != nil {
		return err
	}
	message := packet.Body.(protocol.Message)
	c.mu.Lock()
	c.payloads = append(c.payloads, string(message.Data))
	c.mu.Unlock()
	return nil
}

func (c *orderedPushConnection) GetSession() *Session { return c.session }

type blockingPushConnection struct {
	session *Session
	release <-chan struct{}
}

func (c *blockingPushConnection) Close() {}

func (c *blockingPushConnection) SendMessage([]byte) error {
	<-c.release
	return nil
}

func (c *blockingPushConnection) GetSession() *Session { return c.session }

func TestResponsePreservesConsecutivePushOrder(t *testing.T) {
	manager := &Manager{clientBuckets: []*ClientBucket{NewClientBucket()}}
	connection := &orderedPushConnection{session: NewSession("c1", manager)}
	connection.session.Uid = "u1"
	bucket := manager.getBucket("c1")
	bucket.clients["c1"] = connection

	for _, payload := range []string{"ready", "start"} {
		manager.Response(&stream.Msg{
			Body:     &protocol.Message{Type: protocol.Push, Route: "ServerMessagePush", Data: []byte(payload)},
			PushUser: []string{"u1"},
		})
	}

	connection.mu.Lock()
	defer connection.mu.Unlock()
	if len(connection.payloads) != 2 || connection.payloads[0] != "ready" || connection.payloads[1] != "start" {
		t.Fatalf("push order = %#v, want [ready start]", connection.payloads)
	}
}

func TestRemoteReadDoesNotBlockWhenPushQueueIsFull(t *testing.T) {
	manager := &Manager{
		RemoteReadChan: make(chan []byte, 1),
		RemotePushChan: make(chan *stream.Msg, 1),
		clientBuckets:  []*ClientBucket{NewClientBucket()},
	}
	manager.RemotePushChan <- &stream.Msg{}
	release := make(chan struct{})
	connection := &blockingPushConnection{session: NewSession("c1", manager), release: release}
	connection.session.Uid = "u1"
	manager.getBucket("c1").clients["c1"] = connection

	body, err := json.Marshal(stream.Msg{
		Body:     &protocol.Message{Type: protocol.Push, Route: "ServerMessagePush"},
		PushUser: []string{"u1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	manager.RemoteReadChan <- body
	close(manager.RemoteReadChan)

	done := make(chan struct{})
	go func() {
		manager.remoteReadChanHandler()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("remote read handler blocked on a full push queue")
	}
	close(release)
	manager.remotePushMu.Lock()
	overflowCount := len(manager.remotePushOverflow)
	manager.remotePushMu.Unlock()
	if overflowCount != 1 {
		t.Fatalf("overflow queue length = %d, want 1", overflowCount)
	}
}

func TestRemotePushOverflowPreservesEnqueueOrder(t *testing.T) {
	manager := &Manager{RemotePushChan: make(chan *stream.Msg, 1)}
	messages := []*stream.Msg{{Router: "first"}, {Router: "second"}, {Router: "third"}}
	for _, msg := range messages {
		manager.enqueueRemotePush(msg)
	}
	if got := (<-manager.RemotePushChan).Router; got != "first" {
		t.Fatalf("channel message = %q, want first", got)
	}
	overflow := manager.takeRemotePushOverflow(10)
	if len(overflow) != 2 || overflow[0].Router != "second" || overflow[1].Router != "third" {
		t.Fatalf("overflow order = %#v, want second/third", overflow)
	}
}
