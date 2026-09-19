package remote

import "testing"

func TestNatsClientSendMsgRequiresConnection(t *testing.T) {
	client := NewNatsClient("server", make(chan []byte, 1))
	if err := client.SendMsg("destination", []byte("payload")); err == nil {
		t.Fatal("SendMsg reported success without a NATS connection")
	}
}
