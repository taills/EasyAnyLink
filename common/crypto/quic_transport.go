package crypto

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"google.golang.org/grpc"
)

// QUICListener implements net.Listener for QUIC connections
type QUICListener struct {
	listener *quic.Listener
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewQUICListener creates a new QUIC listener
func NewQUICListener(addr string, tlsConfig *tls.Config) (*QUICListener, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen UDP: %w", err)
	}

	quicConfig := &quic.Config{
		MaxIdleTimeout:  300 * 1e9, // 300 seconds
		KeepAlivePeriod: 30 * 1e9,  // 30 seconds
		EnableDatagrams: false,
	}

	listener, err := quic.Listen(udpConn, tlsConfig, quicConfig)
	if err != nil {
		udpConn.Close()
		return nil, fmt.Errorf("failed to create QUIC listener: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &QUICListener{
		listener: listener,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Accept waits for and returns the next connection to the listener
func (l *QUICListener) Accept() (net.Conn, error) {
	conn, err := l.listener.Accept(l.ctx)
	if err != nil {
		return nil, err
	}

	stream, err := conn.AcceptStream(l.ctx)
	if err != nil {
		conn.CloseWithError(0, "failed to accept stream")
		return nil, err
	}

	return &quicStreamConn{
		stream: stream,
		conn:   conn,
	}, nil
}

// Close closes the listener
func (l *QUICListener) Close() error {
	l.cancel()
	return l.listener.Close()
}

// Addr returns the listener's network address
func (l *QUICListener) Addr() net.Addr {
	return l.listener.Addr()
}

// quicStreamConn wraps a QUIC stream to implement net.Conn
type quicStreamConn struct {
	stream quic.Stream
	conn   quic.Connection
	mu     sync.Mutex
}

func (c *quicStreamConn) Read(b []byte) (n int, err error) {
	return c.stream.Read(b)
}

func (c *quicStreamConn) Write(b []byte) (n int, err error) {
	return c.stream.Write(b)
}

func (c *quicStreamConn) Close() error {
	c.stream.Close()
	return c.conn.CloseWithError(0, "connection closed")
}

func (c *quicStreamConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *quicStreamConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *quicStreamConn) SetDeadline(t time.Time) error {
	return c.stream.SetDeadline(t)
}

func (c *quicStreamConn) SetReadDeadline(t time.Time) error {
	return c.stream.SetReadDeadline(t)
}

func (c *quicStreamConn) SetWriteDeadline(t time.Time) error {
	return c.stream.SetWriteDeadline(t)
}

// QUICDialer implements gRPC dialer for QUIC
type QUICDialer struct {
	tlsConfig *tls.Config
}

// NewQUICDialer creates a new QUIC dialer
func NewQUICDialer(tlsConfig *tls.Config) *QUICDialer {
	return &QUICDialer{
		tlsConfig: tlsConfig,
	}
}

// DialContext dials a QUIC connection
func (d *QUICDialer) DialContext(ctx context.Context, addr string) (net.Conn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	quicConfig := &quic.Config{
		MaxIdleTimeout:  300 * 1e9, // 300 seconds
		KeepAlivePeriod: 30 * 1e9,  // 30 seconds
		EnableDatagrams: false,
	}

	conn, err := quic.DialAddr(ctx, udpAddr.String(), d.tlsConfig, quicConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial QUIC: %w", err)
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		conn.CloseWithError(0, "failed to open stream")
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	return &quicStreamConn{
		stream: stream,
		conn:   conn,
	}, nil
}

// GRPCServerOption returns gRPC server options for QUIC transport
func GRPCServerOption(listener *QUICListener) grpc.ServerOption {
	return grpc.Creds(nil) // QUIC handles TLS internally
}

// GRPCDialOption returns gRPC dial options for QUIC transport
func GRPCDialOption(dialer *QUICDialer) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, addr)
	})
}
