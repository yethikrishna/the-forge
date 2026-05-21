package wsep_test

import (
	"bytes"
	"testing"

	"github.com/forge/sword/internal/wsep"
)

func TestChannelTransport(t *testing.T) {
	transport := wsep.NewChannelTransport()
	defer transport.Close()

	msg := wsep.Message{Type: wsep.TypeStdout, Data: []byte("hello")}
	if err := transport.Send(msg); err != nil {
		t.Fatalf("send error: %v", err)
	}

	received := <-transport.Outgoing()
	if received.Type != wsep.TypeStdout {
		t.Errorf("expected stdout type, got %s", received.Type)
	}
	if string(received.Data) != "hello" {
		t.Errorf("expected 'hello', got %s", received.Data)
	}
}

func TestChannelTransportIncoming(t *testing.T) {
	transport := wsep.NewChannelTransport()
	defer transport.Close()

	transport.Incoming() <- wsep.Message{Type: wsep.TypeExit, ExitCode: 0}

	msg, err := transport.Recv()
	if err != nil {
		t.Fatalf("recv error: %v", err)
	}
	if msg.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", msg.ExitCode)
	}
}

func TestJSONTransport(t *testing.T) {
	var buf bytes.Buffer
	pipe := &readWriteCloser{&buf, &buf}

	transport := wsep.NewJSONTransport(pipe)

	msg := wsep.Message{Type: wsep.TypeStderr, Data: []byte("error output")}
	if err := transport.Send(msg); err != nil {
		t.Fatalf("send error: %v", err)
	}

	received, err := transport.Recv()
	if err != nil {
		t.Fatalf("recv error: %v", err)
	}
	if received.Type != wsep.TypeStderr {
		t.Errorf("expected stderr type, got %s", received.Type)
	}
}

func TestExecutor(t *testing.T) {
	transport := wsep.NewChannelTransport()
	executor := wsep.NewExecutor(transport)

	// Simulate: send init, then get response
	go func() {
		msg := <-transport.Outgoing()
		if msg.Type != wsep.TypeInit {
			t.Errorf("expected init type, got %s", msg.Type)
		}
		transport.Incoming() <- wsep.Message{Type: wsep.TypeStdout, Data: []byte("output")}
		transport.Incoming() <- wsep.Message{Type: wsep.TypeExit, ExitCode: 0}
	}()

	_, _, exitCh, err := executor.Execute(nil, wsep.InitMessage{
		Command: []string{"echo", "hello"},
	})
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	code := <-exitCh
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// readWriteCloser wraps bytes.Buffer to implement io.ReadWriteCloser.
type readWriteCloser struct {
	r *bytes.Buffer
	w *bytes.Buffer
}

func (rwc *readWriteCloser) Read(p []byte) (int, error)  { return rwc.r.Read(p) }
func (rwc *readWriteCloser) Write(p []byte) (int, error) { return rwc.w.Write(p) }
func (rwc *readWriteCloser) Close() error                { return nil }
