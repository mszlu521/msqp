package discovery

import (
	"common/config"
	"testing"
)

func TestResolverCloseIsSafeBeforeAndAfterBuild(t *testing.T) {
	resolver := NewResolver(config.EtcdConf{})
	resolver.Close()
	resolver.Close()
}

func TestRegisterCloseIsSafeBeforeAndAfterRegister(t *testing.T) {
	register := NewRegister()
	register.Close()
	register.closeCh = make(chan struct{})
	register.Close()
	register.Close()

	select {
	case <-register.closeCh:
	default:
		t.Fatal("register close channel was not closed")
	}
}

func TestParseKey(t *testing.T) {
	server, err := ParseKey("game/v1/127.0.0.1:12000")
	if err != nil {
		t.Fatalf("ParseKey returned error: %v", err)
	}
	if server.Name != "game" || server.Version != "v1" || server.Addr != "127.0.0.1:12000" {
		t.Fatalf("unexpected server: %+v", server)
	}

	if _, err := ParseKey("invalid/key/with/too/many/parts"); err == nil {
		t.Fatal("ParseKey accepted an invalid key")
	}
}
