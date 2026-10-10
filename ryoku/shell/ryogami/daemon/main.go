package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// unmarshalStrict is json.Unmarshal with the error surfaced (helper shared by
// the verb layer; declared here to keep verbs.go free of encoding imports).
func unmarshalStrict(body string, into interface{}) error {
	return json.Unmarshal([]byte(body), into)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "daemon" {
		if err := runDaemon(); err != nil {
			fmt.Fprintln(os.Stderr, "ryogami:", err)
			os.Exit(1)
		}
		return
	}
	// Client mode: forward `wallpaper ...` / `depth ...` verb lines to the
	// daemon and print any reply that is not a bare ok, exactly like the Rust
	// CLI this replaces (keybinds and scripts shell out to it).
	switch args[0] {
	case "wallpaper", "depth":
		line := strings.Join(args, " ")
		reply, err := sendLine(line)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ryogami:", err)
			os.Exit(1)
		}
		if strings.HasPrefix(reply, "err") {
			fmt.Fprintln(os.Stderr, "ryogami:", strings.TrimPrefix(reply, "err "))
			os.Exit(1)
		}
		if reply != "" && reply != "ok" {
			fmt.Println(reply)
		}
	case "upscale":
		clientUpscale(args[1:])
	case "upscale-worker":
		if err := runUpscaleWorker(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "ryogami:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: ryogami [daemon | wallpaper set [--workspace] [--screen <output>] <path> | wallpaper assign|unassign [--screen <output>] | wallpaper <mode> ... | depth <set|clear> ... | upscale <start|status|cancel> ...]")
		os.Exit(2)
	}
}

func sendLine(line string) (string, error) {
	conn, err := net.DialTimeout("unix", socketPath(), 2*time.Second)
	if err != nil {
		return "", fmt.Errorf("daemon not reachable at %s (is `ryogami` running?)", socketPath())
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if _, err := fmt.Fprintf(conn, "%s\n", line); err != nil {
		return "", err
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && reply == "" {
		return "", err
	}
	return strings.TrimSpace(reply), nil
}

// sendRPC sends one JSON request and returns the reply's result payload; the
// daemon's error replies surface as Go errors.
func sendRPC(method string, params map[string]interface{}) (map[string]interface{}, error) {
	conn, err := net.DialTimeout("unix", socketPath(), 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("daemon not reachable at %s (is `ryogami` running?)", socketPath())
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	req, err := json.Marshal(request{Method: method, Params: raw, ID: 1})
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(conn, "%s\n", req); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && line == "" {
		return nil, err
	}
	var resp response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("%s", resp.Error.Message)
	}
	if m, ok := resp.Result.(map[string]interface{}); ok {
		return m, nil
	}
	return map[string]interface{}{}, nil
}

// clientUpscale is the terminal's window into the enhance job:
//
//	ryogami upscale start <file> [scale]   kick one off
//	ryogami upscale status                 phase, frames, the saved path
//	ryogami upscale cancel                 kill the running job
//
// status is watch-friendly, so a long video job has a progress face outside
// the picker (whose edit panel shows the same state live).
func clientUpscale(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: ryogami upscale start <file> [scale] | status | cancel")
		os.Exit(2)
	}
	switch args[0] {
	case "start":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: ryogami upscale start <file> [scale]")
			os.Exit(2)
		}
		params := map[string]interface{}{"input": args[1]}
		if len(args) > 2 {
			if n, err := strconv.Atoi(args[2]); err == nil && n > 0 {
				params["scale"] = n
			}
		}
		if _, err := sendRPC("upscale.start", params); err != nil {
			fmt.Fprintln(os.Stderr, "ryogami:", err)
			os.Exit(1)
		}
		fmt.Println("started; progress: ryogami upscale status")
	case "status":
		st, err := sendRPC("upscale.status", nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ryogami:", err)
			os.Exit(1)
		}
		state := "idle"
		if b, ok := st["running"].(bool); ok && b {
			state = "running"
		}
		fmt.Printf("%s %s %d/%d %s\n", state, st["phase"], jsonInt(st["progress"]), jsonInt(st["total"]), st["file"])
		if v, ok := st["verdict"].(map[string]interface{}); ok && len(v) > 0 {
			if out, _ := v["out"].(string); out != "" {
				fmt.Println("saved:", out)
			} else if why, _ := v["why"].(string); why != "" {
				fmt.Println("last run:", v["result"], "("+why+")")
			} else if r, _ := v["result"].(string); r != "" {
				fmt.Println("last run:", r)
			}
		}
	case "cancel":
		if _, err := sendRPC("upscale.cancel", nil); err != nil {
			fmt.Fprintln(os.Stderr, "ryogami:", err)
			os.Exit(1)
		}
		fmt.Println("cancelled")
	default:
		fmt.Fprintln(os.Stderr, "usage: ryogami upscale start <file> [scale] | status | cancel")
		os.Exit(2)
	}
}

// jsonInt reads a decoded JSON number.
func jsonInt(v interface{}) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}
