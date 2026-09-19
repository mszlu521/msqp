package node

import (
	"testing"
)

func TestDecodeRemoteMessageRequiresBody(t *testing.T) {
	if _, err := decodeRemoteMessage([]byte(`{"src":"game"}`)); err == nil {
		t.Fatal("message without body was accepted")
	}
	if _, err := decodeRemoteMessage([]byte(`{"body":null}`)); err == nil {
		t.Fatal("null message body was accepted")
	}
}

func TestDecodeRemoteMessageAcceptsValidMessage(t *testing.T) {
	message, err := decodeRemoteMessage([]byte(`{"src":"game","body":{"id":7,"data":"e30="}}`))
	if err != nil {
		t.Fatalf("valid message rejected: %v", err)
	}
	if message.Body == nil || message.Body.ID != 7 {
		t.Fatalf("decoded message = %#v", message)
	}
}
