package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Thin verb wrappers over the concern files (vault.go, index.go, agents.go,
// hermes.go, server.go, setup.go) so main.go never changes shape.

func cmdIndex() error {
	if err := EnsureVault(); err != nil {
		return err
	}
	return Reindex()
}

func cmdWire(agent string) error {
	if err := EnsureVault(); err != nil {
		return err
	}
	switch agent {
	case "":
		n := WireAll() // WireAll also links the ryoku skill everywhere
		if err := wireHermesIfPresent(); err == nil {
			n++
		}
		fmt.Printf("wired %d agents\n", n)
		return nil
	case "hermes":
		if err := WireHermesMemory(); err != nil {
			return err
		}
	default:
		if err := Wire(agent); err != nil {
			return err
		}
	}
	// A single-agent or hermes wire still refreshes the always-created skill
	// links (~/.agents, ~/.hermes, every hermes profile) and prowl's
	// skills for the detected clients, so one wire is a complete inject.
	_, _ = WireSkill()
	wireProwlSkills()
	return nil
}

// cmdPaths prints the agent manifest: every path Rashin exposes plus a
// paste-ready snippet, so any coding agent (supported or not) can be pointed at
// the vault, the skill, and prowl.
func cmdPaths(format string) error {
	m := BuildManifest(LoadConfig())
	switch format {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(m)
	case "snippet":
		fmt.Print(m.Snippet)
		return nil
	}
	fmt.Println("Rashin exposes these to your coding agent:")
	fmt.Println()
	fmt.Printf("  skill   %s\n          %s\n", dashIfEmpty(m.Skill.Path), m.Skill.Desc)
	fmt.Printf("  prowl   %s\n          %s\n", dashIfEmpty(m.Prowl.Path), m.Prowl.Desc)
	fmt.Println("  vault (generated = read-only, refreshed each reindex; yours = write here):")
	for _, v := range m.Vault {
		mark := "+"
		if !v.Exists {
			mark = "-"
		}
		fmt.Printf("    %s %-11s %-10s %s\n", mark, v.Label, "["+v.Owner+"]", v.Path)
	}
	fmt.Println()
	fmt.Println("Agents (one-click wire drops the pointer + skill + prowl):")
	for _, a := range m.Agents {
		fmt.Printf("  %-10s present=%-5v wired=%-5v skill=%-5v %s\n", a.ID, a.Present, a.Wired, a.SkillWired, a.File)
	}
	fmt.Println()
	fmt.Println("Chat backends (Super+S; Hermes recommended):")
	for _, b := range m.ChatBackends {
		tag := ""
		if b.Recommended {
			tag = " (recommended)"
		}
		if b.Active {
			tag += " [active]"
		}
		fmt.Printf("  %-8s available=%-5v%s\n", b.ID, b.Available, tag)
	}
	fmt.Println()
	fmt.Println("Point any other agent at Ryoku by pasting this into its instructions:")
	fmt.Println()
	fmt.Println(m.Snippet)
	return nil
}

// cmdAgent lists agents and chat backends, or sets the chat backend with
// `agent use <id>`.
func cmdAgent(args []string) error {
	if len(args) >= 1 && args[0] == "use" {
		id := ""
		if len(args) >= 2 {
			id = args[1]
		}
		if _, err := setChatAgent(context.Background(), id); err != nil {
			return err
		}
		// Tell a running daemon to drop its live session so the switch takes
		// effect on the very next turn, not only after a restart.
		notifyDaemonChatAgent(LoadConfig().Port, id)
		fmt.Printf("chat backend set to %q\n", id)
		return nil
	}
	if len(args) >= 1 && args[0] == "--json" {
		return json.NewEncoder(os.Stdout).Encode(BuildManifest(LoadConfig()).ChatBackends)
	}
	m := BuildManifest(LoadConfig())
	for _, a := range m.Agents {
		fmt.Printf("agent:   %-10s present=%-5v wired=%-5v skill=%-5v\n", a.ID, a.Present, a.Wired, a.SkillWired)
	}
	for _, b := range m.ChatBackends {
		fmt.Printf("chat:    %-10s available=%-5v recommended=%-5v active=%-5v\n", b.ID, b.Available, b.Recommended, b.Active)
	}
	return nil
}

// dashIfEmpty renders an empty path as "-" (not installed).
func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func cmdUnwire(agent string) error {
	if agent == "" {
		for _, a := range DetectAgents() {
			if a.Wired {
				if err := Unwire(a.ID); err != nil {
					return err
				}
			}
		}
		UnwireSkill()
		return nil
	}
	return Unwire(agent)
}

func cmdStatus(asJSON bool) error {
	st := BuildStatus(LoadConfig())
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(st)
	}
	running := "stopped"
	if st.Running {
		running = fmt.Sprintf("running on http://127.0.0.1:%d", st.Port)
	}
	fmt.Printf("daemon:  %s (enabled: %v)\n", running, st.Enabled)
	fmt.Printf("vault:   %s (%d files)\n", st.Vault.Path, st.Vault.Files)
	fmt.Printf("hermes:  installed=%v configured=%v wired=%v skill=%v %s\n",
		st.Hermes.Installed, st.Hermes.Configured, st.Hermes.Wired, st.Hermes.SkillWired, st.Hermes.Version)
	for _, a := range st.Agents {
		fmt.Printf("agent:   %-10s present=%-5v wired=%-5v skill=%-5v %s\n", a.ID, a.Present, a.Wired, a.SkillWired, a.File)
	}
	return nil
}

func wireHermesIfPresent() error {
	if _, ok := FindHermes(); !ok {
		return errors.New("hermes not installed")
	}
	return WireHermesMemory()
}
