// ryoseek is Ryoku's search daemon: one per user, started on demand by the
// Hyakki bar, it owns every web engine's budget and block timer so they
// survive the bar closing.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	i18n "ryoku-i18n"

	"ryoget/internal/paths"
	"ryoget/internal/search"
)

var version = "dev"

const usage = `usage:
  ryoseek serve
  ryoseek ensure
  ryoseek query <text> [--scope S] [--mode suggest|full] [--json]
  ryoseek version
`

const (
	exitOK      = 0
	exitFail    = 1
	exitUsage   = 2
	exitMissing = 3

	maxLine      = 1 << 20
	queryTimeout = 30 * time.Second
)

func main() {
	i18n.Use("")
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return exitUsage
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	switch args[0] {
	case "serve":
		return serve(ctx)
	case "ensure":
		return ensure(ctx)
	case "query":
		return query(ctx, args[1:], os.Stdout)
	case "version", "--version":
		fmt.Println("ryoseek", version)
		return exitOK
	case "help", "-h", "--help":
		fmt.Print(usage)
		return exitOK
	}
	fmt.Fprint(os.Stderr, usage)
	return exitUsage
}

func serve(ctx context.Context) int {
	logger := log.New(os.Stderr, "ryoseek: ", log.LstdFlags)
	err := runServer(ctx, serverOptions{
		Socket: paths.SeekSocket(),
		Lock:   paths.SeekLock(),
		Idle:   idleFromEnv(),
		Log:    logger,
		UID:    os.Getuid(),
		NewService: func() (*search.Service, error) {
			return search.New(search.Options{
				StateFile: paths.SeekHealthFile(),
				CacheDir:  paths.SeekCacheDir(),
				Version:   version,
			})
		},
	})
	if errors.Is(err, errRunning) {
		logger.Print(err)
		return exitOK
	}
	if err != nil {
		logger.Print(err)
		return exitFail
	}
	return exitOK
}

func reachable(ctx context.Context) bool {
	var d net.Dialer
	cctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	nc, err := d.DialContext(cctx, "unix", paths.SeekSocket())
	if err != nil {
		return false
	}
	nc.Close()
	return true
}

func waitSocket(ctx context.Context, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if reachable(ctx) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(50 * time.Millisecond):
		}
	}
	return false
}

// ensure starts the daemon through ryoku-host, the init-agnostic service door,
// and falls back to a detached `ryoseek serve` where no unit answers.
func ensure(ctx context.Context) int {
	if reachable(ctx) {
		return exitOK
	}
	host, hostErr := exec.LookPath("ryoku-host")
	if hostErr == nil {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := exec.CommandContext(cctx, host, "svc", "--user", "start", "ryoseek").Run()
		cancel()
		if err == nil && waitSocket(ctx, 3*time.Second) {
			return exitOK
		}
	}
	self, err := selfPath()
	if err != nil {
		if hostErr != nil {
			return exitMissing
		}
		return exitFail
	}
	if err := spawnServe(self); err != nil {
		fmt.Fprintln(os.Stderr, "ryoseek:", err)
		return exitFail
	}
	if waitSocket(ctx, 3*time.Second) {
		return exitOK
	}
	return exitFail
}

var selfPath = func() (string, error) {
	if p, err := os.Executable(); err == nil {
		return p, nil
	}
	return exec.LookPath("ryoseek")
}

// spawnServe starts `bin serve` in its own session so it outlives the caller.
func spawnServe(bin string) error {
	if err := paths.EnsureDir(paths.SeekStateDir()); err != nil {
		return err
	}
	logf, err := os.OpenFile(paths.SeekLog(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logf.Close()
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer devnull.Close()
	cmd := exec.Command(bin, "serve")
	cmd.Dir = paths.Home()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

type querySpec struct {
	Text  string
	Scope string
	Mode  string
	JSON  bool
}

func parseQuery(args []string) (querySpec, error) {
	q := querySpec{Scope: "all", Mode: "full"}
	var words []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--json":
			q.JSON = true
		case "--scope", "--mode":
			if i+1 >= len(args) {
				return q, fmt.Errorf("%s needs a value", a)
			}
			i++
			if a == "--scope" {
				q.Scope = args[i]
			} else {
				q.Mode = args[i]
			}
		default:
			words = append(words, a)
		}
	}
	q.Text = strings.TrimSpace(strings.Join(words, " "))
	if q.Text == "" {
		return q, errors.New("nothing to search for")
	}
	if q.Mode != "suggest" && q.Mode != "full" {
		return q, fmt.Errorf("unknown mode %q", q.Mode)
	}
	return q, nil
}

type sourceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type item struct {
	Kind    string          `json:"kind"`
	Title   string          `json:"title"`
	URL     string          `json:"url"`
	Snippet string          `json:"snippet"`
	Site    string          `json:"site"`
	Meta    json.RawMessage `json:"meta"`
}

type reply struct {
	Ready   bool         `json:"ready"`
	Sources []sourceInfo `json:"sources"`
	ID      *int64       `json:"id"`
	Source  string       `json:"source"`
	State   string       `json:"state"`
	Items   []item       `json:"items"`
	Rank    int          `json:"rank"`
	Until   string       `json:"until"`
	Error   string       `json:"error"`
}

type lane struct {
	reply
	order int
}

func query(ctx context.Context, args []string, out io.Writer) int {
	q, err := parseQuery(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ryoseek:", err)
		fmt.Fprint(os.Stderr, usage)
		return exitUsage
	}
	if code := ensure(ctx); code != exitOK {
		fmt.Fprintln(os.Stderr, i18n.T("ryoseek: the search daemon is not running and could not be started"))
		return code
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	if err := ask(ctx, q, out); err != nil {
		fmt.Fprintln(os.Stderr, "ryoseek:", err)
		return exitFail
	}
	return exitOK
}

func ask(ctx context.Context, q querySpec, out io.Writer) error {
	var d net.Dialer
	nc, err := d.DialContext(ctx, "unix", paths.SeekSocket())
	if err != nil {
		return err
	}
	defer nc.Close()
	stop := context.AfterFunc(ctx, func() { nc.Close() })
	defer stop()

	enc := json.NewEncoder(nc)
	hello := map[string]any{"hello": map[string]any{"client": "cli", "version": 2, "lang": localeTag()}}
	if err := enc.Encode(hello); err != nil {
		return err
	}
	if err := enc.Encode(map[string]any{"id": 1, "q": q.Text, "scope": q.Scope, "mode": q.Mode}); err != nil {
		return err
	}

	sc := bufio.NewScanner(nc)
	sc.Buffer(make([]byte, 64<<10), maxLine)
	names := map[string]string{}
	// ties in rank keep the order of the ready line
	position := map[string]int{}
	lanes := map[string]*lane{}
	for sc.Scan() {
		line := sc.Bytes()
		var r reply
		if err := json.Unmarshal(line, &r); err != nil {
			return err
		}
		if r.Ready {
			for i, s := range r.Sources {
				names[s.ID] = s.Name
				position[s.ID] = i
			}
		}
		if q.JSON {
			fmt.Fprintf(out, "%s\n", line)
		}
		if r.ID == nil || *r.ID != 1 {
			continue
		}
		if r.Source == "" {
			if r.State == "done" {
				if !q.JSON {
					printLanes(out, lanes, names)
				}
				return nil
			}
			continue
		}
		l := lanes[r.Source]
		if l == nil {
			pos, ok := position[r.Source]
			if !ok {
				pos = len(position) + len(lanes)
			}
			l = &lane{order: pos}
			lanes[r.Source] = l
		}
		l.reply = r
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		return err
	}
	if ctx.Err() != nil {
		return errors.New(i18n.T("the search did not finish in time"))
	}
	return errors.New(i18n.T("the search daemon closed the connection"))
}

func printLanes(w io.Writer, lanes map[string]*lane, names map[string]string) {
	order := make([]*lane, 0, len(lanes))
	for _, l := range lanes {
		order = append(order, l)
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].Rank != order[j].Rank {
			return order[i].Rank > order[j].Rank
		}
		return order[i].order < order[j].order
	})
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	for _, l := range order {
		name := names[l.Source]
		if name == "" {
			name = l.Source
		}
		switch {
		case l.State == "paused" && l.Until != "":
			fmt.Fprintln(bw, i18n.Tf("%s: paused until %s", name, untilClock(l.Until)))
			continue
		case l.State == "error":
			fmt.Fprintf(bw, "%s: %s\n", name, l.Error)
			continue
		case len(l.Items) == 0:
			continue
		}
		fmt.Fprintln(bw, name)
		for i, it := range l.Items {
			printItem(bw, i+1, it)
		}
		fmt.Fprintln(bw)
	}
}

func printItem(w io.Writer, n int, it item) {
	switch it.Kind {
	case "suggestion":
		fmt.Fprintf(w, "  %s\n", it.Title)
	case "definition":
		var m struct {
			PartOfSpeech string `json:"partOfSpeech"`
			Attribution  string `json:"attribution"`
		}
		_ = json.Unmarshal(it.Meta, &m)
		head := it.Title
		if m.PartOfSpeech != "" {
			head += " (" + m.PartOfSpeech + ")"
		}
		fmt.Fprintf(w, "  %s: %s\n", head, it.Snippet)
		if m.Attribution != "" {
			fmt.Fprintf(w, "    %s\n", m.Attribution)
		}
	default:
		fmt.Fprintf(w, "  %d. %s\n", n, it.Title)
		if it.URL != "" {
			fmt.Fprintf(w, "     %s\n", it.URL)
		}
		if s := oneLine(it.Snippet, 160); s != "" {
			fmt.Fprintf(w, "     %s\n", s)
		}
	}
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

func untilClock(v string) string {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return v
	}
	return t.Local().Format("15:04")
}

// localeTag gives engines the reader's full locale ("pt-BR"), not just the
// catalog language.
func localeTag() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := os.Getenv(k)
		if i := strings.IndexAny(v, ".@"); i >= 0 {
			v = v[:i]
		}
		if v == "" || v == "C" || v == "POSIX" {
			continue
		}
		return strings.ReplaceAll(v, "_", "-")
	}
	return strings.ReplaceAll(i18n.Lang(), "_", "-")
}
