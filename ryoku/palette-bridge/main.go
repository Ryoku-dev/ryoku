package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type snapshot struct {
	data    []byte
	version uint64
}

type paletteHub struct {
	mu      sync.RWMutex
	current snapshot
	clients map[chan snapshot]struct{}
}

func newPaletteHub() *paletteHub {
	return &paletteHub{clients: make(map[chan snapshot]struct{})}
}

func validatePalette(data []byte) ([]byte, error) {
	var palette map[string]string
	if err := json.Unmarshal(data, &palette); err != nil {
		return nil, fmt.Errorf("decode palette: %w", err)
	}
	for _, role := range []string{"primary", "surface", "onSurface"} {
		color := palette[role]
		if !hexColor.MatchString(color) {
			return nil, fmt.Errorf("palette role %q is missing or invalid", role)
		}
	}
	for role, color := range palette {
		if !hexColor.MatchString(color) {
			return nil, fmt.Errorf("palette role %q has invalid color %q", role, color)
		}
	}
	compact := new(bytes.Buffer)
	if err := json.Compact(compact, data); err != nil {
		return nil, err
	}
	return compact.Bytes(), nil
}

func (h *paletteHub) publish(data []byte) error {
	valid, err := validatePalette(data)
	if err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if bytes.Equal(valid, h.current.data) {
		return nil
	}
	h.current = snapshot{data: valid, version: h.current.version + 1}
	for client := range h.clients {
		select {
		case client <- h.current:
		default:
			// A slow client needs the latest palette, not an outdated queued one.
			select {
			case <-client:
			default:
			}
			client <- h.current
		}
	}
	return nil
}

func (h *paletteHub) readFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return h.publish(data)
}

func (h *paletteHub) snapshot() snapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return snapshot{data: bytes.Clone(h.current.data), version: h.current.version}
}

func corsHeaders(header http.Header) {
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	header.Set("Access-Control-Allow-Private-Network", "true")
}

func (h *paletteHub) paletteHandler(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w.Header())
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s := h.snapshot()
	if len(s.data) == 0 {
		http.Error(w, "palette unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("ETag", fmt.Sprintf("\"%d\"", s.version))
	_, _ = w.Write(s.data)
}

func writeEvent(w http.ResponseWriter, s snapshot) error {
	_, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", s.version, s.data)
	return err
}

func (h *paletteHub) eventsHandler(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w.Header())
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	updates := make(chan snapshot, 1)
	h.mu.Lock()
	h.clients[updates] = struct{}{}
	initial := h.current
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, updates)
		h.mu.Unlock()
	}()

	if len(initial.data) > 0 {
		if err := writeEvent(w, initial); err != nil {
			return
		}
		flusher.Flush()
	}

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case update := <-updates:
			if err := writeEvent(w, update); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func watchPalette(path string, reload func() error) error {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	dir, name := filepath.Dir(path), filepath.Base(path)
	_, err = syscall.InotifyAddWatch(fd, dir, syscall.IN_CLOSE_WRITE|syscall.IN_MOVED_TO)
	if err != nil {
		return err
	}

	buffer := make([]byte, syscall.SizeofInotifyEvent+4096)
	for {
		n, err := syscall.Read(fd, buffer)
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return err
		}
		changed := false
		for offset := 0; offset+syscall.SizeofInotifyEvent <= n; {
			event := (*syscall.InotifyEvent)(unsafe.Pointer(&buffer[offset]))
			start := offset + syscall.SizeofInotifyEvent
			end := start + int(event.Len)
			eventName := string(bytes.TrimRight(buffer[start:end], "\x00"))
			offset = end
			if eventName == name {
				changed = true
			}
		}
		if changed {
			if err := reload(); err != nil {
				log.Printf("palette update ignored: %v", err)
			}
		}
	}
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		cache = filepath.Join(home, ".cache")
	}
	palette := flag.String("palette", filepath.Join(cache, "ryoku/colors.json"), "palette JSON path")
	listen := flag.String("listen", "127.0.0.1:47616", "HTTP listen address")
	flag.Parse()

	hub := newPaletteHub()
	if err := hub.readFile(*palette); err != nil {
		log.Fatalf("initial palette: %v", err)
	}
	go func() {
		if err := watchPalette(*palette, func() error { return hub.readFile(*palette) }); err != nil {
			log.Fatalf("palette watcher: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/colors.json", hub.paletteHandler)
	mux.HandleFunc("/v1/palette", hub.paletteHandler)
	mux.HandleFunc("/v1/events", hub.eventsHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		_ = server.Close()
	}()
	log.Printf("serving %s on http://%s", *palette, *listen)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
