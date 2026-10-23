// Package fakecam is an in-process HERO9 Black emulator for tests.
//
// It serves the recorded fixtures under testdata/fixtures and models the
// small amount of state the CLI depends on: recording, settings, presets,
// zoom, turbo, webcam status, media deletion, keep-alives, and the Host
// header guard. It is the contract boundary for integration tests, so every
// Verified or Spec operation in the capability matrix must have a handler.
package fakecam

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/settings"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// FixtureDir is the recorded-response directory used by default.
const FixtureDir = "hero9_HD9.01.01.72.70"

// Server is a fake camera.
type Server struct {
	*httptest.Server
	mu sync.Mutex

	fixtures map[string][]byte
	now      func() time.Time

	// Observable state (read under Lock or via Snapshot).
	Recording     bool
	Sleeping      bool
	Locate        bool
	Turbo         bool
	Zoom          int
	KeepAlives    int
	WebcamStatus  int // 0 off, 1 idle, 2 high-power preview, 3 low-power preview
	Streaming     bool
	PresetLoaded  uint32
	PresetGroup   uint32
	Settings      map[uint16]int64
	NotApplicable map[uint16]bool // settings the camera rejects in the current mode
	Status        map[uint16]json.RawMessage
	Clock         time.Time
	Media         []MediaFile
	Hilights      map[string][]int64
	Requests      []string

	// Behaviour toggles.
	HostCheck bool
	FailWith  map[string]int // path -> status code to force

	// StreamPort is where UDP MPEG-TS packets are sent while streaming
	// (default 8554, like the camera). StreamPackets is how many to emit per
	// start (default 50, ~20 ms apart).
	StreamPort    int
	StreamPackets int
}

// MediaFile is one file on the fake SD card.
type MediaFile struct {
	Dir    string
	Name   string
	Size   int64
	Mod    int64
	LRV    bool
	Hidden bool // companions (.LRV/.THM) are served by the DCIM tree but absent from the media list
	Body   []byte
}

// Option configures the server.
type Option func(*Server)

// WithClock fixes the camera clock.
func WithClock(now func() time.Time) Option { return func(s *Server) { s.now = now } }

// WithHostCheck toggles the Host header guard (default on).
func WithHostCheck(on bool) Option { return func(s *Server) { s.HostCheck = on } }

// New starts a fake camera and registers cleanup with t.
func New(tb testing.TB, opts ...Option) *Server {
	tb.Helper()
	s, err := Start(opts...)
	if err != nil {
		tb.Fatalf("fakecam: %v", err)
	}
	tb.Cleanup(s.Close)
	return s
}

// Start launches a fake camera without a testing.TB (for examples and manual use).
func Start(opts ...Option) (*Server, error) {
	s := &Server{
		fixtures:      map[string][]byte{},
		now:           time.Now,
		HostCheck:     true,
		WebcamStatus:  1,
		Hilights:      map[string][]int64{},
		FailWith:      map[string]int{},
		NotApplicable: map[uint16]bool{},
		StreamPort:    8554,
		StreamPackets: 50,
	}
	for _, o := range opts {
		o(s)
	}
	if err := s.loadFixtures(); err != nil {
		return nil, err
	}
	if err := s.loadState(); err != nil {
		return nil, err
	}
	s.loadMedia()
	s.Server = httptest.NewServer(s.handler())
	return s, nil
}

// Endpoint returns the transport endpoint of the fake camera.
func (s *Server) Endpoint() transport.Endpoint {
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(s.URL, "http://"))
	p, _ := strconv.Atoi(port)
	return transport.Endpoint{Host: host, Port: p}
}

func fixturePath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "fixtures", FixtureDir)
}

func (s *Server) loadFixtures() error {
	dir := fixturePath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read fixtures %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".resp") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		s.fixtures[strings.TrimSuffix(e.Name(), ".resp")] = b
	}
	if len(s.fixtures) == 0 {
		return fmt.Errorf("no fixtures in %s", dir)
	}
	return nil
}

func (s *Server) loadState() error {
	var st struct {
		Status   map[string]json.RawMessage `json:"status"`
		Settings map[string]int64           `json:"settings"`
	}
	if err := json.Unmarshal(s.fixtures["gopro_camera_state"], &st); err != nil {
		return fmt.Errorf("state fixture: %w", err)
	}
	s.Status = map[uint16]json.RawMessage{}
	for k, v := range st.Status {
		id, _ := strconv.ParseUint(k, 10, 16)
		s.Status[uint16(id)] = v
	}
	s.Settings = map[uint16]int64{}
	for k, v := range st.Settings {
		id, _ := strconv.ParseUint(k, 10, 16)
		s.Settings[uint16(id)] = v
	}
	return nil
}

func (s *Server) loadMedia() {
	var ml struct {
		Media []struct {
			D  string `json:"d"`
			Fs []struct {
				N   string `json:"n"`
				Mod string `json:"mod"`
				S   string `json:"s"`
				Ls  string `json:"ls"`
			} `json:"fs"`
		} `json:"media"`
	}
	_ = json.Unmarshal(s.fixtures["gopro_media_list"], &ml)
	for _, d := range ml.Media {
		for _, f := range d.Fs {
			mod, _ := strconv.ParseInt(f.Mod, 10, 64)
			// Synthetic bodies: deterministic size 4–64 KiB derived from the name,
			// so Range downloads can be verified byte-for-byte.
			h := fnv.New32a()
			_, _ = h.Write([]byte(f.N))
			size := int64(4096 + h.Sum32()%61440)
			body := make([]byte, size)
			for i := range body {
				body[i] = byte((int(h.Sum32()) + i) % 251)
			}
			s.Media = append(s.Media, MediaFile{Dir: d.D, Name: f.N, Size: size, Mod: mod, LRV: f.Ls != "", Body: body})
			// Companion files exist on the card like on the real camera.
			if len(f.N) >= 8 {
				stem := f.N[2:8]
				thm := make([]byte, 2048)
				copy(thm, []byte{0xFF, 0xD8, 0xFF, 0xE0})
				s.Media = append(s.Media, MediaFile{Dir: d.D, Name: f.N[:len(f.N)-4] + ".THM", Size: int64(len(thm)), Mod: mod, Hidden: true, Body: thm})
				if f.Ls != "" {
					lrv := make([]byte, 8192)
					s.Media = append(s.Media, MediaFile{Dir: d.D, Name: "GL" + stem + ".LRV", Size: int64(len(lrv)), Mod: mod, Hidden: true, Body: lrv})
				}
			}
		}
	}
}

// Visible returns the media files the camera lists (excluding companions).
func (s *Server) Visible() []MediaFile {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []MediaFile
	for _, m := range s.Media {
		if !m.Hidden {
			out = append(out, m)
		}
	}
	return out
}

// Snapshot returns a copy of the request log.
func (s *Server) Snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.Requests...)
}

// FindMedia returns the fake file for "100GOPRO/GH010397.MP4".
func (s *Server) FindMedia(path string) (MediaFile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, name, _ := strings.Cut(path, "/")
	for _, m := range s.Media {
		if m.Dir == dir && m.Name == name {
			return m, true
		}
	}
	return MediaFile{}, false
}

// --- HTTP -----------------------------------------------------------------

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	reg := func(path string, h func(http.ResponseWriter, *http.Request)) { mux.HandleFunc(path, h) }

	reg("/gopro/version", s.fixture("gopro_version"))
	reg("/gopro/camera/state", s.state)
	reg("/gp/gpControl/status", s.state)
	reg("/gp/gpControl/info", s.fixture("gp_gpControl_info"))
	reg("/gopro/camera/keep_alive", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		s.KeepAlives++
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, "{}")
	})
	reg("/gopro/camera/analytics/set_client_info", ok)
	reg("/gopro/camera/get_date_time", s.getDateTime)
	reg("/gopro/camera/set_date_time", s.setDateTime)
	reg("/gopro/camera/shutter/start", s.shutter(true))
	reg("/gopro/camera/shutter/stop", s.shutter(false))
	reg("/gopro/camera/setting", s.setting)
	reg("/gopro/camera/presets/get", s.fixture("gopro_camera_presets_get"))
	reg("/gopro/camera/presets/load", s.presetLoad)
	reg("/gopro/camera/presets/set_group", s.presetGroup)
	reg("/gopro/camera/digital_zoom", s.zoom)
	reg("/gopro/media/turbo_transfer", s.turbo)
	reg("/gp/gpControl/command/system/sleep", func(w http.ResponseWriter, _ *http.Request) {
		s.set(func() { s.Sleeping = true })
		writeJSON(w, http.StatusOK, "{}")
	})
	reg("/gp/gpControl/command/system/locate", func(w http.ResponseWriter, r *http.Request) {
		s.set(func() { s.Locate = r.URL.Query().Get("p") == "1" })
		writeJSON(w, http.StatusOK, "{}")
	})
	reg("/gopro/media/list", s.mediaList)
	reg("/gp/gpMediaList", s.mediaList)
	reg("/gopro/media/thumbnail", s.mediaImage)
	reg("/gopro/media/screennail", s.mediaImage)
	reg("/gopro/media/gpmf", s.mediaBlob(2048))
	reg("/gopro/media/telemetry", s.mediaBlob(16384))
	reg("/gopro/media/hilight/file", s.hilight(true))
	reg("/gopro/media/hilight/remove", s.hilight(false))
	reg("/gopro/media/delete/file", s.deleteFile)
	reg("/gp/gpControl/command/storage/delete/all", func(w http.ResponseWriter, _ *http.Request) {
		s.set(func() { s.Media = nil })
		writeJSON(w, http.StatusOK, "{}")
	})
	reg("/gp/gpControl/command/storage/delete/group", s.deleteGroup)
	reg("/videos/DCIM/", s.dcim)
	reg("/gopro/webcam/status", s.webcamStatus)
	reg("/gp/gpWebcam/STATUS", s.webcamStatus)
	reg("/gp/gpWebcam/VERSION", s.fixture("gopro_webcam_version"))
	reg("/gopro/webcam/version", s.fixture("gopro_webcam_version"))
	reg("/gopro/webcam/start", s.webcam(2))
	reg("/gopro/webcam/preview", s.webcam(3))
	reg("/gopro/webcam/stop", s.webcam(1))
	reg("/gopro/webcam/exit", s.webcam(0))
	reg("/gopro/camera/stream/start", func(w http.ResponseWriter, r *http.Request) {
		s.set(func() { s.Streaming = true })
		s.emitTS(r)
		writeJSON(w, http.StatusOK, "{}")
	})
	reg("/gopro/camera/stream/stop", func(w http.ResponseWriter, _ *http.Request) {
		s.set(func() { s.Streaming = false })
		writeJSON(w, http.StatusOK, "{}")
	})
	reg("/gp/gpSoftUpdate", s.fixture("gp_gpSoftUpdate"))
	reg("/", s.notFound)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.Requests = append(s.Requests, r.Method+" "+r.URL.RequestURI())
		forced, hasForced := s.FailWith[r.URL.Path]
		s.mu.Unlock()
		w.Header().Set("Connection", "close")
		if s.HostCheck && !hostAllowed(r.Host, s.Endpoint()) {
			http.Error(w, "Error 403: Forbidden\nConnection forbidden", http.StatusForbidden)
			return
		}
		if hasForced {
			writeJSON(w, forced, "{}")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func hostAllowed(host string, ep transport.Endpoint) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	return h == ep.Host || h == "10.5.5.9" || h == "localhost" || h == "127.0.0.1"
}

func (s *Server) set(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f()
}

func ok(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, "{}") }

func writeJSON(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, body)
}

func (s *Server) fixture(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		b, ok := s.fixtures[name]
		if !ok {
			writeJSON(w, http.StatusNotFound, "{}")
			return
		}
		ct := "application/json"
		if strings.HasPrefix(strings.TrimSpace(string(b)), "<") {
			ct = "text/html; charset=utf-8"
		}
		w.Header().Set("Content-Type", ct)
		_, _ = w.Write(b)
	}
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/gopro/") {
		writeJSON(w, http.StatusNotFound, "{}")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = io.WriteString(w, "Error 404: Not Found\nNot found")
}

func (s *Server) state(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := map[string]json.RawMessage{}
	for k, v := range s.Status {
		status[strconv.Itoa(int(k))] = v
	}
	enc := "0"
	if s.Recording {
		enc = "1"
	}
	status["8"] = json.RawMessage(enc)  // Busy
	status["10"] = json.RawMessage(enc) // Encoding
	settingsOut := map[string]int64{}
	for k, v := range s.Settings {
		settingsOut[strconv.Itoa(int(k))] = v
	}
	b, _ := json.Marshal(map[string]any{"status": status, "settings": settingsOut})
	writeJSON(w, http.StatusOK, string(b))
}

func (s *Server) getDateTime(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	t := s.Clock
	s.mu.Unlock()
	if t.IsZero() {
		t = s.now()
	}
	writeJSON(w, http.StatusOK, fmt.Sprintf(`{"date":%q,"time":%q}`, t.Format("2006_01_02"), t.Format("15_04_05")))
}

func (s *Server) setDateTime(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	t, err := time.Parse("2006_01_02 15_04_05", q.Get("date")+" "+q.Get("time"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "{}")
		return
	}
	s.set(func() { s.Clock = t })
	writeJSON(w, http.StatusOK, "{}")
}

func (s *Server) shutter(start bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.Recording == start {
			writeJSON(w, http.StatusInternalServerError, "{}") // already in requested state
			return
		}
		s.Recording = start
		writeJSON(w, http.StatusOK, "{}")
	}
}

func (s *Server) setting(w http.ResponseWriter, r *http.Request) {
	// The real camera parses positionally: "option" before "setting" is a 400.
	if strings.HasPrefix(r.URL.RawQuery, "option=") {
		writeJSON(w, http.StatusBadRequest, "{}")
		return
	}
	q := r.URL.Query()
	id, err := strconv.ParseUint(q.Get("setting"), 10, 16)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "{}")
		return
	}
	d, known := settings.ByID(settings.SettingID(id))
	if !known {
		writeJSON(w, http.StatusForbidden, "{}")
		return
	}
	opt, err := strconv.ParseUint(q.Get("option"), 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "{}")
		return
	}
	s.mu.Lock()
	notApplicable := s.NotApplicable[uint16(id)]
	streaming := s.WebcamStatus == 2 || s.WebcamStatus == 3
	s.mu.Unlock()
	if streaming {
		// The real camera answers 500 to any setting change while the webcam streams.
		writeJSON(w, http.StatusInternalServerError, "")
		return
	}
	reject := func(available []uint32) {
		if available == nil {
			available = []uint32{}
		}
		b, _ := json.Marshal(map[string]any{"error": 1, "option_id": opt, "setting_id": id, "available_options": available})
		writeJSON(w, http.StatusForbidden, string(b))
	}
	if notApplicable {
		reject(nil)
		return
	}
	s.mu.Lock()
	current := s.Settings[uint16(id)]
	s.mu.Unlock()
	// Like the real camera, re-asserting the current value always succeeds,
	// even when the spec does not list that value for the HERO9.
	if o, valid := d.FromValue(uint32(opt)); int64(opt) != current && (!valid || !o.SupportedOnHERO9()) {
		var avail []uint32
		for _, oi := range d.Options {
			if oi.HERO9 {
				avail = append(avail, oi.Value)
			}
		}
		reject(avail)
		return
	}
	s.set(func() { s.Settings[uint16(id)] = int64(opt) })
	writeJSON(w, http.StatusOK, "{}")
}

func (s *Server) presetLoad(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.URL.Query().Get("id"), 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, "{}")
		return
	}
	if !strings.Contains(string(s.fixtures["gopro_camera_presets_get"]), fmt.Sprintf(`"id":%d`, id)) &&
		!strings.Contains(string(s.fixtures["gopro_camera_presets_get"]), fmt.Sprintf(`"id": %d`, id)) {
		writeJSON(w, http.StatusForbidden, "{}")
		return
	}
	s.set(func() { s.PresetLoaded = uint32(id) })
	writeJSON(w, http.StatusOK, "{}")
}

func (s *Server) presetGroup(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.URL.Query().Get("id"), 10, 32)
	if err != nil || id < 1000 || id > 1002 {
		writeJSON(w, http.StatusForbidden, "{}")
		return
	}
	s.set(func() { s.PresetGroup = uint32(id) })
	writeJSON(w, http.StatusOK, "{}")
}

func (s *Server) zoom(w http.ResponseWriter, r *http.Request) {
	p, err := strconv.Atoi(r.URL.Query().Get("percent"))
	if err != nil || p < 0 || p > 100 {
		writeJSON(w, http.StatusBadRequest, "{}")
		return
	}
	s.set(func() { s.Zoom = p })
	writeJSON(w, http.StatusOK, "{}")
}

func (s *Server) turbo(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("p")
	if p != "0" && p != "1" {
		writeJSON(w, http.StatusBadRequest, "{}")
		return
	}
	s.set(func() { s.Turbo = p == "1" })
	writeJSON(w, http.StatusOK, "{}")
}

func (s *Server) mediaList(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type item map[string]string
	dirs := map[string][]item{}
	order := []string{}
	for _, m := range s.Media {
		if m.Hidden {
			continue
		}
		if _, ok := dirs[m.Dir]; !ok {
			order = append(order, m.Dir)
		}
		it := item{"n": m.Name, "cre": strconv.FormatInt(m.Mod, 10), "mod": strconv.FormatInt(m.Mod, 10), "s": strconv.FormatInt(m.Size, 10)}
		if m.LRV {
			it["glrv"] = "1000"
			it["ls"] = "-1"
		}
		dirs[m.Dir] = append(dirs[m.Dir], it)
	}
	media := make([]map[string]any, 0, len(order))
	for _, d := range order {
		media = append(media, map[string]any{"d": d, "fs": dirs[d]})
	}
	b, _ := json.Marshal(map[string]any{"id": "1902797806607713128", "media": media})
	writeJSON(w, http.StatusOK, string(b))
}

func (s *Server) mediaImage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.FindMedia(r.URL.Query().Get("path")); !ok {
		writeJSON(w, http.StatusNotFound, "{}")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("fake-jpeg")...))
}

func (s *Server) mediaBlob(size int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, ok := s.FindMedia(r.URL.Query().Get("path"))
		if !ok || !strings.HasSuffix(strings.ToUpper(m.Name), ".MP4") {
			writeJSON(w, http.StatusNotFound, "{}")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(gpmfBlob(size))
	}
}

// gpmfBlob builds a tiny but structurally valid GPMF stream: DEVC { DVID, DVNM }.
func gpmfBlob(pad int) []byte {
	var b []byte
	put := func(key string, typ byte, size, repeat int, payload []byte) {
		b = append(b, []byte(key)...)
		b = append(b, typ, byte(size), byte(repeat>>8), byte(repeat))
		b = append(b, payload...)
		for len(payload)%4 != 0 {
			b = append(b, 0)
			payload = append(payload, 0)
		}
	}
	inner := make([]byte, 0, 32)
	inner = append(inner, []byte("DVID")...)
	inner = append(inner, 'L', 4, 0, 1, 0, 0, 0, 1)
	name := []byte("Fake HERO9  ")
	inner = append(inner, []byte("DVNM")...)
	inner = append(inner, 'c', 1, 0, byte(len(name)))
	inner = append(inner, name...)
	put("DEVC", 0, 1, len(inner), inner)
	for len(b) < pad {
		b = append(b, 0)
	}
	return b
}

func (s *Server) hilight(add bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if _, ok := s.FindMedia(path); !ok {
			writeJSON(w, http.StatusNotFound, "{}")
			return
		}
		ms, _ := strconv.ParseInt(r.URL.Query().Get("ms"), 10, 64)
		s.set(func() {
			if add {
				s.Hilights[path] = append(s.Hilights[path], ms)
				return
			}
			var keep []int64
			for _, h := range s.Hilights[path] {
				if h != ms {
					keep = append(keep, h)
				}
			}
			s.Hilights[path] = keep
		})
		writeJSON(w, http.StatusOK, "{}")
	}
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if _, ok := s.FindMedia(path); !ok {
		writeJSON(w, http.StatusNotFound, "{}")
		return
	}
	dir, name, _ := strings.Cut(path, "/")
	s.set(func() {
		var keep []MediaFile
		for _, m := range s.Media {
			if m.Dir != dir || m.Name != name {
				keep = append(keep, m)
			}
		}
		s.Media = keep
	})
	writeJSON(w, http.StatusOK, "{}")
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	// p=/100GOPRO/GH010397.MP4 deletes every chapter sharing the 4-digit group number.
	path := strings.TrimPrefix(r.URL.Query().Get("p"), "/")
	dir, name, _ := strings.Cut(path, "/")
	if len(name) < 8 {
		writeJSON(w, http.StatusBadRequest, "{}")
		return
	}
	group := name[4:8]
	s.set(func() {
		var keep []MediaFile
		for _, m := range s.Media {
			if m.Dir == dir && len(m.Name) >= 8 && m.Name[4:8] == group {
				continue
			}
			keep = append(keep, m)
		}
		s.Media = keep
	})
	writeJSON(w, http.StatusOK, "{}")
}

func (s *Server) dcim(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/videos/DCIM/")
	if r.Method == http.MethodHead {
		http.Error(w, "Error 403: Forbidden\nConnection forbidden", http.StatusForbidden)
		return
	}
	s.mu.Lock()
	media := append([]MediaFile(nil), s.Media...)
	s.mu.Unlock()
	switch {
	case rel == "":
		dirs := map[string]bool{}
		var sb strings.Builder
		sb.WriteString("<html><body><table>")
		for _, m := range media {
			if !dirs[m.Dir] {
				dirs[m.Dir] = true
				fmt.Fprintf(&sb, `<tr><td><a href="/videos/DCIM/%s/">%s/</a></td></tr>`, m.Dir, m.Dir)
			}
		}
		sb.WriteString("</table></body></html>")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, sb.String())
	case strings.HasSuffix(rel, "/"):
		dir := strings.TrimSuffix(rel, "/")
		var sb strings.Builder
		sb.WriteString("<html><body><table>")
		for _, m := range media {
			if m.Dir == dir {
				fmt.Fprintf(&sb, `<tr><td><a href="/videos/DCIM/%s/%s">%s</a></td><td>%d</td></tr>`, m.Dir, m.Name, m.Name, m.Size)
			}
		}
		sb.WriteString("</table></body></html>")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, sb.String())
	default:
		for _, m := range media {
			if m.Dir+"/"+m.Name == rel {
				w.Header().Set("Content-Type", "video/mp4")
				http.ServeContent(w, r, m.Name, time.Unix(m.Mod, 0), bytesReader(m.Body))
				return
			}
		}
		s.notFound(w, r)
	}
}

func (s *Server) webcamStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	st := s.WebcamStatus
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, fmt.Sprintf(`{"status":%d,"error":0}`, st))
}

func (s *Server) webcam(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if status == 2 {
			// Wire values from the Open GoPro spec: 4=480p, 7=720p, 12=1080p.
			if res := r.URL.Query().Get("res"); res != "" && res != "4" && res != "7" && res != "12" {
				writeJSON(w, http.StatusOK, `{"status":0,"error":6}`)
				return
			}
		}
		s.set(func() {
			// HERO9 1.72: the fov parameter is ignored, and entering webcam mode
			// from off resets the webcam lens (setting 43) to Superview (3).
			if s.WebcamStatus == 0 && (status == 2 || status == 3) {
				s.Settings[43] = 3
			}
			s.WebcamStatus = status
		})
		if status == 2 || status == 3 {
			s.emitTS(r)
		}
		writeJSON(w, http.StatusOK, `{"status":0,"error":0}`)
	}
}

// emitTS sends synthetic MPEG-TS packets (188 bytes, 0x47 sync) to the
// requesting host, the way the camera streams to whoever started it.
func (s *Server) emitTS(r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return
	}
	port := s.StreamPort
	if p := r.URL.Query().Get("port"); p != "" {
		if n, perr := strconv.Atoi(p); perr == nil {
			port = n
		}
	}
	s.mu.Lock()
	count := s.StreamPackets
	s.mu.Unlock()
	go func() {
		conn, derr := (&net.Dialer{}).DialContext(context.Background(), "udp", net.JoinHostPort(host, strconv.Itoa(port)))
		if derr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		pkt := make([]byte, 188*7) // 7 TS packets per datagram like the camera
		for i := 0; i < 7; i++ {
			pkt[i*188] = 0x47
		}
		for i := 0; i < count; i++ {
			s.mu.Lock()
			streaming := s.Streaming || s.WebcamStatus == 2 || s.WebcamStatus == 3
			s.mu.Unlock()
			if !streaming {
				return
			}
			_, _ = conn.Write(pkt)
			time.Sleep(20 * time.Millisecond)
		}
	}()
}
