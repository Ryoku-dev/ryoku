package harness

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// fakeHomeProbeTurn is a team agent turned against its sandbox: it tries to
// read every path in RASHIN_ACP_TEST_READ and to write every one in
// RASHIN_ACP_TEST_WRITE (":"-separated, "~/" read as its HOME), says how each
// went without ever showing what it read, says where HOME is, and reports
// through Rashin's report tool so its task ends.
func fakeHomeProbeTurn(write func(any), servers []fakeMCPServer) {
	home := os.Getenv("HOME")
	expand := func(path string) string {
		if rest, ok := strings.CutPrefix(path, "~/"); ok {
			return filepath.Join(home, rest)
		}
		return path
	}
	var lines []string
	for _, path := range filepath.SplitList(os.Getenv("RASHIN_ACP_TEST_READ")) {
		data, err := os.ReadFile(expand(path))
		switch {
		case err != nil:
			lines = append(lines, "read "+path+": "+errorClass(err))
		default:
			lines = append(lines, "read "+path+": "+strconv.Itoa(len(data))+" bytes")
		}
	}
	for _, path := range filepath.SplitList(os.Getenv("RASHIN_ACP_TEST_WRITE")) {
		target := expand(path)
		err := os.MkdirAll(filepath.Dir(target), 0o700)
		if err == nil {
			err = os.WriteFile(target, []byte("// planted by a team run\n"), 0o600)
		}
		if err != nil {
			lines = append(lines, "write "+path+": "+errorClass(err))
		} else {
			lines = append(lines, "write "+path+": written")
		}
	}
	lines = append(lines, "HOME="+home)
	text := strings.Join(lines, "\n")
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": text}})
	if len(servers) == 0 {
		return
	}
	client, err := startFakeMCP(servers[0])
	if err != nil {
		return
	}
	defer client.close()
	_, _, _ = client.call("report", map[string]any{"title": "What the sandbox let me do", "body": text})
}

// errorClass names a failure without the path or anything read.
func errorClass(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "not there"
	case errors.Is(err, fs.ErrPermission):
		return "permission denied"
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return "failed"
}
