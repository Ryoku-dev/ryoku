// ryoget is Ryoku's download engine: an on-demand daemon behind the Rashin
// bar's Downloads view and its command line. Web search lives in ryoseek.
package main

import (
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
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	i18n "ryoku-i18n"

	"ryoget/internal/daemon"
	"ryoget/internal/links"
	"ryoget/internal/paths"
	"ryoget/internal/proto"
)

var version = "dev"

const usage = `usage:
  ryoget serve
  ryoget ensure
  ryoget add <url>... | - [--audio|--mute] [--dir DIR] [--paused]
  ryoget list [--json]
  ryoget pause|resume|cancel|remove|retry <id|all>
  ryoget clear [--done|--failed]
  ryoget resolve <url> [--json]
  ryoget version
`

const (
	exitOK      = 0
	exitFail    = 1
	exitUsage   = 2
	exitMissing = 3
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
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "serve":
		return serve(ctx)
	case "ensure":
		return ensure(ctx)
	case "version", "--version":
		fmt.Println("ryoget", version)
		return exitOK
	case "add":
		return withClient(ctx, func(c *daemon.Client) error { return add(ctx, c, rest) })
	case "list":
		return withClient(ctx, func(c *daemon.Client) error { return list(ctx, c, rest) })
	case "pause", "resume", "cancel", "remove", "retry":
		if len(rest) != 1 {
			fmt.Fprint(os.Stderr, usage)
			return exitUsage
		}
		return withClient(ctx, func(c *daemon.Client) error { return act(ctx, c, cmd, rest[0]) })
	case "clear":
		return withClient(ctx, func(c *daemon.Client) error { return clearList(ctx, c, rest) })
	case "resolve":
		return withClient(ctx, func(c *daemon.Client) error { return resolve(ctx, c, rest) })
	case "help", "-h", "--help":
		fmt.Print(usage)
		return exitOK
	}
	fmt.Fprint(os.Stderr, usage)
	return exitUsage
}

func serve(ctx context.Context) int {
	logger := log.New(os.Stderr, "ryoget: ", log.LstdFlags)
	err := daemon.Run(ctx, daemon.Options{Parts: wire, Log: logger})
	if errors.Is(err, daemon.ErrRunning) {
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
	nc, err := d.DialContext(cctx, "unix", paths.Socket())
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
// and falls back to a detached `ryoget serve` where no unit answers.
func ensure(ctx context.Context) int {
	if reachable(ctx) {
		return exitOK
	}
	host, hostErr := exec.LookPath("ryoku-host")
	if hostErr == nil {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := exec.CommandContext(cctx, host, "svc", "--user", "start", "ryoget").Run()
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
		fmt.Fprintln(os.Stderr, "ryoget:", err)
		return exitFail
	}
	if waitSocket(ctx, 3*time.Second) {
		return exitOK
	}
	return exitFail
}

func selfPath() (string, error) {
	if p, err := os.Executable(); err == nil {
		return p, nil
	}
	return exec.LookPath("ryoget")
}

func spawnServe(bin string) error {
	if err := paths.EnsureDir(paths.StateDir()); err != nil {
		return err
	}
	logf, err := os.OpenFile(paths.Log(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
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

func withClient(ctx context.Context, f func(c *daemon.Client) error) int {
	if code := ensure(ctx); code != exitOK {
		fmt.Fprintln(os.Stderr, i18n.T("ryoget: the download daemon is not running and could not be started"))
		return code
	}
	c, err := daemon.Dial(ctx, paths.Socket())
	if err != nil {
		fmt.Fprintln(os.Stderr, "ryoget:", err)
		return exitFail
	}
	defer c.Close()
	if err := f(c); err != nil {
		fmt.Fprintln(os.Stderr, "ryoget:", err)
		return exitFail
	}
	return exitOK
}

func callCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 30*time.Second)
}

func add(ctx context.Context, c *daemon.Client, args []string) error {
	var urls []string
	fromStdin := false
	p := proto.AddParams{}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--audio":
			p.Mode = "audio"
		case "--mute":
			p.Mode = "mute"
		case "--paused":
			p.Paused = true
		case "--dir":
			if i+1 >= len(args) {
				return errors.New(i18n.T("--dir needs a folder"))
			}
			i++
			dir, err := filepath.Abs(paths.Expand(args[i]))
			if err != nil {
				return err
			}
			p.Dir = dir
		case "-":
			fromStdin = true
		default:
			if strings.HasPrefix(a, "--") {
				return fmt.Errorf("%s: %s", i18n.T("unknown option"), a)
			}
			urls = append(urls, a)
		}
	}
	if fromStdin {
		return addText(ctx, c, p, urls)
	}
	if len(urls) == 0 {
		return errors.New(i18n.T("nothing to add"))
	}
	var failed error
	for _, u := range urls {
		if !strings.Contains(u, "://") && !strings.HasPrefix(u, "magnet:") {
			if abs, err := filepath.Abs(u); err == nil {
				if _, err := os.Stat(abs); err == nil {
					u = abs
				}
			}
		}
		p.URL = u
		cctx, cancel := callCtx(ctx)
		var t proto.Task
		err := c.Call(cctx, "add", p, &t)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ryoget: %s: %v\n", u, err)
			failed = err
			continue
		}
		fmt.Printf("%s\t%s\n", t.ID, t.Title)
	}
	return failed
}

// addText adds every link found in standard input, cleaned and without
// duplicates, in one addMany call.
func addText(ctx context.Context, c *daemon.Client, p proto.AddParams, extra []string) error {
	b, err := io.ReadAll(io.LimitReader(os.Stdin, proto.MaxFrame/2))
	if err != nil {
		return err
	}
	text := string(b) + "\n" + strings.Join(extra, "\n")
	found := links.Parse(text)
	req := proto.AddManyParams{Dir: p.Dir, Paused: p.Paused}
	var sent []proto.Link
	for _, l := range found.Links {
		if !l.Duplicate {
			req.Items = append(req.Items, proto.AddManyItem{URL: l.URL, Mode: p.Mode})
			sent = append(sent, l)
		}
	}
	if len(req.Items) == 0 {
		return errors.New(i18n.T("no links found"))
	}
	cctx, cancel := callCtx(ctx)
	defer cancel()
	var res proto.AddManyResult
	if err := c.Call(cctx, "addMany", req, &res); err != nil {
		return err
	}
	for _, t := range res.Tasks {
		fmt.Printf("%s\t%s\n", t.ID, t.Title)
	}
	for _, e := range res.Errors {
		fmt.Fprintf(os.Stderr, "ryoget: %s: %s: %s\n", sent[e.Index].URL, e.Code, e.Message)
	}
	if len(res.Errors) > 0 {
		return errors.New(i18n.Tf("%d of %d links could not be added", len(res.Errors), len(req.Items)))
	}
	return nil
}

func list(ctx context.Context, c *daemon.Client, args []string) error {
	asJSON := len(args) == 1 && args[0] == "--json"
	if len(args) > 0 && !asJSON {
		return errors.New(i18n.T("unknown option"))
	}
	cctx, cancel := callCtx(ctx)
	defer cancel()
	h, err := c.Hello(cctx, "cli")
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(h.Tasks)
	}
	printTasks(os.Stdout, h.Tasks, h.Stats)
	return nil
}

func printTasks(w io.Writer, tasks []proto.Task, st proto.Stats) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join([]string{i18n.T("ID"), i18n.T("STATE"), i18n.T("DONE"), i18n.T("SPEED"), i18n.T("ETA"), i18n.T("CONNS"), i18n.T("NAME")}, "\t"))
	for _, t := range tasks {
		done := human(t.Done)
		if t.Size > 0 {
			done = fmt.Sprintf("%s / %s (%.1f%%)", human(t.Done), human(t.Size), float64(t.Done)*100/float64(t.Size))
		}
		state := t.State
		if t.Stage != "" && t.State != proto.StateDone {
			state += "/" + t.Stage
		}
		if t.Error != nil {
			state += " (" + t.Error.Code + ")"
		}
		eta := "-"
		if t.ETA >= 0 {
			eta = (time.Duration(t.ETA) * time.Second).String()
		}
		conns := fmt.Sprint(t.Conns)
		if t.Kind == "torrent" {
			conns = i18n.Tf("%d peers, %d seeds", t.Peers, t.Seeds)
		}
		name := t.Name
		if name == "" {
			name = t.Title
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s/s\t%s\t%s\t%s\n", t.ID, state, done, human(t.Speed), eta, conns, name)
	}
	tw.Flush()
	fmt.Fprintf(w, "%s %s/s  %s %s/s  %d active, %d queued, %d seeding\n",
		i18n.T("down"), human(st.Down), i18n.T("up"), human(st.Up), st.Active, st.Queued, st.Seeding)
}

func human(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func act(ctx context.Context, c *daemon.Client, method, target string) error {
	cctx, cancel := callCtx(ctx)
	defer cancel()
	if target != "all" {
		return c.Call(cctx, method, proto.IDParams{ID: target}, nil)
	}
	if method == "pause" || method == "resume" {
		return c.Call(cctx, method, proto.IDParams{All: true}, nil)
	}
	h, err := c.Hello(cctx, "cli")
	if err != nil {
		return err
	}
	var failed error
	for _, t := range h.Tasks {
		switch method {
		case "cancel":
			if t.State == proto.StateDone || t.State == proto.StateStopped {
				continue
			}
		case "retry":
			if t.State != proto.StateError && t.State != proto.StateStopped {
				continue
			}
		}
		if err := c.Call(cctx, method, proto.IDParams{ID: t.ID}, nil); err != nil {
			fmt.Fprintf(os.Stderr, "ryoget: %s: %v\n", t.ID, err)
			failed = err
		}
	}
	return failed
}

// a failed download's partial data goes with it, a finished file stays
func clearList(ctx context.Context, c *daemon.Client, args []string) error {
	states := []string{proto.StateDone, proto.StateError, proto.StateStopped}
	for _, a := range args {
		switch a {
		case "--done":
			states = []string{proto.StateDone}
		case "--failed":
			states = []string{proto.StateError, proto.StateStopped}
		default:
			return fmt.Errorf("unknown flag %s", a)
		}
	}
	cctx, cancel := callCtx(ctx)
	defer cancel()
	return c.Call(cctx, "clear", map[string][]string{"states": states}, nil)
}

func resolve(ctx context.Context, c *daemon.Client, args []string) error {
	asJSON := false
	var link string
	for _, a := range args {
		if a == "--json" {
			asJSON = true
		} else {
			link = a
		}
	}
	if link == "" {
		return errors.New(i18n.T("resolve needs a link"))
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var r proto.Resolved
	if err := c.Call(cctx, "resolve", map[string]string{"url": link}, &r); err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	fmt.Printf("%s\t%s\n", r.Kind, r.Title)
	if r.Source.Name != "" {
		fmt.Printf("%s\t%s\n", i18n.T("source"), r.Source.Name)
	}
	if r.Size > 0 {
		fmt.Printf("%s\t%s\n", i18n.T("size"), human(r.Size))
	}
	for _, f := range r.Formats {
		mark := " "
		if f.ID == r.DefaultFormat {
			mark = "*"
		}
		fmt.Printf("%s %s\t%s\t%s\n", mark, f.ID, f.Label, f.Ext)
	}
	for i, f := range r.Files {
		fmt.Printf("  %d\t%s\t%s\n", i, f.Path, human(f.Size))
	}
	if r.Note != "" {
		fmt.Println(r.Note)
	}
	return nil
}
