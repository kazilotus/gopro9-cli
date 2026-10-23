package stream_test

import (
	"bytes"
	"context"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/stream"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

func TestStartStopAndReceive(t *testing.T) {
	t.Parallel()
	cam := fakecam.New(t)
	conn, err := stream.Listen(0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	port := conn.LocalAddr().(*net.UDPAddr).Port
	cam.StreamPort = port
	cam.StreamPackets = 10

	c := stream.New(transport.New(cam.Endpoint()))
	ctx := context.Background()
	require.NoError(t, c.Start(ctx, port))
	assert.True(t, cam.Streaming)
	assert.Contains(t, strings.Join(cam.Snapshot(), "\n"), "/gopro/camera/stream/start?port=")

	var buf bytes.Buffer
	var calls int
	stats, err := stream.Receive(ctx, conn, &buf, stream.ReceiveOptions{IdleTimeout: 500 * time.Millisecond, OnPacket: func(stream.Stats) { calls++ }})
	require.NoError(t, err)
	assert.Equal(t, int64(10), stats.Packets)
	assert.Equal(t, int64(10*188*7), stats.Bytes)
	assert.Equal(t, 10, calls)
	assert.Equal(t, byte(0x47), buf.Bytes()[0], "MPEG-TS sync byte")
	assert.Positive(t, stats.Bitrate())
	assert.GreaterOrEqual(t, stats.FirstPacket, time.Duration(0)) // Windows clocks tick at ~15 ms; the first packet may land in the same tick

	require.NoError(t, c.Stop(ctx))
	assert.False(t, cam.Streaming)
	require.NoError(t, c.Start(ctx, 0))
	reqs := cam.Snapshot()
	assert.Equal(t, "GET /gopro/camera/stream/start", reqs[len(reqs)-1], "port 0 sends no parameter")
}

func TestReceiveNoData(t *testing.T) {
	t.Parallel()
	conn, err := stream.Listen(0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = stream.Receive(context.Background(), conn, &bytes.Buffer{}, stream.ReceiveOptions{IdleTimeout: 100 * time.Millisecond})
	require.ErrorIs(t, err, stream.ErrNoData)
}

func TestReceiveMaxDurationAndCancel(t *testing.T) {
	t.Parallel()
	conn, err := stream.Listen(0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	port := conn.LocalAddr().(*net.UDPAddr).Port

	// Feed packets continuously from a goroutine.
	stopFeed := make(chan struct{})
	go func() {
		out, _ := net.Dial("udp", net.JoinHostPort("127.0.0.1", itoa(port)))
		defer func() { _ = out.Close() }()
		pkt := make([]byte, 188)
		pkt[0] = 0x47
		for {
			select {
			case <-stopFeed:
				return
			default:
				_, _ = out.Write(pkt)
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()
	t.Cleanup(func() { close(stopFeed) })

	stats, err := stream.Receive(context.Background(), conn, &bytes.Buffer{}, stream.ReceiveOptions{MaxDuration: 150 * time.Millisecond})
	require.NoError(t, err)
	assert.Positive(t, stats.Packets)
	assert.GreaterOrEqual(t, stats.Duration, 150*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	stats, err = stream.Receive(ctx, conn, &bytes.Buffer{}, stream.ReceiveOptions{})
	require.NoError(t, err)
	assert.Positive(t, stats.Packets)
}

func itoa(n int) string { return strings.TrimSpace(strings.Repeat("", 0) + fmtInt(n)) }

func fmtInt(n int) string {
	b := []byte{}
	if n == 0 {
		return "0"
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestListenConflict(t *testing.T) {
	t.Parallel()
	conn, err := stream.Listen(0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = stream.Listen(conn.LocalAddr().(*net.UDPAddr).Port)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listen udp")
}

func TestURLAndPlayers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "udp://@:8554", stream.URL(0))
	assert.Equal(t, "udp://@:9000", stream.URL(9000))
	_, err := stream.FindPlayer("definitely-not-a-player")
	require.Error(t, err)
	p, err := stream.FindPlayer("")
	if err != nil {
		t.Skip("no media player installed:", err)
	}
	assert.NotEmpty(t, p.Path)
	cmd := p.Command(context.Background(), stream.URL(0))
	assert.Contains(t, strings.Join(cmd.Args, " "), "udp://@:8554")
	_, lerr := exec.LookPath(p.Name)
	require.NoError(t, lerr)
}

func TestStatsBitrateZero(t *testing.T) {
	t.Parallel()
	assert.Zero(t, stream.Stats{}.Bitrate())
}
