package meowcaller

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRelayConn struct {
	id int
}

// blockedConnect returns a connect func that completes only when release is closed.
func blockedConnect(conn *fakeRelayConn, err error, release <-chan struct{}) func() (*fakeRelayConn, error) {
	return func() (*fakeRelayConn, error) {
		<-release
		return conn, err
	}
}

func waitClosed(t *testing.T, closed <-chan *fakeRelayConn) *fakeRelayConn {
	t.Helper()
	select {
	case conn := <-closed:
		return conn
	case <-time.After(2 * time.Second):
		t.Fatal("late connection was never closed")
		return nil
	}
}

func assertNotClosed(t *testing.T, closed <-chan *fakeRelayConn) {
	t.Helper()
	select {
	case conn := <-closed:
		t.Fatalf("connection %d was closed, want it kept", conn.id)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestAwaitRelayConnectReturnsConnection(t *testing.T) {
	closed := make(chan *fakeRelayConn, 1)
	release := make(chan struct{})
	close(release)
	want := &fakeRelayConn{id: 1}

	got, err := awaitRelayConnect(context.Background(), time.Second, blockedConnect(want, nil, release),
		func(c *fakeRelayConn) { closed <- c })

	if err != nil || got != want {
		t.Fatalf("awaitRelayConnect() = %v, %v; want connection %d", got, err, want.id)
	}
	assertNotClosed(t, closed)
}

func TestAwaitRelayConnectWrapsConnectError(t *testing.T) {
	closed := make(chan *fakeRelayConn, 1)
	release := make(chan struct{})
	close(release)
	boom := errors.New("dtls failed")

	_, err := awaitRelayConnect(context.Background(), time.Second, blockedConnect(nil, boom, release),
		func(c *fakeRelayConn) { closed <- c })

	if !errors.Is(err, boom) || err.Error() != "relay connect: dtls failed" {
		t.Fatalf("awaitRelayConnect() error = %v, want wrapped %v", err, boom)
	}
	assertNotClosed(t, closed)
}

func TestAwaitRelayConnectClosesConnectionCompletedAfterCancel(t *testing.T) {
	closed := make(chan *fakeRelayConn, 1)
	release := make(chan struct{})
	late := &fakeRelayConn{id: 2}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := awaitRelayConnect(ctx, time.Minute, blockedConnect(late, nil, release),
		func(c *fakeRelayConn) { closed <- c })

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("awaitRelayConnect() error = %v, want %v", err, context.Canceled)
	}
	close(release)
	if got := waitClosed(t, closed); got != late {
		t.Fatalf("closed connection %d, want %d", got.id, late.id)
	}
}

func TestAwaitRelayConnectClosesConnectionCompletedAfterTimeout(t *testing.T) {
	closed := make(chan *fakeRelayConn, 1)
	release := make(chan struct{})
	late := &fakeRelayConn{id: 3}

	_, err := awaitRelayConnect(context.Background(), 10*time.Millisecond, blockedConnect(late, nil, release),
		func(c *fakeRelayConn) { closed <- c })

	if err == nil || err.Error() != "relay connect timed out (DTLS didn't complete)" {
		t.Fatalf("awaitRelayConnect() error = %v, want timeout", err)
	}
	close(release)
	if got := waitClosed(t, closed); got != late {
		t.Fatalf("closed connection %d, want %d", got.id, late.id)
	}
}

func TestAwaitRelayConnectIgnoresConnectFailingAfterCancel(t *testing.T) {
	closed := make(chan *fakeRelayConn, 1)
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _ = awaitRelayConnect(ctx, time.Minute, blockedConnect(nil, errors.New("dtls failed"), release),
		func(c *fakeRelayConn) { closed <- c })

	close(release)
	assertNotClosed(t, closed)
}
