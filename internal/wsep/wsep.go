// Package wsep implements the workspace execution protocol for running
// commands remotely over WebSocket. Execute anywhere the forge reaches.
package wsep

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// MessageType identifies the type of message in the protocol.
type MessageType string

const (
	TypeInit   MessageType = "init"
	TypeStdin  MessageType = "stdin"
	TypeStdout MessageType = "stdout"
	TypeStderr MessageType = "stderr"
	TypeExit   MessageType = "exit"
	TypeResize MessageType = "resize"
	TypeSignal MessageType = "signal"
	TypeError  MessageType = "error"
)

// Message is a protocol message.
type Message struct {
	Type     MessageType `json:"type"`
	Data     []byte      `json:"data,omitempty"`
	ExitCode int         `json:"exit_code,omitempty"`
	Error    string      `json:"error,omitempty"`
	Width    int         `json:"width,omitempty"`
	Height   int         `json:"height,omitempty"`
	Signal   string      `json:"signal,omitempty"`
}

// InitMessage is the initial message to start a command.
type InitMessage struct {
	Command []string          `json:"command"`
	Env     map[string]string `json:"env,omitempty"`
	Dir     string            `json:"dir,omitempty"`
	TTY     bool              `json:"tty,omitempty"`
	Width   int               `json:"width,omitempty"`
	Height  int               `json:"height,omitempty"`
}

// Transport sends and receives protocol messages.
type Transport interface {
	Send(msg Message) error
	Recv() (Message, error)
	Close() error
}

// ChannelTransport uses channels for in-process communication.
type ChannelTransport struct {
	sendCh chan Message
	recvCh chan Message
	closed chan struct{}
}

// NewChannelTransport creates a channel-based transport.
func NewChannelTransport() *ChannelTransport {
	return &ChannelTransport{
		sendCh: make(chan Message, 64),
		recvCh: make(chan Message, 64),
		closed: make(chan struct{}),
	}
}

// Send sends a message.
func (t *ChannelTransport) Send(msg Message) error {
	select {
	case t.sendCh <- msg:
		return nil
	case <-t.closed:
		return fmt.Errorf("wsep: transport closed")
	}
}

// Recv receives a message.
func (t *ChannelTransport) Recv() (Message, error) {
	select {
	case msg := <-t.recvCh:
		return msg, nil
	case <-t.closed:
		return Message{}, io.EOF
	}
}

// Close closes the transport.
func (t *ChannelTransport) Close() error {
	close(t.closed)
	return nil
}

// Outgoing returns the channel of outgoing messages.
func (t *ChannelTransport) Outgoing() <-chan Message {
	return t.sendCh
}

// Incoming returns the channel for incoming messages.
func (t *ChannelTransport) Incoming() chan<- Message {
	return t.recvCh
}

// JSONTransport uses JSON encoding over a read/write closer.
type JSONTransport struct {
	encoder *json.Encoder
	decoder *json.Decoder
	mu      sync.Mutex
	closer  io.Closer
}

// NewJSONTransport creates a JSON-based transport.
func NewJSONTransport(rw io.ReadWriteCloser) *JSONTransport {
	return &JSONTransport{
		encoder: json.NewEncoder(rw),
		decoder: json.NewDecoder(rw),
		closer:  rw,
	}
}

// Send sends a JSON-encoded message.
func (t *JSONTransport) Send(msg Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.encoder.Encode(msg)
}

// Recv receives a JSON-decoded message.
func (t *JSONTransport) Recv() (Message, error) {
	var msg Message
	if err := t.decoder.Decode(&msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}

// Close closes the underlying connection.
func (t *JSONTransport) Close() error {
	return t.closer.Close()
}

// Executor runs commands via the protocol.
type Executor struct {
	transport Transport
}

// NewExecutor creates a new command executor.
func NewExecutor(transport Transport) *Executor {
	return &Executor{transport: transport}
}

// Execute runs a command and streams output.
func (e *Executor) Execute(ctx context.Context, init InitMessage) (stdout, stderr io.Reader, exitCode <-chan int, err error) {
	initData, _ := json.Marshal(init)
	if err := e.transport.Send(Message{
		Type: TypeInit,
		Data: initData,
	}); err != nil {
		return nil, nil, nil, fmt.Errorf("wsep: send init: %w", err)
	}

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	exitCh := make(chan int, 1)

	go func() {
		defer close(exitCh)
		defer stdoutW.Close()
		defer stderrW.Close()

		for {
			msg, err := e.transport.Recv()
			if err != nil {
				exitCh <- -1
				return
			}

			switch msg.Type {
			case TypeStdout:
				stdoutW.Write(msg.Data)
			case TypeStderr:
				stderrW.Write(msg.Data)
			case TypeExit:
				exitCh <- msg.ExitCode
				return
			case TypeError:
				stderrW.Write([]byte(msg.Error))
				exitCh <- -1
				return
			}
		}
	}()

	return stdoutR, stderrR, exitCh, nil
}

// Resize sends a terminal resize message.
func (e *Executor) Resize(width, height int) error {
	return e.transport.Send(Message{
		Type:   TypeResize,
		Width:  width,
		Height: height,
	})
}

// Signal sends a signal to the running process.
func (e *Executor) Signal(signal string) error {
	return e.transport.Send(Message{
		Type:   TypeSignal,
		Signal: signal,
	})
}

// Stdin sends input to the running process.
func (e *Executor) Stdin(data []byte) error {
	return e.transport.Send(Message{
		Type: TypeStdin,
		Data: data,
	})
}
