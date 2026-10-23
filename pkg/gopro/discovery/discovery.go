// Package discovery finds a HERO9 on the host's network interfaces.
//
// Over USB the camera is a CDC-ECM Ethernet adapter that runs a DHCP server
// and always takes host .51 of its /24. Discovery therefore probes
// <subnet>.51 on every candidate interface in parallel, and additionally
// browses mDNS for _gopro-web._tcp, which carries the firmware version.
package discovery

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sort"
	"sync"
	"time"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// Source says how a candidate was found.
type Source string

// Discovery sources.
const (
	SourceSubnet   Source = "subnet"
	SourceMDNS     Source = "mdns"
	SourceExplicit Source = "explicit"
)

// Candidate is a camera endpoint that answered.
type Candidate struct {
	Address    netip.Addr `json:"address" yaml:"address"`
	Port       int        `json:"port" yaml:"port"`
	Interface  string     `json:"interface,omitempty" yaml:"interface,omitempty"`
	Source     Source     `json:"source" yaml:"source"`
	APIVersion string     `json:"api_version,omitempty" yaml:"api_version,omitempty"`
	Hostname   string     `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	Firmware   string     `json:"firmware,omitempty" yaml:"firmware,omitempty"`
	Model      string     `json:"model,omitempty" yaml:"model,omitempty"`
	Serial     string     `json:"serial,omitempty" yaml:"serial,omitempty"`
}

// Endpoint converts the candidate into a transport endpoint.
func (c Candidate) Endpoint() transport.Endpoint {
	return transport.Endpoint{Host: c.Address.String(), Port: c.Port}
}

// ErrNotFound is returned when no camera answered.
var ErrNotFound = errors.New("discovery: no camera found on any interface (is the camera on and set to GoPro Connect?)")

// Options tune discovery.
type Options struct {
	Port         int           // HTTP port to probe (default 8080)
	ProbeTimeout time.Duration // per-candidate HTTP timeout (default 1500ms)
	MDNSTimeout  time.Duration // how long to wait for mDNS answers (default 1500ms); 0 disables mDNS
	CameraHost   uint8         // host octet the camera takes on its /24 (default 51)
	Interfaces   func() ([]net.Interface, error)
	Addrs        func(net.Interface) ([]net.Addr, error)
	Probe        func(ctx context.Context, ep transport.Endpoint, timeout time.Duration) (string, error)
	Browse       func(ctx context.Context, timeout time.Duration) ([]MDNSRecord, error)
}

func (o *Options) defaults() {
	if o.Port == 0 {
		o.Port = 8080
	}
	if o.ProbeTimeout == 0 {
		o.ProbeTimeout = 1500 * time.Millisecond
	}
	if o.CameraHost == 0 {
		o.CameraHost = 51
	}
	if o.Interfaces == nil {
		o.Interfaces = net.Interfaces
	}
	if o.Addrs == nil {
		o.Addrs = func(i net.Interface) ([]net.Addr, error) { return i.Addrs() }
	}
	if o.Probe == nil {
		o.Probe = probeVersion
	}
	if o.Browse == nil {
		o.Browse = Browse
	}
}

// subnetCandidates computes the .51 address on every eligible IPv4 interface.
func subnetCandidates(o *Options) ([]Candidate, error) {
	ifaces, err := o.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []Candidate
	seen := map[netip.Addr]bool{}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := o.Addrs(ifc)
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipn.IP.To4()
			if ip4 == nil {
				continue
			}
			ones, _ := ipn.Mask.Size()
			if ones < 16 || ones > 30 {
				continue
			}
			cam := netip.AddrFrom4([4]byte{ip4[0], ip4[1], ip4[2], o.CameraHost})
			self := netip.AddrFrom4([4]byte{ip4[0], ip4[1], ip4[2], ip4[3]})
			if cam == self || seen[cam] {
				continue
			}
			seen[cam] = true
			out = append(out, Candidate{Address: cam, Port: o.Port, Interface: ifc.Name, Source: SourceSubnet})
		}
	}
	return out, nil
}

// Discover probes every interface and returns the cameras that answered,
// enriched with mDNS metadata when available. Explicit hosts are not probed here.
func Discover(ctx context.Context, opts Options) ([]Candidate, error) {
	opts.defaults()
	cands, err := subnetCandidates(&opts)
	if err != nil {
		return nil, err
	}

	var (
		mu    sync.Mutex
		found []Candidate
		wg    sync.WaitGroup
	)
	for i := range cands {
		wg.Add(1)
		go func(c Candidate) {
			defer wg.Done()
			ver, perr := opts.Probe(ctx, c.Endpoint(), opts.ProbeTimeout)
			if perr != nil {
				return
			}
			c.APIVersion = ver
			mu.Lock()
			found = append(found, c)
			mu.Unlock()
		}(cands[i])
	}

	var records []MDNSRecord
	if opts.MDNSTimeout > 0 {
		records, _ = opts.Browse(ctx, opts.MDNSTimeout) // best effort
	}
	wg.Wait()

	for i := range found {
		for _, r := range records {
			if r.Address == found[i].Address {
				found[i].Hostname, found[i].Firmware, found[i].Model, found[i].Serial = r.Hostname, r.Firmware, r.Model, r.Serial
			}
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Address.Less(found[j].Address) })
	if len(found) == 0 {
		return nil, ErrNotFound
	}
	return found, nil
}

// Verify checks that an explicit host answers the Open GoPro version query.
func Verify(ctx context.Context, host string, port int, timeout time.Duration) (Candidate, error) {
	ep := transport.Endpoint{Host: host, Port: port}
	ver, err := probeVersion(ctx, ep, timeout)
	if err != nil {
		return Candidate{}, err
	}
	c := Candidate{Port: port, Source: SourceExplicit, APIVersion: ver}
	if a, perr := netip.ParseAddr(host); perr == nil {
		c.Address = a
	} else {
		c.Hostname = host
	}
	return c, nil
}

type versionBody struct {
	Version string `json:"version"`
}

func probeVersion(ctx context.Context, ep transport.Endpoint, timeout time.Duration) (string, error) {
	t := transport.New(ep, transport.WithTimeout(timeout), transport.WithRetries(0, 0))
	v, err := transport.GetJSON[versionBody](ctx, t, transport.Request{Path: "/gopro/version"})
	if err != nil {
		return "", err
	}
	return v.Version, nil
}
