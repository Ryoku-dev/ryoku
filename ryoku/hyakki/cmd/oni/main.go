package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"ryoku-rashin/internal/account"
	"ryoku-rashin/internal/autolearn"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/daemon"
	"ryoku-rashin/internal/decisions"
	"ryoku-rashin/internal/engine"
	engineask "ryoku-rashin/internal/engine/ask"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/engine/tools/agents"
	asktools "ryoku-rashin/internal/engine/tools/ask"
	"ryoku-rashin/internal/engine/tools/askuser"
	"ryoku-rashin/internal/engine/tools/code"
	"ryoku-rashin/internal/engine/tools/fs"
	"ryoku-rashin/internal/engine/tools/goal"
	"ryoku-rashin/internal/engine/tools/learn"
	"ryoku-rashin/internal/engine/tools/rashin"
	"ryoku-rashin/internal/engine/tools/report"
	"ryoku-rashin/internal/engine/tools/shell"
	"ryoku-rashin/internal/engine/tools/todo"
	"ryoku-rashin/internal/engine/tools/web"
	"ryoku-rashin/internal/gateway"
	"ryoku-rashin/internal/grievances"
	"ryoku-rashin/internal/index"
	"ryoku-rashin/internal/index/embed"
	"ryoku-rashin/internal/judge"
	"ryoku-rashin/internal/localmodels"
	"ryoku-rashin/internal/mcpservers"
	"ryoku-rashin/internal/memory"
	"ryoku-rashin/internal/migration"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/redact"
	"ryoku-rashin/internal/rules"
	"ryoku-rashin/internal/sandbox"
	"ryoku-rashin/internal/skills"
	"ryoku-rashin/internal/teams"
	"ryoku-rashin/internal/workspace"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__runtime-mcp" {
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "oni: runtime bridge requires exactly one private socket")
			os.Exit(2)
		}
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		if err := engine.ServeRuntimeMCP(ctx, os.Args[2], os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "oni: runtime bridge:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("Oni %s\n", version)
		return
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: oni [--version]")
		os.Exit(1)
	}
	if err := run(); err != nil {
		logStartup(err)
		os.Exit(1)
	}
}
func run() error {
	// a core dump, or a same-user read of /proc/<pid>/mem, would lift the
	// vault's key and every value the daemon opens
	if _, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_DUMPABLE, 0, 0); errno != 0 {
		return fmt.Errorf("prctl PR_SET_DUMPABLE: %w", errno)
	}
	if err := migration.Ensure(); err != nil {
		return err
	}
	lock, acquired, err := daemon.AcquireLock()
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	owned := true
	defer func() {
		if owned {
			_ = lock.Close()
		}
	}()
	initial, err := config.Load()
	if err != nil {
		return err
	}
	if !initial.Enabled {
		return nil
	}
	settings := func() protocol.Settings {
		s, err := config.Load()
		if err != nil {
			return config.Defaults()
		}
		return s
	}
	gw, err := gateway.Open(paths.StateDir(), settings)
	if err != nil {
		return err
	}
	events := daemon.NewBroadcaster()
	idx, err := index.Open(events, settings)
	if err != nil {
		gw.Close()
		return err
	}
	work, err := workspace.Open(events, gw, settings)
	if err != nil {
		idx.Close()
		gw.Close()
		return err
	}
	crew, err := teams.Open(events, settings)
	if err != nil {
		work.Close()
		idx.Close()
		gw.Close()
		return err
	}
	crew.SetModels(gw.Models)
	// every scrubbed transcript and memory takes out the vault's values and
	// the provider keys by name before the patterns run
	defer redact.RegisterStore(crew.Scrubber, crew.VaultOpen)()
	defer redact.RegisterStore(gw.Scrubber, nil)()
	// memory reaches the daemon's settings once the daemon is made, which
	// it is before memory runs anything
	var d *daemon.Daemon
	models := func(ctx context.Context) []protocol.ProviderInfo {
		providers, _ := gw.Providers(ctx)
		return providers
	}
	providerModels := func(ctx context.Context, provider string) []protocol.ModelInfo {
		result, err := gw.Models(ctx, protocol.ModelsParams{Provider: provider})
		if err != nil {
			return nil
		}
		return result.Models
	}
	mem := memory.Open(memory.Deps{
		Settings:       settings,
		Client:         gw,
		Models:         models,
		ProviderModels: providerModels,
		Broadcast:      events.Broadcast,
		Names:          memoryNames{work: work, teams: crew},
		Embed: func() (memory.Embedder, func(), error) {
			model, err := embed.Load()
			if err != nil {
				return nil, nil, err
			}
			return model, embed.Release, nil
		},
		Engines:   memory.NewEngines(memory.EngineDeps{Gateway: gw, Secrets: crew, Models: models, ProviderModels: providerModels}),
		SetEngine: func(engine string) error { return d.SetMemoryEngine(engine) },
	})
	crew.SetMemory(mem)
	// These constructors do no work while their optional features are off.
	library := skills.New(paths.SkillsDir())
	managed := library.WithManaged(paths.ManagedSkillsDir())
	crew.SetSkills(library)
	local := localmodels.New(settings, events)
	gw.SetLocal(local)
	ryoku := account.New(settings, gw)
	gw.SetHosted(ryoku)
	learning := autolearn.NewObserver(gw, managed, settings, events)
	decisionBank := decisions.New(decisions.Deps{Client: gw, Settings: settings, Broadcast: events})
	verdicts := judge.New(gw, settings)
	rulebook := rules.New(rules.Deps{Client: gw, Settings: settings, Judge: verdicts})
	issues := grievances.Open("")
	registry := tool.NewRegistry()
	for _, group := range [][]tool.Tool{
		fs.Tools(), shell.Tools(), code.Tools(), web.Tools(), asktools.Tools(),
		rashin.Tools(), agents.Tools(), todo.Tools(), askuser.Tools(),
		goal.Tools(), learn.Tools(managed, events), report.Tools(issues, events),
		engine.ContextTools(),
	} {
		registry.Add(group...)
	}
	registry.AddProvider(mcpservers.NewProvider(func() (*mcpservers.Store, error) { return d.MCPStore() }))
	factory := engine.New(engine.Deps{
		Gateway: gw, Index: idx, Memory: mem, Teams: crew, Skills: library,
		Tools: registry,
		Executor: func(info protocol.SessionInfo, spec *sandbox.Spec) exec.Executor {
			if spec == nil {
				return exec.Host(exec.HostOptions{})
			}
			return exec.Sandboxed(func(ctx context.Context) (*os.Process, io.ReadWriteCloser, error) {
				argv, err := d.ToolHostCommand(*spec)
				if err != nil {
					return nil, nil, err
				}
				return exec.StartCommand(ctx, argv)
			})
		},
		Host: func(info protocol.SessionInfo) tool.Host { return d.ToolHost(info) },
		Context: []engine.ContextProvider{
			autolearn.NewSkillsContext(library, settings, crew.SkillsOff),
			decisionBank, rulebook, daemon.MemoryContext(mem),
		},
		Guards:    []engine.StreamGuards{rulebook},
		Observers: []engine.TurnObserver{learning, decisionBank, rulebook},
		Judge:     verdicts,
		Ask:       engineask.New(gw),
	})
	deps := daemon.Deps{
		Gateway: gw, FastDecisions: gw, Asker: factory, Agents: factory, Indexer: idx,
		Work: work, Teams: crew, Memory: mem, Skills: library, Local: local, Account: ryoku,
		ManagedSkills: managed, Decisions: decisionBank, Rules: rulebook, Grievances: issues,
		Version: version, Events: events, Lock: lock,
		CloseEngine:  func() { _ = factory.Close(); learning.Close(); _ = verdicts.Close() },
		LearningBusy: learning.Busy,
	}
	// read now: once the file is replaced the kernel no longer names it
	if program, programErr := os.Executable(); programErr == nil {
		deps.Program = program
	}
	d = daemon.New(deps)
	gw.OnChange(d.ProvidersChanged)
	owned = false
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	return d.Run(ctx)
}

func logStartup(err error) {
	_ = os.MkdirAll(filepath.Dir(paths.DaemonLog()), 0o700)
	if stat, statErr := os.Stat(paths.DaemonLog()); statErr == nil && stat.Size() >= 1<<20 {
		_ = os.Remove(paths.DaemonLog() + ".1")
		_ = os.Rename(paths.DaemonLog(), paths.DaemonLog()+".1")
	}
	f, openErr := os.OpenFile(paths.DaemonLog(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if openErr != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s startup: %v\n", time.Now().Format(time.RFC3339), err)
}
