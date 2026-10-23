package discovery

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

func fakeIfaces() ([]net.Interface, error) {
	return []net.Interface{
		{Index: 1, Name: "lo0", Flags: net.FlagUp | net.FlagLoopback},
		{Index: 2, Name: "en0", Flags: net.FlagUp},
		{Index: 3, Name: "en37", Flags: net.FlagUp},
		{Index: 4, Name: "down0", Flags: 0},
	}, nil
}

func fakeAddrs(i net.Interface) ([]net.Addr, error) {
	switch i.Name {
	case "lo0":
		return []net.Addr{&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}}, nil
	case "en0":
		return []net.Addr{
			&net.IPNet{IP: net.IPv4(192, 168, 1, 20), Mask: net.CIDRMask(24, 32)},
			&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
		}, nil
	case "en37":
		return []net.Addr{&net.IPNet{IP: net.IPv4(172, 24, 135, 52), Mask: net.CIDRMask(24, 32)}}, nil
	}
	return nil, errors.New("no addrs")
}

func TestSubnetCandidates(t *testing.T) {
	t.Parallel()
	o := Options{Interfaces: fakeIfaces, Addrs: fakeAddrs}
	o.defaults()
	c, err := subnetCandidates(&o)
	require.NoError(t, err)
	require.Len(t, c, 2)
	assert.Equal(t, "192.168.1.51", c[0].Address.String())
	assert.Equal(t, "en0", c[0].Interface)
	assert.Equal(t, "172.24.135.51", c[1].Address.String())
	assert.Equal(t, 8080, c[1].Port)
}

func TestDiscoverProbesAllAndMergesMDNS(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	probed := map[string]bool{}
	opts := Options{
		Interfaces: fakeIfaces, Addrs: fakeAddrs, MDNSTimeout: time.Millisecond,
		Probe: func(_ context.Context, ep transport.Endpoint, _ time.Duration) (string, error) {
			mu.Lock()
			probed[ep.Host] = true
			mu.Unlock()
			if ep.Host == "172.24.135.51" {
				return "2.0", nil
			}
			return "", transport.ErrUnreachable
		},
		Browse: func(context.Context, time.Duration) ([]MDNSRecord, error) {
			return []MDNSRecord{{Address: netip.MustParseAddr("172.24.135.51"), Hostname: "C3X.local.", Firmware: "HD9.01.01.72.70", Model: "0x37", Serial: "C3X"}}, nil
		},
	}
	found, err := Discover(context.Background(), opts)
	require.NoError(t, err)
	require.Len(t, found, 1)
	mu.Lock()
	assert.True(t, probed["192.168.1.51"])
	mu.Unlock()
	assert.Equal(t, "2.0", found[0].APIVersion)
	assert.Equal(t, "HD9.01.01.72.70", found[0].Firmware)
	assert.Equal(t, "C3X", found[0].Serial)
	assert.Equal(t, SourceSubnet, found[0].Source)
	assert.Equal(t, transport.Endpoint{Host: "172.24.135.51", Port: 8080}, found[0].Endpoint())
}

func TestDiscoverNotFound(t *testing.T) {
	t.Parallel()
	opts := Options{
		Interfaces: fakeIfaces, Addrs: fakeAddrs,
		Probe: func(context.Context, transport.Endpoint, time.Duration) (string, error) {
			return "", transport.ErrUnreachable
		},
	}
	_, err := Discover(context.Background(), opts)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestDiscoverInterfaceError(t *testing.T) {
	t.Parallel()
	_, err := Discover(context.Background(), Options{Interfaces: func() ([]net.Interface, error) { return nil, errors.New("boom") }})
	require.Error(t, err)
}

func TestVerify(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, ln.Close())
	port := ln.Addr().(*net.TCPAddr).Port
	_, err = Verify(context.Background(), "127.0.0.1", port, 200*time.Millisecond)
	require.Error(t, err)
}

func buildResponse(t *testing.T) []byte {
	t.Helper()
	mk := func(s string) dnsmessage.Name {
		n, err := dnsmessage.NewName(s)
		require.NoError(t, err)
		return n
	}
	svc := mk(ServiceName)
	inst := mk("C3X._gopro-web._tcp.local.")
	host := mk("C3X.local.")
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{Response: true, Authoritative: true})
	require.NoError(t, b.StartAnswers())
	require.NoError(t, b.PTRResource(dnsmessage.ResourceHeader{Name: svc, Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET}, dnsmessage.PTRResource{PTR: inst}))
	require.NoError(t, b.StartAdditionals())
	require.NoError(t, b.SRVResource(dnsmessage.ResourceHeader{Name: inst, Type: dnsmessage.TypeSRV, Class: dnsmessage.ClassINET}, dnsmessage.SRVResource{Port: 80, Target: host}))
	require.NoError(t, b.TXTResource(dnsmessage.ResourceHeader{Name: inst, Type: dnsmessage.TypeTXT, Class: dnsmessage.ClassINET}, dnsmessage.TXTResource{TXT: []string{"model=0x37", "fw_version=HD9.01.01.72.70", "platform_version=86eaae4", "protocol_version=1"}}))
	require.NoError(t, b.AResource(dnsmessage.ResourceHeader{Name: host, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET}, dnsmessage.AResource{A: [4]byte{172, 24, 135, 51}}))
	pkt, err := b.Finish()
	require.NoError(t, err)
	return pkt
}

func TestParseMDNS(t *testing.T) {
	t.Parallel()
	recs := ParseMDNS([][]byte{buildResponse(t), []byte("garbage")})
	require.Len(t, recs, 1)
	r := recs[0]
	assert.Equal(t, "C3X", r.Instance)
	assert.Equal(t, "C3X", r.Serial)
	assert.Equal(t, "C3X.local.", r.Hostname)
	assert.Equal(t, 80, r.Port)
	assert.Equal(t, "172.24.135.51", r.Address.String())
	assert.Equal(t, "HD9.01.01.72.70", r.Firmware)
	assert.Equal(t, "0x37", r.Model)
	assert.Equal(t, "1", r.TXT["protocol_version"])
	assert.Equal(t, "C3X", instanceOf("C3X._gopro-web._tcp.local."))
	assert.Equal(t, "plain", instanceOf("plain."))
}

func TestBuildQuery(t *testing.T) {
	t.Parallel()
	q, err := buildQuery()
	require.NoError(t, err)
	var p dnsmessage.Parser
	_, err = p.Start(q)
	require.NoError(t, err)
	qs, err := p.AllQuestions()
	require.NoError(t, err)
	require.Len(t, qs, 1)
	assert.Equal(t, ServiceName, qs[0].Name.String())
	assert.Equal(t, dnsmessage.TypePTR, qs[0].Type)
	assert.Equal(t, dnsmessage.Class(0x8001), qs[0].Class, "QU bit set")
}

func TestBrowseDoesNotPanic(t *testing.T) {
	// Real network; just make sure it returns within the timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := Browse(ctx, 300*time.Millisecond)
	assert.NoError(t, err)
}
