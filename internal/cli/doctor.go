package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/internal/exitcode"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/capability"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/discovery"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

type checkStatus string

const (
	checkPass checkStatus = "pass"
	checkWarn checkStatus = "warn"
	checkFail checkStatus = "fail"
)

type doctorCheck struct {
	Name   string      `json:"name" yaml:"name"`
	Status checkStatus `json:"status" yaml:"status"`
	Detail string      `json:"detail" yaml:"detail"`
}

type doctorResult struct {
	Endpoint string        `json:"endpoint" yaml:"endpoint"`
	Checks   []doctorCheck `json:"checks" yaml:"checks"`
	Failed   int           `json:"failed" yaml:"failed"`
	Warned   int           `json:"warned" yaml:"warned"`
}

func (r doctorResult) Headers() []string { return []string{"CHECK", "STATUS", "DETAIL"} }
func (r doctorResult) Rows() [][]string {
	rows := make([][]string, 0, len(r.Checks))
	for _, c := range r.Checks {
		rows = append(rows, []string{c.Name, string(c.Status), c.Detail})
	}
	return rows
}

func (r doctorResult) Plain() string {
	if r.Failed > 0 {
		return "fail"
	}
	if r.Warned > 0 {
		return "warn"
	}
	return "pass"
}

func (r *doctorResult) add(name string, st checkStatus, format string, args ...any) {
	r.Checks = append(r.Checks, doctorCheck{Name: name, Status: st, Detail: fmt.Sprintf(format, args...)})
	switch st {
	case checkFail:
		r.Failed++
	case checkWarn:
		r.Warned++
	case checkPass:
	}
}

// probe is a read-only operation the doctor exercises for capability drift.
type probe struct {
	op      capability.Op
	path    string
	query   string
	fixture string // fixture file name when --record is used
}

var safeProbes = []probe{
	{capability.OpGPCAMERAOpengoproVersion, "/gopro/version", "", "gopro_version"},
	{capability.OpOGPGetState, "/gopro/camera/state", "", "gopro_camera_state"},
	{capability.OpOGPPresetsGet, "/gopro/camera/presets/get", "", "gopro_camera_presets_get"},
	{capability.OpOGPGetDateAndTimeDst, "/gopro/camera/get_date_time", "", "gopro_camera_get_date_time"},
	{capability.OpOGPMediaList, "/gopro/media/list", "", "gopro_media_list"},
	{capability.OpOGPGetLastMedia, "/gopro/media/last_captured", "", ""},
	{capability.OpOGPCameraInfo, "/gopro/camera/info", "", ""},
	{capability.OpGPCAMERAWebcamStatusOgp, "/gopro/webcam/status", "", "gopro_webcam_status"},
	{capability.OpGPCAMERAWebcamVersionOgp, "/gopro/webcam/version", "", "gopro_webcam_version"},
	{capability.OpGPInfo, "/gp/gpControl/info", "", "gp_gpControl_info"},
	{capability.OpGPStatus, "/gp/gpControl/status", "", "gp_gpControl_status"},
	{capability.OpGPMediaList, "/gp/gpMediaList", "", ""},
	{capability.OpGPWebcamStatus, "/gp/gpWebcam/STATUS", "", ""},
	{capability.OpGPWebcamVersion, "/gp/gpWebcam/VERSION", "", ""},
	{capability.OpDcimIndex, "/videos/DCIM/", "", "videos_DCIM_"},
}

func (a *App) newDoctorCommand() *cobra.Command {
	var (
		record  string
		noProbe bool
	)
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the USB link and check the camera against the capability matrix",
		Long: `Runs connectivity checks (discovery, API version, Host-header guard, alternate
ports, mDNS) and then exercises every read-only operation to detect capability
drift between this firmware and the matrix shipped with gopro9.

Exit code is 69 when any check fails. --record DIR saves the probe responses
as scrubbed fixtures for the fake camera used in tests.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := a.runDoctor(cmd.Context(), record, !noProbe)
			if err != nil {
				return err
			}
			if perr := a.print(cmd, res); perr != nil {
				return perr
			}
			if res.Failed > 0 {
				return &cameraError{err: fmt.Errorf("%d doctor check(s) failed", res.Failed), code: exitcode.Unavailable}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&record, "record", "", "directory to write scrubbed response fixtures into")
	cmd.Flags().BoolVar(&noProbe, "no-probe", false, "skip the capability drift probes")
	return cmd
}

func (a *App) runDoctor(ctx context.Context, record string, doProbe bool) (doctorResult, error) {
	res := doctorResult{}

	// 1. Endpoint
	var ep transport.Endpoint
	if a.opts.Host != "" {
		ep = transport.Endpoint{Host: a.opts.Host, Port: a.opts.Port}
		res.add("endpoint", checkPass, "explicit %s", ep)
	} else {
		dctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		found, err := discovery.Discover(dctx, discovery.Options{Port: a.opts.Port, MDNSTimeout: 1500 * time.Millisecond})
		cancel()
		if err != nil {
			res.add("endpoint", checkFail, "%v", err)
			return res, nil
		}
		c := found[0]
		ep = c.Endpoint()
		res.add("endpoint", checkPass, "%s via %s (%s)", ep, c.Interface, c.Source)
		if c.Firmware != "" {
			res.add("mdns", checkPass, "%s fw=%s serial=%s", strings.TrimSuffix(c.Hostname, "."), c.Firmware, c.Serial)
		} else {
			res.add("mdns", checkWarn, "no _gopro-web._tcp answer (camera found by subnet probe only)")
		}
	}
	res.Endpoint = ep.String()
	t := transport.New(ep, transport.WithTimeout(a.opts.Timeout), transport.WithRetries(0, 0))

	// 2. API version + identity
	type verBody struct {
		Version string `json:"version"`
	}
	v, err := transport.GetJSON[verBody](ctx, t, transport.Request{Path: "/gopro/version"})
	if err != nil {
		res.add("api", checkFail, "%v", err)
		return res, nil
	}
	if v.Version == "2.0" {
		res.add("api", checkPass, "Open GoPro %s", v.Version)
	} else {
		res.add("api", checkWarn, "unexpected Open GoPro version %q (gopro9 targets 2.0)", v.Version)
	}
	type infoBody struct {
		Info struct {
			Model    string `json:"model_name"`
			Firmware string `json:"firmware_version"`
		} `json:"info"`
	}
	info, err := transport.GetJSON[infoBody](ctx, t, transport.Request{Path: "/gp/gpControl/info"})
	switch {
	case err != nil:
		res.add("identity", checkWarn, "gpControl/info failed: %v", err)
	case info.Info.Model != "HERO9 Black":
		res.add("identity", checkWarn, "%s %s: gopro9 is built for the HERO9 Black", info.Info.Model, info.Info.Firmware)
	default:
		known := false
		for _, f := range capability.FirmwareTested {
			if f == info.Info.Firmware {
				known = true
			}
		}
		if known {
			res.add("identity", checkPass, "%s %s (tested firmware)", info.Info.Model, info.Info.Firmware)
		} else {
			res.add("identity", checkWarn, "%s %s: untested firmware, capability matrix may drift", info.Info.Model, info.Info.Firmware)
		}
	}

	// 3. Host header guard
	hgStatus, hgDetail := a.checkHostGuard(ctx, ep)
	res.add("host-guard", hgStatus, "%s", hgDetail)

	// 4. Alternate ports
	for _, p := range []int{80, 8000, 8001} {
		if p == ep.Port {
			continue
		}
		alt := transport.New(transport.Endpoint{Host: ep.Host, Port: p}, transport.WithTimeout(a.opts.Timeout), transport.WithRetries(0, 0))
		if _, err := alt.Get(ctx, transport.Request{Path: "/gopro/version"}); err != nil {
			res.add("port-"+strconv.Itoa(p), checkWarn, "not answering: %v", errors.Unwrap(err))
		} else {
			res.add("port-"+strconv.Itoa(p), checkPass, "same API served")
		}
	}

	// 5. Capability drift
	if doProbe {
		if record != "" {
			if err := os.MkdirAll(record, 0o750); err != nil {
				return res, err
			}
		}
		for _, p := range safeProbes {
			a.runProbe(ctx, t, p, record, &res)
		}
	}
	return res, nil
}

func (a *App) checkHostGuard(ctx context.Context, ep transport.Endpoint) (checkStatus, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.BaseURL()+"/gopro/version", http.NoBody)
	if err != nil {
		return checkWarn, err.Error()
	}
	req.Host = "evil.example"
	client := &http.Client{Timeout: a.opts.Timeout, Transport: &http.Transport{DisableKeepAlives: true, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext}}
	resp, err := client.Do(req)
	if err != nil {
		return checkWarn, err.Error()
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return checkPass, "foreign Host header rejected with 403 (DNS-rebinding guard active)"
	}
	return checkWarn, fmt.Sprintf("foreign Host header accepted (HTTP %d)", resp.StatusCode)
}

func (a *App) runProbe(ctx context.Context, t *transport.Client, p probe, record string, res *doctorResult) {
	expected := capability.Support(p.op)
	resp, err := t.Get(ctx, transport.Request{Path: p.path, Accept: []int{200, 400, 403, 404, 500, 501}})
	name := "probe." + string(p.op)
	if err != nil {
		res.add(name, checkWarn, "transport error: %v", err)
		return
	}
	works := resp.Status == http.StatusOK
	switch {
	case works && expected == capability.Unsupported:
		res.add(name, checkWarn, "DRIFT: matrix says unsupported but camera answered 200")
	case !works && expected != capability.Unsupported:
		res.add(name, checkFail, "DRIFT: matrix says %s but camera answered %d", expected, resp.Status)
	case works:
		res.add(name, checkPass, "200 (%s)", expected)
	default:
		res.add(name, checkPass, "%d as expected (unsupported)", resp.Status)
	}
	if record != "" && works && p.fixture != "" {
		if werr := os.WriteFile(filepath.Join(record, p.fixture+".resp"), scrub(resp.Body), 0o600); werr != nil {
			res.add(name, checkWarn, "record failed: %v", werr)
		}
	}
}

var (
	serialRe = regexp.MustCompile(`C3\d{12}`)
	macRe    = regexp.MustCompile(`"ap_mac"\s*:\s*"[0-9a-fA-F]{12}"`)
	ssidRe   = regexp.MustCompile(`"(ap_ssid|30)"\s*:\s*"[^"]*"`)
)

// scrub removes identifying values from a recorded response before it becomes a fixture.
func scrub(b []byte) []byte {
	s := serialRe.ReplaceAllString(string(b), "C3XXXXXXXXXXXX")
	s = macRe.ReplaceAllString(s, `"ap_mac":"000000000000"`)
	s = ssidRe.ReplaceAllString(s, `"$1":"HERO9 Black"`)
	return []byte(s)
}
