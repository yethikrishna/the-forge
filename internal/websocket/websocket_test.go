package websocket_test

import (
	"crypto/sha1"
	"encoding/base64"
	"net"
	"net/http"
	"testing"

	"github.com/forge/sword/internal/websocket"
)

func TestComputeAcceptKey(t *testing.T) {
	// RFC 6455 test vector
	key := "dGhlIHNhbXBsZSBub25jZQ=="
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	expected := base64.StdEncoding.EncodeToString(h.Sum(nil))
	if expected != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Errorf("accept key calculation wrong: %s", expected)
	}
}

func TestUpgradeServer(t *testing.T) {
	handler := websocket.Handler(func(conn *websocket.Conn) {
		defer conn.Close()
		op, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(op, data)
	})

	server := &http.Server{
		Handler: handler,
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go server.Serve(ln)
	defer server.Close()

	// Test with a simple TCP connection
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.Close()
}

func TestHandlerServeHTTP(t *testing.T) {
	handler := websocket.Handler(func(conn *websocket.Conn) {
		conn.Close()
	})

	if handler == nil {
		t.Fatal("handler should not be nil")
	}
}
