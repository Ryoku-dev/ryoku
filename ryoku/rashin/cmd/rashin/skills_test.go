package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/skills"
)

// skillsDaemon answers hello and skills.* from the built-in sets written out
// under a folder of the test's own, with the sets in off turned off.
func skillsDaemon(t *testing.T, off []string) *skills.Library {
	t.Helper()
	dir, err := os.MkdirTemp("", "rashin-cli")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	library := skills.New(filepath.Join(dir, "skills"))
	socket := filepath.Join(dir, "rashin.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	t.Setenv("RASHIN_SOCKET", socket)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				frames := protocol.NewDecoder(conn)
				for {
					frame, err := frames.Next()
					if err != nil || frame.ID == nil {
						return
					}
					var result any = protocol.HelloResult{Daemon: "rashind", Protocol: protocol.Version, Enabled: true}
					var failed error
					switch frame.Method {
					case protocol.MSkillsList:
						result, failed = library.List(off)
					case protocol.MSkillsSearch:
						var p protocol.SkillsSearchParams
						_ = json.Unmarshal(frame.Params, &p)
						result, failed = library.Search(p, off)
					case protocol.MSkillsGet:
						var p protocol.SkillParams
						_ = json.Unmarshal(frame.Params, &p)
						result, failed = library.Get(p, off)
					}
					response := protocol.Response{ID: *frame.ID}
					if failed != nil {
						response.Error = &protocol.Error{Code: protocol.CodeNotFound, Message: failed.Error()}
					} else {
						response.Result, _ = json.Marshal(result)
					}
					line, _ := json.Marshal(response)
					if _, err := conn.Write(append(line, '\n')); err != nil {
						return
					}
				}
			}()
		}
	}()
	return library
}

func TestSkillsListAsksTheDaemon(t *testing.T) {
	skillsDaemon(t, []string{"egm"})
	out, code := captured(t, func() int { return runSkills([]string{"list", "--format", "human"}) })
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	for _, want := range []string{"Superpowers (superpowers)", "superpowers/test-driven-development", "OpenSpec (openspec)", "Spec Kit (speckit)", "Elephant and Goldfish (egm)", "egm/egm-draft", "(when you ask)", "Skills lie in "} {
		if !strings.Contains(out, want) {
			t.Errorf("the list lacks %q:\n%s", want, out)
		}
	}
	if line := strings.SplitN(out[strings.Index(out, "Elephant and Goldfish"):], "\n", 2)[0]; !strings.HasSuffix(line, "off") {
		t.Errorf("a set that is off reads %q", line)
	}
	out, code = captured(t, func() int { return runSkills([]string{"list", "--format", "json"}) })
	var result protocol.SkillsResult
	if code != 0 || json.Unmarshal([]byte(out), &result) != nil || len(result.Sets) != 4 {
		t.Fatalf("json list: exit %d, %s", code, out)
	}
}

func TestSkillsSearchAndReadAskTheDaemon(t *testing.T) {
	skillsDaemon(t, nil)
	out, code := captured(t, func() int {
		return runSkills([]string{"search", "write", "a", "failing", "test", "first", "--format", "human"})
	})
	if code != 0 || !strings.HasPrefix(out, "  superpowers/test-driven-development  ") {
		t.Fatalf("search: exit %d:\n%s", code, out)
	}
	out, code = captured(t, func() int { return runSkills([]string{"read", "superpowers/test-driven-development"}) })
	if code != 0 || !strings.HasPrefix(out, "---\nname: test-driven-development\n") {
		t.Fatalf("read: exit %d:\n%.200s", code, out)
	}
}

func TestSkillsSetupCopiesSpecKitOnceAndRefusesAgain(t *testing.T) {
	library := skillsDaemon(t, nil)
	project := t.TempDir()
	out, code := captured(t, func() int { return runSkills([]string{"setup", "speckit", project}) })
	if code != 0 {
		t.Fatalf("setup: exit %d: %s", code, out)
	}
	root, err := library.Root()
	if err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(root, "speckit", ".specify")
	err = filepath.WalkDir(payload, func(name string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(payload, name)
		want, _ := os.ReadFile(name)
		got, readErr := os.ReadFile(filepath.Join(project, ".specify", rel))
		if readErr != nil || string(got) != string(want) {
			t.Errorf("%s was not copied: %v", rel, readErr)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.Stat(filepath.Join(project, ".specify", "scripts", "bash", "create-new-feature.sh"))
	if err != nil || script.Mode().Perm()&0o100 == 0 {
		t.Fatalf("the scripts cannot run: %v, %v", script, err)
	}

	marker := filepath.Join(project, ".specify", "memory", "mine.md")
	if err := os.WriteFile(marker, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, code := captured(t, func() int { return runSkills([]string{"setup", "speckit", project}) }); code == 0 {
		t.Fatal("a second setup did not refuse")
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "mine" {
		t.Fatalf("the second setup touched the project: %v", err)
	}
	if _, code := captured(t, func() int { return runSkills([]string{"setup", "superpowers", project}) }); code == 0 {
		t.Fatal("a set with no setup was set up")
	}
}

func TestSpecKitSetupRefusesASetThatIsOff(t *testing.T) {
	skillsDaemon(t, []string{"speckit"})
	project := t.TempDir()
	if _, code := captured(t, func() int { return runSkills([]string{"setup", "speckit", project}) }); code == 0 {
		t.Fatal("a set that is off was set up")
	}
	if _, err := os.Stat(filepath.Join(project, ".specify")); !os.IsNotExist(err) {
		t.Fatalf("the project changed: %v", err)
	}
}

func TestEveryAgentIsHandedTheSkillTools(t *testing.T) {
	skillsDaemon(t, nil)
	for _, env := range []map[string]string{
		{"RASHIN_SESSION": "session-1", "RASHIN_CWD": "/work/app"},
		{"RASHIN_SESSION": "session-2", "RASHIN_TEAM": "team-1", "RASHIN_TEAM_TOKEN": "token"},
	} {
		instructions, tools := serverTools(func(key string) string { return env[key] })
		found := map[string]bool{}
		for _, tool := range tools {
			if brief.IsSkillTool(tool.Name) {
				found[tool.Name] = tool.ReadOnly
			}
		}
		if !found[brief.ToolSkills] || !found[brief.ToolSkill] || !strings.Contains(instructions, "skills searches") {
			t.Fatalf("with %v the skill tools are %v", env, found)
		}
	}
	_, tools := serverTools(func(string) string { return "" })
	for _, tool := range tools {
		if tool.Name != brief.ToolSkill {
			continue
		}
		result := tool.Call(context.Background(), json.RawMessage(`{"id":"superpowers/test-driven-development"}`))
		raw, _ := json.Marshal(result)
		if !strings.Contains(string(raw), "SKILL.md of superpowers/test-driven-development") || !strings.Contains(string(raw), "name: test-driven-development") {
			t.Fatalf("the skill tool answered %s", raw)
		}
	}
}
