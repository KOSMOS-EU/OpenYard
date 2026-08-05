package cs3client

import (
	"context"
	"strings"
	"sync"
	"time"

	gateway "github.com/cs3org/go-cs3apis/cs3/gateway/v1beta1"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Client wraps the CS3 gateway gRPC connection with automatic reconnect.
type Client struct {
	addr    string
	mu      sync.RWMutex
	conn    *grpc.ClientConn
	Gateway gateway.GatewayAPIClient
}

// retryPolicy enables automatic retries on transient failures.
const retryPolicy = `{
	"methodConfig": [{
		"name": [{"service": "cs3.gateway.v1beta1.GatewayAPI"}],
		"retryPolicy": {
			"maxAttempts": 3,
			"initialBackoff": "0.5s",
			"maxBackoff": "5s",
			"backoffMultiplier": 2.0,
			"retryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED", "CANCELLED"]
		}
	}]
}`

func dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(
		"dns:///"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)),
		grpc.WithDefaultServiceConfig(retryPolicy),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                5 * time.Minute,
			Timeout:             20 * time.Second,
			PermitWithoutStream: false,
		}),
	)
}

func New(addr string) (*Client, error) {
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	c := &Client{
		addr:    addr,
		conn:    conn,
		Gateway: gateway.NewGatewayAPIClient(conn),
	}
	go c.watchConnection()
	return c, nil
}

// watchConnection monitors the gRPC connection state and reconnects
// when it enters TransientFailure or Shutdown.
func (c *Client) watchConnection() {
	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		state := conn.GetState()
		if !conn.WaitForStateChange(context.Background(), state) {
			return // conn closed
		}
		newState := conn.GetState()
		if newState == connectivity.TransientFailure || newState == connectivity.Shutdown {
			log.Warn().Str("state", newState.String()).Msg("cs3 gateway connection lost, reconnecting")
			c.reconnect()
		}
	}
}

func (c *Client) reconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close old connection (ignore errors)
	c.conn.Close()

	for attempt := 1; ; attempt++ {
		conn, err := dial(c.addr)
		if err == nil {
			c.conn = conn
			c.Gateway = gateway.NewGatewayAPIClient(conn)
			log.Info().Int("attempt", attempt).Msg("cs3 gateway reconnected")
			return
		}
		wait := time.Duration(attempt) * 2 * time.Second
		if wait > 30*time.Second {
			wait = 30 * time.Second
		}
		log.Error().Err(err).Int("attempt", attempt).Dur("retry_in", wait).Msg("cs3 gateway reconnect failed")
		time.Sleep(wait)
	}
}

func (c *Client) Close() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn.Close()
}

// CheckError inspects a gRPC error. If it indicates a broken connection
// ("connection is closing", "transport is closing"), it triggers a reconnect
// and returns true. Callers should retry the operation.
func (c *Client) CheckError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "connection is closing") ||
		strings.Contains(msg, "transport is closing") {
		log.Warn().Msg("cs3 gateway: broken connection detected, reconnecting")
		c.reconnect()
		return true
	}
	return false
}
