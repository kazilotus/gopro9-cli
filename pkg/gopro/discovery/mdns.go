package discovery

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// ServiceName is the mDNS service the camera advertises.
const ServiceName = "_gopro-web._tcp.local."

// MDNSRecord is a resolved _gopro-web._tcp instance.
type MDNSRecord struct {
	Instance string            `json:"instance" yaml:"instance"` // usually the serial number
	Hostname string            `json:"hostname" yaml:"hostname"` // e.g. C3XXXXXXXXXXXX.local.
	Port     int               `json:"port" yaml:"port"`
	Address  netip.Addr        `json:"address" yaml:"address"`
	TXT      map[string]string `json:"txt" yaml:"txt"`
	Model    string            `json:"model,omitempty" yaml:"model,omitempty"`
	Firmware string            `json:"firmware,omitempty" yaml:"firmware,omitempty"`
	Serial   string            `json:"serial,omitempty" yaml:"serial,omitempty"`
}

var mdnsGroup = netip.MustParseAddrPort("224.0.0.251:5353")

// Browse sends an mDNS PTR query for the GoPro web service on every
// multicast-capable IPv4 interface and collects answers for timeout.
// It requests unicast responses (QU bit), so it does not need to bind port 5353.
func Browse(ctx context.Context, timeout time.Duration) ([]MDNSRecord, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	query, err := buildQuery()
	if err != nil {
		return nil, err
	}

	var (
		mu      sync.Mutex
		packets [][]byte
		wg      sync.WaitGroup
	)
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagMulticast == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, aerr := ifc.Addrs()
		if aerr != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil {
				continue
			}
			conn, lerr := net.ListenUDP("udp4", &net.UDPAddr{IP: ipn.IP})
			if lerr != nil {
				continue
			}
			wg.Add(1)
			go func(conn *net.UDPConn) {
				defer wg.Done()
				defer func() { _ = conn.Close() }()
				_, _ = conn.WriteToUDPAddrPort(query, mdnsGroup)
				deadline, _ := ctx.Deadline()
				_ = conn.SetReadDeadline(deadline)
				buf := make([]byte, 9000)
				for {
					n, _, rerr := conn.ReadFromUDPAddrPort(buf)
					if rerr != nil {
						return
					}
					pkt := make([]byte, n)
					copy(pkt, buf[:n])
					mu.Lock()
					packets = append(packets, pkt)
					mu.Unlock()
				}
			}(conn)
		}
	}
	wg.Wait()
	return ParseMDNS(packets), nil
}

func buildQuery() ([]byte, error) {
	name, err := dnsmessage.NewName(ServiceName)
	if err != nil {
		return nil, err
	}
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{})
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	// Class IN with the top bit set requests a unicast response (RFC 6762 §5.4).
	if err := b.Question(dnsmessage.Question{Name: name, Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET | 0x8000}); err != nil {
		return nil, err
	}
	return b.Finish()
}

type partialRecord struct {
	rec MDNSRecord
}

type collector struct {
	byInstance map[string]*partialRecord
	hostAddrs  map[string]netip.Addr
	hostTarget map[string]string // instance -> host
}

func newCollector() *collector {
	return &collector{byInstance: map[string]*partialRecord{}, hostAddrs: map[string]netip.Addr{}, hostTarget: map[string]string{}}
}

// ParseMDNS assembles service records from raw mDNS response packets.
// It is exported so recorded packets can be replayed in tests.
func ParseMDNS(packets [][]byte) []MDNSRecord {
	c := newCollector()
	for _, pkt := range packets {
		var p dnsmessage.Parser
		if _, err := p.Start(pkt); err != nil {
			continue
		}
		if err := p.SkipAllQuestions(); err != nil {
			continue
		}
		for _, section := range []func() ([]dnsmessage.Resource, error){p.AllAnswers, p.AllAuthorities, p.AllAdditionals} {
			rrs, err := section()
			if err != nil {
				break
			}
			for i := range rrs {
				c.consume(&rrs[i])
			}
		}
	}
	out := make([]MDNSRecord, 0, len(c.byInstance))
	for inst, pr := range c.byInstance {
		host := c.hostTarget[inst]
		if addr, ok := c.hostAddrs[host]; ok {
			pr.rec.Address = addr
		}
		pr.rec.Hostname = host
		pr.rec.Model = pr.rec.TXT["model"]
		pr.rec.Firmware = pr.rec.TXT["fw_version"]
		pr.rec.Serial = inst
		out = append(out, pr.rec)
	}
	return out
}

func (c *collector) get(inst string) *partialRecord {
	if p, ok := c.byInstance[inst]; ok {
		return p
	}
	p := &partialRecord{rec: MDNSRecord{Instance: inst, TXT: map[string]string{}}}
	c.byInstance[inst] = p
	return p
}

func (c *collector) consume(rr *dnsmessage.Resource) {
	name := rr.Header.Name.String()
	inService := strings.HasSuffix(strings.ToLower(name), "."+ServiceName)
	switch body := rr.Body.(type) {
	case *dnsmessage.PTRResource:
		if strings.EqualFold(name, ServiceName) {
			c.get(instanceOf(body.PTR.String()))
		}
	case *dnsmessage.SRVResource:
		if inService {
			inst := instanceOf(name)
			c.get(inst).rec.Port = int(body.Port)
			c.hostTarget[inst] = body.Target.String()
		}
	case *dnsmessage.TXTResource:
		if inService {
			p := c.get(instanceOf(name))
			for _, kv := range body.TXT {
				k, v, _ := strings.Cut(kv, "=")
				p.rec.TXT[k] = v
			}
		}
	case *dnsmessage.AResource:
		c.hostAddrs[name] = netip.AddrFrom4(body.A)
	}
}

// instanceOf extracts "C3441326546435" from "C3441326546435._gopro-web._tcp.local.".
func instanceOf(fqdn string) string {
	i := strings.Index(strings.ToLower(fqdn), "."+ServiceName)
	if i < 0 {
		return strings.TrimSuffix(fqdn, ".")
	}
	return fqdn[:i]
}
