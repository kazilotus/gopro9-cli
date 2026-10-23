// Package stream receives the camera's MPEG-TS over UDP (preview stream or
// webcam output) and hands it to a file, a writer, or a media player.
package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// DefaultPort is the UDP port the camera streams to by default.
const DefaultPort = 8554

// Client starts and stops the low-resolution preview stream.
type Client struct{ t *transport.Client }

// New wraps a transport client.
func New(t *transport.Client) *Client { return &Client{t: t} }

// Start asks the camera to begin the preview stream. port 0 keeps the camera default.
func (c *Client) Start(ctx context.Context, port int) error {
	var params transport.Params
	if port != 0 {
		params = append(params, transport.P("port", strconv.Itoa(port)))
	}
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/camera/stream/start", Params: params})
	return err
}

// Stop ends the preview stream.
func (c *Client) Stop(ctx context.Context) error {
	_, err := c.t.Get(ctx, transport.Request{Path: "/gopro/camera/stream/stop"})
	return err
}

// Stats summarise a receive session.
type Stats struct {
	Packets  int64         `json:"packets" yaml:"packets"`
	Bytes    int64         `json:"bytes" yaml:"bytes"`
	Duration time.Duration `json:"duration" yaml:"duration"`
	// FirstPacket is how long after listening the first datagram arrived (0 if none).
	FirstPacket time.Duration `json:"first_packet" yaml:"first_packet"`
}

// Bitrate returns the average bitrate in bits per second.
func (s Stats) Bitrate() float64 {
	if s.Duration <= 0 {
		return 0
	}
	return float64(s.Bytes*8) / s.Duration.Seconds()
}

// ErrNoData is returned when nothing arrived before the idle timeout.
var ErrNoData = errors.New("stream: no data received (is the stream started and the port reachable?)")

// ReceiveOptions tune Receive.
type ReceiveOptions struct {
	// Port to listen on (default 8554).
	Port int
	// IdleTimeout aborts when no datagram arrives for this long (default 5s).
	IdleTimeout time.Duration
	// MaxDuration stops after this long once data has started (0 = until ctx is done).
	MaxDuration time.Duration
	// OnPacket is invoked per datagram with running stats (may be nil).
	OnPacket func(Stats)
}

// Listen binds the UDP socket so the caller can start the camera stream
// knowing the port is already open. Port 0 picks a free ephemeral port
// (tests); real sessions pass DefaultPort or the user's choice.
func Listen(port int) (*net.UDPConn, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: port})
	if err != nil {
		return nil, fmt.Errorf("stream: listen udp :%d: %w", port, err)
	}
	return conn, nil
}

// Receive copies datagrams from conn to w until ctx is done, MaxDuration
// elapses, or the stream goes idle. The connection is not closed.
func Receive(ctx context.Context, conn *net.UDPConn, w io.Writer, o ReceiveOptions) (Stats, error) {
	if o.IdleTimeout == 0 {
		o.IdleTimeout = 5 * time.Second
	}
	var (
		stats  Stats
		start  = time.Now()
		first  time.Time
		closed atomic.Bool
		buf    = make([]byte, 65536)
	)
	stop := context.AfterFunc(ctx, func() {
		closed.Store(true)
		_ = conn.SetReadDeadline(time.Now())
	})
	defer stop()
	for o.MaxDuration <= 0 || first.IsZero() || time.Since(first) < o.MaxDuration {
		// A steady packet flow never errors, so cancellation must be checked
		// explicitly before each read, not only on read failure.
		if closed.Load() || ctx.Err() != nil {
			break
		}
		if err := conn.SetReadDeadline(time.Now().Add(o.IdleTimeout)); err != nil {
			return stats, err
		}
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if closed.Load() || ctx.Err() != nil {
				break
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				if stats.Packets == 0 {
					stats.Duration = time.Since(start)
					return stats, ErrNoData
				}
				break // stream ended
			}
			return stats, err
		}
		if first.IsZero() {
			first = time.Now()
			stats.FirstPacket = first.Sub(start)
		}
		if _, werr := w.Write(buf[:n]); werr != nil {
			return stats, werr
		}
		stats.Packets++
		stats.Bytes += int64(n)
		stats.Duration = time.Since(first)
		if o.OnPacket != nil {
			o.OnPacket(stats)
		}
	}
	if !first.IsZero() {
		stats.Duration = time.Since(first)
	}
	return stats, nil
}

// Player is an external media player that can open a UDP MPEG-TS URL.
type Player struct {
	Name string
	Path string
	Args func(url string) []string
}

var knownPlayers = []Player{
	{Name: "ffplay", Args: func(u string) []string {
		return []string{"-fflags", "nobuffer", "-flags", "low_delay", "-framedrop", "-window_title", "GoPro HERO9", u}
	}},
	{Name: "mpv", Args: func(u string) []string { return []string{"--profile=low-latency", "--untimed", u} }},
	{Name: "vlc", Args: func(u string) []string { return []string{"--network-caching=100", u} }},
}

// FindPlayer locates a player by name ("" = first available of ffplay, mpv, vlc).
func FindPlayer(name string) (Player, error) {
	for _, p := range knownPlayers {
		if name != "" && p.Name != name {
			continue
		}
		if path, err := exec.LookPath(p.Name); err == nil {
			p.Path = path
			return p, nil
		}
	}
	if name != "" {
		return Player{}, fmt.Errorf("stream: player %q not found on PATH", name)
	}
	return Player{}, errors.New("stream: no player found (install ffplay, mpv or vlc, or use `stream record`)")
}

// URL returns the udp:// URL a player should open for the given port.
func URL(port int) string {
	if port == 0 {
		port = DefaultPort
	}
	return "udp://@:" + strconv.Itoa(port)
}

// Command builds the exec.Cmd for the player (not started).
func (p Player) Command(ctx context.Context, url string) *exec.Cmd {
	return exec.CommandContext(ctx, p.Path, p.Args(url)...) //nolint:gosec // player path comes from exec.LookPath
}
