// Package redjet provides a lightweight Redis client with connection
// pooling and common command support. A red jet of data through the forge.
package redjet

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config holds Redis client configuration.
type Config struct {
	Addr         string        // host:port, default "localhost:6379"
	Password     string        // AUTH password
	DB           int           // Database number
	DialTimeout  time.Duration // Connection timeout
	ReadTimeout  time.Duration // Read timeout
	WriteTimeout time.Duration // Write timeout
	PoolSize     int           // Max connections in pool, default 10
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Addr:        "localhost:6379",
		DialTimeout: 5 * time.Second,
		PoolSize:    10,
	}
}

// Client is a Redis client.
type Client struct {
	config Config
	pool   chan *conn
	mu     sync.Mutex
}

type conn struct {
	net.Conn
	br *bufio.Reader
}

// New creates a new Redis client.
func New(cfg Config) *Client {
	if cfg.Addr == "" {
		cfg.Addr = "localhost:6379"
	}
	if cfg.PoolSize <= 0 {
		cfg.PoolSize = 10
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}

	return &Client{
		config: cfg,
		pool:   make(chan *conn, cfg.PoolSize),
	}
}

// Close closes all connections in the pool.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	for {
		select {
		case cn := <-c.pool:
			if closeErr := cn.Close(); closeErr != nil {
				err = closeErr
			}
		default:
			return err
		}
	}
}

// Get retrieves a key's value.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.do(ctx, "GET", key)
}

// Set sets a key's value with optional expiration.
func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	args := []string{key, value}
	if ttl > 0 {
		args = append(args, "PX", strconv.FormatInt(ttl.Milliseconds(), 10))
	}
	_, err := c.do(ctx, "SET", args...)
	return err
}

// Del deletes one or more keys.
func (c *Client) Del(ctx context.Context, keys ...string) (int64, error) {
	args := []string{}
	args = append(args, keys...)
	result, err := c.do(ctx, "DEL", args...)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(result, 10, 64)
}

// Exists checks if a key exists.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.do(ctx, "EXISTS", key)
	if err != nil {
		return false, err
	}
	return result == "1", nil
}

// Expire sets a key's expiration.
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	_, err := c.do(ctx, "PEXPIRE", key, strconv.FormatInt(ttl.Milliseconds(), 10))
	return err
}

// TTL returns the remaining time to live of a key.
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	result, err := c.do(ctx, "PTTL", key)
	if err != nil {
		return 0, err
	}
	ms, err := strconv.ParseInt(result, 10, 64)
	if ms < 0 {
		return -1, nil
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// Incr increments a key's integer value.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	result, err := c.do(ctx, "INCR", key)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(result, 10, 64)
}

// Ping checks the connection.
func (c *Client) Ping(ctx context.Context) (string, error) {
	return c.do(ctx, "PING")
}

// HGet gets a hash field.
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	return c.do(ctx, "HGET", key, field)
}

// HSet sets a hash field.
func (c *Client) HSet(ctx context.Context, key, field, value string) error {
	_, err := c.do(ctx, "HSET", key, field, value)
	return err
}

// LPush prepends a value to a list.
func (c *Client) LPush(ctx context.Context, key, value string) error {
	_, err := c.do(ctx, "LPUSH", key, value)
	return err
}

// RPush appends a value to a list.
func (c *Client) RPush(ctx context.Context, key, value string) error {
	_, err := c.do(ctx, "RPUSH", key, value)
	return err
}

// LPop removes and returns the first element of a list.
func (c *Client) LPop(ctx context.Context, key string) (string, error) {
	return c.do(ctx, "LPOP", key)
}

// SAdd adds a member to a set.
func (c *Client) SAdd(ctx context.Context, key, member string) error {
	_, err := c.do(ctx, "SADD", key, member)
	return err
}

// Publish publishes a message to a channel.
func (c *Client) Publish(ctx context.Context, channel, message string) (int64, error) {
	result, err := c.do(ctx, "PUBLISH", channel, message)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(result, 10, 64)
}

// do executes a Redis command.
func (c *Client) do(ctx context.Context, command string, args ...string) (string, error) {
	cn, err := c.getConn(ctx)
	if err != nil {
		return "", fmt.Errorf("redjet: connect: %w", err)
	}

	// Build RESP command
	cmd := fmt.Sprintf("*%d\r\n", len(args)+1)
	cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(command), command)
	for _, arg := range args {
		cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg)
	}

	// Write
	if c.config.WriteTimeout > 0 {
		cn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	}
	if _, err := fmt.Fprint(cn, cmd); err != nil {
		cn.Close()
		return "", fmt.Errorf("redjet: write: %w", err)
	}

	// Read response
	if c.config.ReadTimeout > 0 {
		cn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	}
	result, err := readResp(cn.br)
	if err != nil {
		cn.Close()
		return "", fmt.Errorf("redjet: read: %w", err)
	}

	// Return connection to pool
	c.putConn(cn)

	return result, nil
}

func (c *Client) getConn(ctx context.Context) (*conn, error) {
	select {
	case cn := <-c.pool:
		return cn, nil
	default:
	}

	dialer := net.Dialer{Timeout: c.config.DialTimeout}
	netConn, err := dialer.DialContext(ctx, "tcp", c.config.Addr)
	if err != nil {
		return nil, err
	}

	cn := &conn{Conn: netConn, br: bufio.NewReader(netConn)}

	// AUTH if password set
	if c.config.Password != "" {
		_, err = c.doWithConn(cn, "AUTH", c.config.Password)
		if err != nil {
			cn.Close()
			return nil, err
		}
	}

	// SELECT database
	if c.config.DB > 0 {
		_, err = c.doWithConn(cn, "SELECT", strconv.Itoa(c.config.DB))
		if err != nil {
			cn.Close()
			return nil, err
		}
	}

	return cn, nil
}

func (c *Client) putConn(cn *conn) {
	select {
	case c.pool <- cn:
	default:
		cn.Close()
	}
}

func (c *Client) doWithConn(cn *conn, command string, args ...string) (string, error) {
	cmd := fmt.Sprintf("*%d\r\n", len(args)+1)
	cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(command), command)
	for _, arg := range args {
		cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg)
	}
	fmt.Fprint(cn, cmd)
	return readResp(cn.br)
}

// readResp reads a RESP protocol response.
func readResp(br *bufio.Reader) (string, error) {
	line, err := br.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(line, "\r\n")

	switch line[0] {
	case '+': // Simple string
		return line[1:], nil
	case '-': // Error
		return "", fmt.Errorf("redis: %s", line[1:])
	case ':': // Integer
		return line[1:], nil
	case '$': // Bulk string
		length, _ := strconv.Atoi(line[1:])
		if length < 0 {
			return "", nil // Nil
		}
		data := make([]byte, length+2)
		if _, err := br.Read(data); err != nil {
			return "", err
		}
		return string(data[:length]), nil
	case '*': // Array
		count, _ := strconv.Atoi(line[1:])
		var results []string
		for i := 0; i < count; i++ {
			s, err := readResp(br)
			if err != nil {
				return "", err
			}
			results = append(results, s)
		}
		return strings.Join(results, ","), nil
	default:
		return "", fmt.Errorf("redjet: unknown response type: %c", line[0])
	}
}
