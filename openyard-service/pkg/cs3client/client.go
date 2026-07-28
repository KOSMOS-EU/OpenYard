package cs3client

import (
	gateway "github.com/cs3org/go-cs3apis/cs3/gateway/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"time"
)

// Client wraps the CS3 gateway gRPC connection.
type Client struct {
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
			"retryableStatusCodes": ["UNAVAILABLE", "DEADLINE_EXCEEDED"]
		}
	}]
}`

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(
		// dns:/// prefix forces the gRPC DNS resolver which re-resolves
		// periodically (default 30min, but also on connection failure).
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
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:    conn,
		Gateway: gateway.NewGatewayAPIClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
