package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	gioapp "gioui.org/app"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/shell"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

var version = "dev"

func main() {
	code, err := execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.Tf("Rashin could not open: %v", err))
	}
	if code != 0 {
		os.Exit(code)
	}
}

func execute() (int, error) {
	pngPath := flag.String("png", "", i18n.T("Render one frame to a PNG and exit"))
	size := flag.String("size", "", i18n.T("Window or PNG size as WIDTHxHEIGHT"))
	demoMode := flag.Bool("demo", false, i18n.T("Use the in-process demo daemon"))
	route := flag.String("route", protocol.TabAsk, i18n.T("Initial route: ask, agent, projects, index or settings"))
	tab := flag.String("tab", "", i18n.T("Initial tab: ask, agent, projects, index or settings"))
	session := flag.String("session", "", i18n.T("Open this session"))
	project := flag.String("project", "", i18n.T("Open this project"))
	view := flag.String("view", "", i18n.T("Project view: overview, graph, changes, tasks or activity"))
	worktree := flag.String("worktree", "", i18n.T("Open this worktree"))
	mode := flag.String("mode", "follow", i18n.T("Palette mode: follow, dark or light"))
	flag.Parse()

	width, height := 0, 0
	if *size != "" {
		var err error
		width, height, err = parseSize(*size)
		if err != nil {
			return 2, err
		}
	}
	target := applyLaunchTarget(parseRouteTarget(*route), *tab, *session, *project, *view, *worktree)
	options := shell.Options{Demo: *demoMode, PNG: *pngPath, Width: width, Height: height, Mode: *mode, Route: target.tab, Session: target.session, Project: target.project, WorkTab: target.workTab, Worktree: target.worktree, SettingsSection: target.settingsSection}
	if options.PNG != "" {
		if !options.Demo {
			options.Demo = true
		}
		if err := shell.Run(options); err != nil {
			return 1, err
		}
		return 0, nil
	}

	lock, err := state.AcquireAppLock()
	if errors.Is(err, state.ErrAppLockHeld) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return 0, openExisting(ctx, protocol.UIOpenParams{Session: target.session, Tab: target.tab, Project: target.project, View: target.workTab, Worktree: target.worktree})
	}
	if err != nil {
		return 1, err
	}
	go exitAfterShell(options, lock)
	gioapp.Main()
	return 0, nil
}

func openExisting(ctx context.Context, params protocol.UIOpenParams) error {
	connection, err := client.Dial(ctx, client.Options{Client: protocol.ClientApp, Start: true})
	if err != nil {
		return err
	}
	defer connection.Close()
	return sendOpen(ctx, params, connection.Call)
}

func sendOpen(ctx context.Context, params protocol.UIOpenParams, call func(context.Context, string, any, any) error) error {
	var result protocol.UIOpenResult
	return call(ctx, protocol.MUIOpen, params, &result)
}

func exitAfterShell(options shell.Options, lock *state.AppLock) {
	err := shell.Run(options)
	if closeErr := lock.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, i18n.Tf("Rashin could not open: %v", err))
		os.Exit(1)
	}
	os.Exit(0)
}

type routeTarget struct {
	tab, session, project, workTab, worktree string
	settingsSection                          int
}

func applyLaunchTarget(target routeTarget, tab, session, project, view, worktree string) routeTarget {
	if tab != "" {
		target.tab = tab
	}
	if session != "" {
		target.session = session
	}
	if project != "" {
		target.project = project
	}
	if view != "" {
		target.workTab = view
	}
	if worktree != "" {
		target.worktree = worktree
	}
	if target.project != "" {
		target.tab = protocol.TabProjects
		target.session = ""
		if target.workTab == "" {
			target.workTab = "overview"
		}
	}
	return target
}

func parseRouteTarget(value string) routeTarget {
	target := routeTarget{tab: protocol.TabAsk, settingsSection: -1}
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return target
	}
	target.tab = parts[0]
	if target.tab == "agents" {
		target.tab = protocol.TabAgent
	}
	switch target.tab {
	case protocol.TabAsk, protocol.TabAgent:
		if len(parts) > 1 {
			target.session = parts[1]
		}
	case protocol.TabProjects:
		if len(parts) > 1 {
			target.project = parts[1]
		}
		if len(parts) > 2 {
			target.workTab = parts[2]
		}
		if len(parts) > 3 {
			target.worktree = parts[3]
		}
	case protocol.TabSettings:
		target.settingsSection = settingsSection(parts)
	}
	return target
}

func settingsSection(parts []string) int {
	if len(parts) < 2 {
		return 0
	}
	switch parts[1] {
	case "providers":
		return 0
	case "ask":
		return 1
	case "agent", "agents":
		return 2
	case "index", "projects":
		return 3
	case "about":
		return 4
	default:
		return 0
	}
}

func parseRoute(value string) (string, string) {
	target := parseRouteTarget(value)
	return target.tab, target.session
}

func parseSize(value string) (int, int, error) {
	widthText, heightText, ok := strings.Cut(strings.ToLower(strings.TrimSpace(value)), "x")
	if !ok {
		return 0, 0, fmt.Errorf("invalid --size %q: use WIDTHxHEIGHT", value)
	}
	width, widthErr := strconv.Atoi(widthText)
	height, heightErr := strconv.Atoi(heightText)
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("invalid --size %q: use positive dimensions", value)
	}
	return width, height, nil
}
