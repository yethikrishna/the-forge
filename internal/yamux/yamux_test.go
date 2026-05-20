package yamux_test

import (
	"io"
	"net"
	"testing"

	"github.com/forge/sword/internal/yamux"
)

func TestSessionClientServer(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	server := yamux.Server(serverConn, nil)
	client := yamux.Client(clientConn, nil)

	go func() {
		stream, err := server.AcceptStream()
		if err != nil {
			t.Errorf("server accept: %v", err)
			return
		}
		buf := make([]byte, 1024)
		n, err := stream.Read(buf)
		if err != nil {
			t.Errorf("server read: %v", err)
			return
		}
		stream.Write(buf[:n])
		stream.Close()
	}()

	stream, err := client.OpenStream()
	if err != nil {
		t.Fatalf("client open: %v", err)
	}

	testData := []byte("hello forge")
	_, err = stream.Write(testData)
	if err != nil {
		t.Fatalf("client write: %v", err)
	}

	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("client read: %v", err)
	}

	if string(buf[:n]) != string(testData) {
		t.Errorf("expected %q, got %q", testData, buf[:n])
	}

	client.Close()
	server.Close()
}

func TestSessionClose(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	server := yamux.Server(serverConn, nil)
	client := yamux.Client(clientConn, nil)

	if client.IsClosed() {
		t.Error("session should not be closed initially")
	}

	client.Close()

	if !client.IsClosed() {
		t.Error("session should be closed after Close()")
	}

	server.Close()
}

func TestNumStreams(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	server := yamux.Server(serverConn, nil)
	client := yamux.Client(clientConn, nil)

	if client.NumStreams() != 0 {
		t.Errorf("expected 0 streams, got %d", client.NumStreams())
	}

	client.Close()
	server.Close()
}

func TestDefaultConfig(t *testing.T) {
	cfg := yamux.DefaultConfig()
	if cfg.MaxFrameSize == 0 {
		t.Error("MaxFrameSize should not be zero")
	}
	if cfg.MaxStreamWindowSize == 0 {
		t.Error("MaxStreamWindowSize should not be zero")
	}
}
