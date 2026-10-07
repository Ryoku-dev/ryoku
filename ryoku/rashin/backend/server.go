package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/coder/websocket"
)

// The console is a Svelte app built from ryoku/rashin/web into web/dist; the
// build output is committed so go build alone ships it (no node at build
// time). `all:` keeps Vite's dot-prefixed files.
//
//go:embed all:web/dist
var webFS embed.FS

// Serve runs the dashboard and the agent bridge on 127.0.0.1.
func Serve(cfg Config) error {
	rt := RuntimeDir()
	if err := os.MkdirAll(rt, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(rt, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another ryoku-rashin daemon is already running")
	}
	pidfile := filepath.Join(rt, "daemon.pid")
	if err := os.WriteFile(pidfile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return err
	}
	defer os.Remove(pidfile)

	go func() { _ = ensureGateway() }()

	if err := EnsureVault(); err != nil {
		return err
	}
	// Re-apply wiring lost to agent onboarding rewrites; drift heals on start.
	if HermesStatus().Installed {
		_ = WireHermesMemory()
	}
	go func() {
		if err := Reindex(); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-rashin: index:", err)
		}
	}()
	go func() {
		for range time.Tick(6 * time.Hour) {
			_ = Reindex()
		}
	}()
	// User-owned changes and the habits layer reindex separately from the
	// full map: a cheap fingerprint every 2 minutes, the rewrite only when
	// the live config or the habits sources (history, asks, runs, recipes)
	// move.
	go func() {
		lastCfg := userConfigFingerprint()
		lastHab := habitsFingerprint()
		for range time.Tick(2 * time.Minute) {
			if cur := userConfigFingerprint(); cur != lastCfg {
				lastCfg = cur
				_ = ReindexUser()
			}
			if cur := habitsFingerprint(); cur != lastHab {
				lastHab = cur
				_ = WriteHabits()
			}
		}
	}()

	// Two lanes, two hubs, two agent processes. The Ryoku lane is the
	// machine agent (the launcher bar, the terminal, the console's first
	// sheet) and the only lane with a quick path; the chat lane is a plain
	// conversation with the harness, spawned only when a surface joins it.
	hub := newChatHub(ryokuLane())
	chatHubPlain := newChatHub(chatLane())
	laneHubs = map[string]*chatHub{laneRyoku: hub, laneChat: chatHubPlain}
	// Pre-warm the Ryoku session: hermes pays its Python cold start at boot,
	// not on the user's first question.
	go hub.warm()
	mux := http.NewServeMux()

	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		return err
	}
	mux.Handle("/", webHandler(sub))

	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, BuildStatus(cfg))
	})
	mux.HandleFunc("GET /api/vitals", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, SampleVitals())
	})
	mux.HandleFunc("GET /api/system", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, SystemNow())
	})
	mux.HandleFunc("GET /api/doctor", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, DoctorNow(r.URL.Query().Get("refresh") == "1"))
	})
	mux.HandleFunc("POST /api/fix", handleFix)
	mux.HandleFunc("GET /api/theme", func(w http.ResponseWriter, r *http.Request) {
		th := ThemeNow()
		wall := map[string]any{"available": false}
		if path, kind, ok := CurrentWallpaper(); ok {
			h := fnv.New32a()
			_, _ = h.Write([]byte(path))
			rev := strconv.FormatUint(uint64(h.Sum32()), 36)
			if st, err := os.Stat(path); err == nil {
				rev += "-" + strconv.FormatInt(st.ModTime().UnixNano(), 36)
			}
			wall = map[string]any{"available": true, "kind": kind, "rev": rev}
		}
		writeJSON(w, map[string]any{
			"roles":        th.Roles,
			"source":       th.Source,
			"reduceMotion": th.ReduceMotion,
			"wallpaper":    wall,
		})
	})
	mux.HandleFunc("GET /api/wallpaper", func(w http.ResponseWriter, r *http.Request) {
		path, _, ok := CurrentWallpaper()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, path)
	})
	mux.HandleFunc("GET /api/vault", func(w http.ResponseWriter, r *http.Request) {
		files, err := VaultTree()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"root": VaultDir(), "files": files})
	})
	mux.HandleFunc("GET /api/vault/file", func(w http.ResponseWriter, r *http.Request) {
		b, err := ReadVaultFile(r.URL.Query().Get("p"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("POST /api/index", func(w http.ResponseWriter, r *http.Request) {
		if err := Reindex(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	migrationCtx, migrationCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := migrateQuickProviderKeys(migrationCtx); err != nil {
		fmt.Fprintln(os.Stderr, "ryoku-rashin: quick key migration:", err)
	}
	migrationCancel()
	mux.HandleFunc("GET /api/agents", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, DetectAgents())
	})
	mux.HandleFunc("GET /api/harnesses", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"harnesses": HarnessesWithRouting(r.Context())})
	})
	mux.HandleFunc("POST /api/agents/wire", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.ID == "" {
			http.Error(w, "missing agent id", http.StatusBadRequest)
			return
		}
		pending, reason, err := connectHarness(r.Context(), body.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !pending && chatAgentInUse(body.ID, LoadConfig()) {
			resetChatLanes(LoadConfig())
		}
		writeJSON(w, map[string]any{"ok": true, "pending": pending, "reason": reason})
	})
	mux.HandleFunc("POST /api/agents/unwire", handleUnwireAgent)
	mux.HandleFunc("POST /api/harnesses/route", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, routeConnectedHarnesses(r.Context()))
	})
	mux.HandleFunc("GET /api/quick", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, quickInfo(r.Context(), LoadConfig()))
	})
	mux.HandleFunc("POST /api/quick", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Route string `json:"route"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			http.Error(w, "missing route", http.StatusBadRequest)
			return
		}
		if err := setQuickRoute(r.Context(), body.Route); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, quickInfo(r.Context(), LoadConfig()))
	})
	mux.HandleFunc("GET /api/manifest", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, BuildManifest(LoadConfig()))
	})
	mux.HandleFunc("GET /api/chat/agent", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, chatBackendInfos(LoadConfig()))
	})
	mux.HandleFunc("POST /api/chat/agent", handleSetChatAgent)

	mux.HandleFunc("GET /api/hermes/skills", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, SkillsReportNow())
	})
	mux.HandleFunc("GET /api/hermes/memory", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, MemoryReportNow())
	})
	registerProwlProxy(mux, cfg.Port)
	mux.HandleFunc("GET /api/about", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, AboutReportNow(cfg))
	})
	mux.HandleFunc("POST /api/ask", hub.handleAsk)
	mux.HandleFunc("POST /api/ask/cancel", hub.handleAskCancel)
	mux.HandleFunc("GET /api/ask/recent", hub.handleAskRecent)
	mux.HandleFunc("POST /api/term", hub.handleTerm)
	mux.HandleFunc("POST /api/term/cancel", hub.handleTermCancel)
	mux.HandleFunc("POST /api/term/ran", hub.handleTermRan)
	mux.HandleFunc("POST /api/perm", hub.handlePerm)

	mux.HandleFunc("GET /ws/vitals", func(w http.ResponseWriter, r *http.Request) {
		ws, err := acceptWS(w, r)
		if err != nil {
			return
		}
		defer ws.CloseNow()
		serveVitalsWS(r.Context(), ws)
	})
	mux.HandleFunc("GET /ws/chat", func(w http.ResponseWriter, r *http.Request) {
		// ?lane= picks the conversation; an absent lane is the Ryoku lane,
		// so every surface built before lanes existed keeps its meaning.
		name := r.URL.Query().Get("lane")
		if name == "" {
			name = laneRyoku
		}
		h, ok := laneHubs[name]
		if !ok {
			http.Error(w, "unknown lane", http.StatusBadRequest)
			return
		}
		ws, err := acceptWS(w, r)
		if err != nil {
			return
		}
		defer ws.CloseNow()
		h.handle(r.Context(), ws)
	})

	srv := &http.Server{
		Addr:              net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port)),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	fmt.Printf("ryoku-rashin: dashboard on http://%s\n", srv.Addr)

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func handleUnwireAgent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.ID == "" {
		http.Error(w, "missing agent id", http.StatusBadRequest)
		return
	}
	if chatAgentInUse(body.ID, LoadConfig()) {
		http.Error(w, "pick another Rashin chat agent before disconnecting this harness", http.StatusConflict)
		return
	}
	if err := disconnectHarness(r.Context(), body.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func handleSetChatAgent(w http.ResponseWriter, r *http.Request) {
	routing, err := setChatAgent(r.Context(), r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resetChatLanes(LoadConfig())
	writeJSON(w, chatAgentSelection{Agents: chatBackendInfos(LoadConfig()), Routing: routing})
}

// laneHubs holds the running daemon's chat lanes; it stays nil outside serve.
var laneHubs map[string]*chatHub

// resetChatLanes drops each lane's live harness process so the next turn
// starts one that reads the harness's current config, as after Prowl routing
// rewrote it. The daemon resets the lanes it owns; the CLI asks a running
// daemon to do it through the chat-agent route, which also re-confirms routing.
func resetChatLanes(cfg Config) {
	if laneHubs != nil {
		for _, h := range laneHubs {
			h.resetConn()
		}
		return
	}
	notifyDaemonChatAgent(cfg.Port, cfg.ChatAgent)
}

// notifyDaemonChatAgent tells a running daemon to switch (or re-confirm) its
// chat agent, which drops its live sessions. A variable because it reaches a
// real daemon on this machine's port: tests replace it.
var notifyDaemonChatAgent = func(port int, id string) {
	if pingDaemon(port) {
		askPost(port, "/api/chat/agent?id="+url.QueryEscape(id))
	}
}

// loopbackOrigin reports whether a browser request came from this machine's
// own dashboard. A request with no Origin (the CLI, curl) is local by the
// listener's own bind.
func loopbackOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	return err == nil && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost")
}

// acceptWS upgrades only when the Origin is this machine's own dashboard.
func acceptWS(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	if !loopbackOrigin(r) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return nil, errors.New("bad origin")
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"127.0.0.1:*", "localhost:*"},
	})
	if err == nil {
		// User messages carry base64 image blocks; the 32 KiB default read
		// limit rejects all but the smallest, so raise it past a downscaled image.
		conn.SetReadLimit(8 << 20)
	}
	return conn, err
}

func cmdServe(ifEnabled bool) error {
	cfg := LoadConfig()
	if ifEnabled && !cfg.Enabled {
		return nil
	}
	return Serve(cfg)
}

// systemd helpers: rashin prefers running as a systemd user unit so it starts
// with every login (and with the machine itself once lingering is on),
// survives Hyprland restarts, and restarts on crash. Everything degrades to
// the plain detached spawn when systemd is not around.

const rashinUnit = "ryoku-rashin.service"

func systemctlUser(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", append([]string{"--user"}, args...)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// unitKnown reports whether the user unit file is installed anywhere systemd
// looks (package: /usr/lib/systemd/user, dev deploy: ~/.config/systemd/user).
func unitKnown() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", "--user", "list-unit-files", rashinUnit, "--no-legend").Output()
	return err == nil && strings.Contains(string(out), rashinUnit)
}

func haveSystemd() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	return os.Getenv("XDG_RUNTIME_DIR") != ""
}

func cmdEnable(atBoot bool) error {
	cfg := LoadConfig()
	cfg.Enabled = true
	cfg.OptedOut = false
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	if haveSystemd() && unitKnown() {
		// An old detached daemon would hold the flock and wedge the unit.
		stopSpawnedDaemon()
		if err := systemctlUser("enable", "--now", rashinUnit); err != nil {
			return err
		}
		if err := ensureGateway(); err != nil {
			return err
		}
		if atBoot {
			// Lingering starts the user manager (and this unit) at BOOT,
			// before login. Needs polkit auth or root once.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			lc := exec.CommandContext(ctx, "loginctl", "enable-linger", currentUser())
			lc.Stdout, lc.Stderr = os.Stdout, os.Stderr
			if err := lc.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "ryoku-rashin: enable-linger failed (dashboard will start at login instead of boot):", err)
			} else {
				fmt.Println("rashin enabled (starts at boot via lingering)")
				return nil
			}
		}
		fmt.Println("rashin enabled (systemd user unit, starts at login)")
		return nil
	}
	// No systemd: fall back to a detached spawn for this session only.
	if !pingDaemon(cfg.Port) {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		cmd := exec.Command(self, "serve")
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return err
		}
		_ = cmd.Process.Release()
	}
	if err := ensureGateway(); err != nil {
		return err
	}
	fmt.Println("rashin enabled")
	return nil
}

func cmdDisable() error {
	cfg := LoadConfig()
	cfg.Enabled = false
	cfg.OptedOut = true
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	var stopErr error
	if haveSystemd() && unitKnown() {
		stopErr = systemctlUser("disable", "--now", rashinUnit)
	} else {
		stopErr = stopGateway()
	}
	stopSpawnedDaemon()
	if stopErr != nil {
		return stopErr
	}
	fmt.Println("rashin disabled")
	return nil
}

// cmdEnsure is the default-on convergence for installers and `ryoku doctor`:
// enable at boot unless the user opted out. Idempotent and quiet.
func cmdEnsure() error {
	cfg := LoadConfig()
	if cfg.OptedOut {
		fmt.Println("rashin left off by choice")
		return nil
	}
	if cfg.Enabled && rashinActive() {
		return ensureGateway()
	}
	return cmdEnable(true)
}

// rashinActive: is the user unit running? False when systemd is absent.
func rashinActive() bool {
	if !haveSystemd() || !unitKnown() {
		return false
	}
	out, _ := exec.Command("systemctl", "--user", "is-active", rashinUnit).Output()
	return strings.TrimSpace(string(out)) == "active"
}

// stopSpawnedDaemon SIGTERMs a daemon started by the pre-systemd spawn path.
func stopSpawnedDaemon() {
	b, err := os.ReadFile(filepath.Join(RuntimeDir(), "daemon.pid"))
	if err != nil {
		return
	}
	if pid, perr := strconv.Atoi(strings.TrimSpace(string(b))); perr == nil {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return strconv.Itoa(os.Getuid())
}

func pingDaemon(port int) bool {
	c := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/api/ping", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
