package client

import (
	"fmt"
	"go-rpc/server"
	"net"
	"os"
	"testing"
)

func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

func TestXDial(t *testing.T) {
	ch := make(chan struct{})
	addr := "/tmp/rpc.sock"
	go func() {
		_ = os.Remove(addr)
		l, err := net.Listen("unix", addr)
		if err != nil {
			t.Error("failed to listen unix socket")
			return
		}
		ch <- struct{}{}
		server.Accept(l)
	}()
	<-ch
	_, err := XDial("unix@" + addr)
	_assert(err == nil, "failed to connect unix socket")
}
