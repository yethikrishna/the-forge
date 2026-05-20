// Package websocket provides a WebSocket client and server implementation
// following RFC 6455. Built for the forge's real-time communication needs.
package websocket

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// OpCode represents a WebSocket frame opcode.
type OpCode byte

const (
	OpContinuation OpCode = 0x0
	OpText         OpCode = 0x1
	OpBinary       OpCode = 0x2
	OpClose        OpCode = 0x8
	OpPing         OpCode = 0x9
	OpPong         OpCode = 0xA
)

// Close codes.
const (
	CloseNormal          = 1000
	CloseGoingAway       = 1001
	CloseProtocolError   = 1002
	CloseUnsupportedData = 1003
	CloseInternalError   = 1011
)

// ErrClosed is returned when the connection is closed.
var ErrClosed = errors.New("websocket: connection closed")

// Conn represents a WebSocket connection.
type Conn struct {
	conn net.Conn
	br   *bufio.Reader
	mask bool // Client-side connections mask frames
}

// websocketGUID is the magic string for the accept key.
const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Dial connects to a WebSocket server.
func Dial(rawURL string) (*Conn, *http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, fmt.Errorf("websocket: parse url: %w", err)
	}

	// Convert http(s) to ws(s)
	switch u.Scheme {
	case "http", "ws":
		u.Scheme = "http"
	case "https", "wss":
		u.Scheme = "https"
	default:
		return nil, nil, fmt.Errorf("websocket: unsupported scheme %q", u.Scheme)
	}

	// Generate Sec-WebSocket-Key
	key := make([]byte, 16)
	rand.Read(key)
	acceptKey := base64.StdEncoding.EncodeToString(key)

	req := &http.Request{
		Method: "GET",
		URL:    u,
		Header: http.Header{
			"Upgrade":               []string{"websocket"},
			"Connection":            []string{"Upgrade"},
			"Sec-WebSocket-Key":     []string{acceptKey},
			"Sec-WebSocket-Version": []string{"13"},
		},
	}

	// Dial TCP
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	netConn, err := net.Dial("tcp", host)
	if err != nil {
		return nil, nil, fmt.Errorf("websocket: dial: %w", err)
	}

	// Send handshake
	if err := req.Write(netConn); err != nil {
		netConn.Close()
		return nil, nil, fmt.Errorf("websocket: write handshake: %w", err)
	}

	// Read response
	br := bufio.NewReader(netConn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		netConn.Close()
		return nil, nil, fmt.Errorf("websocket: read response: %w", err)
	}

	// Verify accept key
	expected := computeAcceptKey(acceptKey)
	if resp.Header.Get("Sec-WebSocket-Accept") != expected {
		netConn.Close()
		return nil, resp, fmt.Errorf("websocket: invalid accept key")
	}

	return &Conn{conn: netConn, br: br, mask: true}, resp, nil
}

// Upgrade upgrades an HTTP connection to WebSocket.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if r.Header.Get("Upgrade") != "websocket" {
		return nil, fmt.Errorf("websocket: not a websocket request")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("websocket: missing Sec-WebSocket-Key")
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("websocket: response writer cannot hijack")
	}

	netConn, br, err := hj.Hijack()
	if err != nil {
		return nil, fmt.Errorf("websocket: hijack: %w", err)
	}

	acceptKey := computeAcceptKey(key)
	resp := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Accept: %s\r\n\r\n", acceptKey)

	if _, err := netConn.Write([]byte(resp)); err != nil {
		netConn.Close()
		return nil, fmt.Errorf("websocket: write handshake: %w", err)
	}

	return &Conn{conn: netConn, br: br, mask: false}, nil
}

// ReadMessage reads a complete WebSocket message.
func (c *Conn) ReadMessage() (OpCode, []byte, error) {
	var message []byte
	var messageOp OpCode

	for {
		op, data, err := c.readFrame()
		if err != nil {
			return 0, nil, err
		}

		switch op {
		case OpPing:
			c.WriteMessage(OpPong, data)
			continue
		case OpPong:
			continue
		case OpClose:
			return OpClose, data, ErrClosed
		}

		if message == nil {
			messageOp = op
		}
		message = append(message, data...)

		// For simplicity, treat each frame as a complete message
		// A full implementation would check FIN bit
		break
	}

	return messageOp, message, nil
}

// WriteMessage writes a complete WebSocket message.
func (c *Conn) WriteMessage(op OpCode, data []byte) error {
	frame := c.buildFrame(op, true, data)
	_, err := c.conn.Write(frame)
	return err
}

// Close closes the WebSocket connection.
func (c *Conn) Close() error {
	c.WriteMessage(OpClose, nil)
	return c.conn.Close()
}

// RemoteAddr returns the remote address.
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr returns the local address.
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Conn) readFrame() (OpCode, []byte, error) {
	// Read first 2 bytes
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.br, header); err != nil {
		return 0, nil, err
	}

	op := OpCode(header[0] & 0x0F)
	length := int64(header[1] & 0x7F)
	masked := (header[1] & 0x80) != 0

	switch length {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(c.br, ext); err != nil {
			return 0, nil, err
		}
		length = int64(ext[0])<<8 | int64(ext[1])
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(c.br, ext); err != nil {
			return 0, nil, err
		}
		length = int64(ext[0])<<56 | int64(ext[1])<<48 |
			int64(ext[2])<<40 | int64(ext[3])<<32 |
			int64(ext[4])<<24 | int64(ext[5])<<16 |
			int64(ext[6])<<8 | int64(ext[7])
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.br, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	data := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(c.br, data); err != nil {
			return 0, nil, err
		}
	}

	if masked {
		for i := range data {
			data[i] ^= maskKey[i%4]
		}
	}

	return op, data, nil
}

func (c *Conn) buildFrame(op OpCode, fin bool, data []byte) []byte {
	var frame []byte

	b1 := byte(op)
	if fin {
		b1 |= 0x80
	}
	frame = append(frame, b1)

	b2 := byte(0)
	if c.mask {
		b2 |= 0x80
	}

	length := len(data)
	switch {
	case length <= 125:
		frame = append(frame, b2|byte(length))
	case length <= 65535:
		frame = append(frame, b2|126)
		frame = append(frame, byte(length>>8), byte(length))
	default:
		frame = append(frame, b2|127)
		for i := 7; i >= 0; i-- {
			frame = append(frame, byte(length>>(i*8)))
		}
	}

	if c.mask {
		var maskKey [4]byte
		rand.Read(maskKey[:])
		frame = append(frame, maskKey[:]...)

		masked := make([]byte, length)
		for i := range data {
			masked[i] = data[i] ^ maskKey[i%4]
		}
		frame = append(frame, masked...)
	} else {
		frame = append(frame, data...)
	}

	return frame
}

// Handler is a function that handles WebSocket connections.
type Handler func(*Conn)

// ServeHTTP implements http.Handler for WebSocket upgrades.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrade(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h(conn)
}

func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
