// Package yamux provides stream multiplexing over a single net.Conn.
// Many streams, one connection — like many blades, one forge.
package yamux

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Protocol constants.
const (
	version      = 1
	headerSize   = 12 // streamID(4) + length(4) + flags(4)
	maxStreamID  = 1 << 31
	windowSize   = 256 * 1024 // 256KB initial window
	maxFrameSize = 32 * 1024  // 32KB max frame payload
)

// Frame types (flags).
const (
	flagSYN  uint32 = 1 << iota // Stream open
	flagACK                      // Stream acknowledged
	flagFIN                      // Stream closed (write side)
	flagRST                      // Stream reset
	flagDATA                     // Data frame
)

// ErrSessionClosed is returned when the session is closed.
var ErrSessionClosed = errors.New("yamux: session closed")

// ErrStreamClosed is returned when a stream is closed.
var ErrStreamClosed = errors.New("yamux: stream closed")

// Config holds session configuration.
type Config struct {
	MaxStreamWindowSize uint32
	MaxFrameSize        uint32
	KeepAliveInterval   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		MaxStreamWindowSize: windowSize,
		MaxFrameSize:        maxFrameSize,
		KeepAliveInterval:   30 * time.Second,
	}
}

// Session manages a multiplexed connection.
type Session struct {
	conn    net.Conn
	config  *Config
	isClient bool

	nextStreamID atomic.Uint32
	streams      map[uint32]*Stream
	streamLock   sync.Mutex

	readCh  chan []byte
	errCh   chan error
	closeCh chan struct{}

	sendLock sync.Mutex
}

// Client creates a new client session over the connection.
func Client(conn net.Conn, config *Config) *Session {
	return newSession(conn, config, true)
}

// Server creates a new server session over the connection.
func Server(conn net.Conn, config *Config) *Session {
	return newSession(conn, config, false)
}

func newSession(conn net.Conn, config *Config, isClient bool) *Session {
	if config == nil {
		config = DefaultConfig()
	}

	s := &Session{
		conn:      conn,
		config:    config,
		isClient:  isClient,
		streams:   make(map[uint32]*Stream),
		readCh:    make(chan []byte, 64),
		errCh:     make(chan error, 1),
		closeCh:   make(chan struct{}),
	}

	// Start client stream ID at 1 or 2 depending on role
	if isClient {
		s.nextStreamID.Store(1)
	} else {
		s.nextStreamID.Store(2)
	}

	go s.recvLoop()

	return s
}

// OpenStream opens a new bidirectional stream.
func (s *Session) OpenStream() (*Stream, error) {
	select {
	case <-s.closeCh:
		return nil, ErrSessionClosed
	default:
	}

	streamID := s.nextStreamID.Add(2)
	stream := newStream(streamID, s)

	s.streamLock.Lock()
	s.streams[streamID] = stream
	s.streamLock.Unlock()

	// Send SYN to open the stream
	if err := s.sendFrame(streamID, flagSYN|flagDATA, nil); err != nil {
		s.streamLock.Lock()
		delete(s.streams, streamID)
		s.streamLock.Unlock()
		return nil, fmt.Errorf("yamux: open stream: %w", err)
	}

	return stream, nil
}

// AcceptStream waits for and returns the next incoming stream.
func (s *Session) AcceptStream() (*Stream, error) {
	for {
		select {
		case <-s.closeCh:
			return nil, ErrSessionClosed
		case data := <-s.readCh:
			streamID, _, flags := parseHeader(data)
			if flags&flagSYN != 0 {
				stream := newStream(streamID, s)
				s.streamLock.Lock()
				s.streams[streamID] = stream
				s.streamLock.Unlock()

				// Send ACK
				s.sendFrame(streamID, flagACK, nil)
				return stream, nil
			}
		}
	}
}

// Close closes the session and all streams.
func (s *Session) Close() error {
	select {
	case <-s.closeCh:
		return nil
	default:
	}

	close(s.closeCh)

	s.streamLock.Lock()
	for id, stream := range s.streams {
		stream.close()
		delete(s.streams, id)
	}
	s.streamLock.Unlock()

	return s.conn.Close()
}

// LocalAddr returns the local address.
func (s *Session) LocalAddr() net.Addr { return s.conn.LocalAddr() }

// RemoteAddr returns the remote address.
func (s *Session) RemoteAddr() net.Addr { return s.conn.RemoteAddr() }

// NumStreams returns the number of active streams.
func (s *Session) NumStreams() int {
	s.streamLock.Lock()
	defer s.streamLock.Unlock()
	return len(s.streams)
}

// IsClosed returns whether the session is closed.
func (s *Session) IsClosed() bool {
	select {
	case <-s.closeCh:
		return true
	default:
		return false
	}
}

func (s *Session) sendFrame(streamID uint32, flags uint32, data []byte) error {
	s.sendLock.Lock()
	defer s.sendLock.Unlock()

	header := make([]byte, headerSize)
	binary.BigEndian.PutUint32(header[0:4], streamID)
	binary.BigEndian.PutUint32(header[4:8], uint32(len(data)))
	binary.BigEndian.PutUint32(header[8:12], flags)

	if _, err := s.conn.Write(header); err != nil {
		return fmt.Errorf("yamux: write header: %w", err)
	}

	if len(data) > 0 {
		if _, err := s.conn.Write(data); err != nil {
			return fmt.Errorf("yamux: write data: %w", err)
		}
	}

	return nil
}

func (s *Session) recvLoop() {
	header := make([]byte, headerSize)
	for {
		select {
		case <-s.closeCh:
			return
		default:
		}

		if _, err := io.ReadFull(s.conn, header); err != nil {
			s.errCh <- err
			return
		}

		streamID, length, flags := parseHeader(header)

		var data []byte
		if length > 0 {
			data = make([]byte, length)
			if _, err := io.ReadFull(s.conn, data); err != nil {
				s.errCh <- err
				return
			}
		}

		// Handle control frames
		if flags&flagSYN != 0 {
			s.readCh <- header
			continue
		}

		if flags&flagRST != 0 {
			s.streamLock.Lock()
			if stream, ok := s.streams[streamID]; ok {
				stream.close()
				delete(s.streams, streamID)
			}
			s.streamLock.Unlock()
			continue
		}

		if flags&flagFIN != 0 {
			s.streamLock.Lock()
			if stream, ok := s.streams[streamID]; ok {
				stream.closeWrite()
			}
			s.streamLock.Unlock()
			continue
		}

		// Data frame
		if len(data) > 0 {
			s.streamLock.Lock()
			if stream, ok := s.streams[streamID]; ok {
				stream.pushData(data)
			}
			s.streamLock.Unlock()
		}
	}
}

func parseHeader(header []byte) (streamID uint32, length uint32, flags uint32) {
	streamID = binary.BigEndian.Uint32(header[0:4])
	length = binary.BigEndian.Uint32(header[4:8])
	flags = binary.BigEndian.Uint32(header[8:12])
	return
}

// Stream represents a multiplexed stream.
type Stream struct {
	id      uint32
	session *Session

	readBuf  []byte
	readCh   chan []byte
	writeCh  chan []byte
	closeCh  chan struct{}
	closed   atomic.Bool
	writeClosed atomic.Bool
}

func newStream(id uint32, session *Session) *Stream {
	return &Stream{
		id:      id,
		session: session,
		readCh:  make(chan []byte, 64),
		closeCh: make(chan struct{}),
	}
}

// Read reads data from the stream.
func (st *Stream) Read(p []byte) (int, error) {
	if st.closed.Load() {
		return 0, io.EOF
	}

	// Drain buffer first
	if len(st.readBuf) > 0 {
		n := copy(p, st.readBuf)
		st.readBuf = st.readBuf[n:]
		return n, nil
	}

	select {
	case data := <-st.readCh:
		n := copy(p, data)
		if n < len(data) {
			st.readBuf = append(st.readBuf, data[n:]...)
		}
		return n, nil
	case <-st.closeCh:
		return 0, io.EOF
	}
}

// Write writes data to the stream.
func (st *Stream) Write(p []byte) (int, error) {
	if st.closed.Load() {
		return 0, ErrStreamClosed
	}

	// Chunk into frames
	sent := 0
	maxFrame := int(st.session.config.MaxFrameSize)
	for sent < len(p) {
		end := sent + maxFrame
		if end > len(p) {
			end = len(p)
		}

		if err := st.session.sendFrame(st.id, flagDATA, p[sent:end]); err != nil {
			return sent, err
		}
		sent = end
	}

	return sent, nil
}

// Close closes the stream.
func (st *Stream) Close() error {
	st.close()
	st.session.sendFrame(st.id, flagFIN, nil)
	return nil
}

func (st *Stream) close() {
	if st.closed.CompareAndSwap(false, true) {
		close(st.closeCh)
	}
}

func (st *Stream) closeWrite() {
	if st.writeClosed.CompareAndSwap(false, true) {
		// Signal EOF on reads
		st.close()
	}
}

func (st *Stream) pushData(data []byte) {
	select {
	case st.readCh <- data:
	default:
		// Buffer full, append to readBuf
		st.readBuf = append(st.readBuf, data...)
	}
}

// StreamID returns the stream's ID.
func (st *Stream) StreamID() uint32 {
	return st.id
}
