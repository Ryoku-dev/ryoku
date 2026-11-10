package shell

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"ryoku-rashin/internal/app/demo"
	projectspage "ryoku-rashin/internal/app/pages/projects"
	"ryoku-rashin/internal/protocol"
)

var storyboardEpoch = demo.Epoch

type storyboardEvent struct {
	at      int
	command string
	args    []string
}

func readStoryboard(path string) ([]storyboardEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open storyboard: %w", err)
	}
	defer file.Close()
	var events []storyboardEvent
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) < 2 {
			return nil, fmt.Errorf("storyboard line %d: expected <ms> <event>", line)
		}
		at, err := strconv.Atoi(fields[0])
		if err != nil || at < 0 {
			return nil, fmt.Errorf("storyboard line %d: invalid time %q", line, fields[0])
		}
		event := storyboardEvent{at: at, command: strings.ToLower(fields[1]), args: fields[2:]}
		switch event.command {
		case "type":
			parts := strings.SplitN(text, fields[1], 2)
			if len(parts) == 2 {
				event.args = []string{strings.TrimSpace(parts[1])}
			}
		case "click", "secondary":
			// a real press and its release land in different frames, and a menu that opens on release counts on it
			press := storyboardEvent{at: at, command: "press", args: event.args}
			if event.command == "secondary" {
				press.command = "press-secondary"
			}
			events = append(events, press)
			event.command = "release"
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read storyboard: %w", err)
	}
	for index := 1; index < len(events); index++ {
		if events[index].at < events[index-1].at {
			return nil, fmt.Errorf("storyboard event at %d ms is before the previous event", events[index].at)
		}
	}
	return events, nil
}

func applyStoryboardEvent(event storyboardEvent, router *input.Router, shell *uiShell, metricScale float32) error {
	coordinate := func(index int) (float32, error) {
		if index >= len(event.args) {
			return 0, fmt.Errorf("%s needs more coordinates", event.command)
		}
		value, err := strconv.ParseFloat(event.args[index], 32)
		return float32(value) * metricScale, err
	}
	switch event.command {
	case "move", "press", "press-secondary", "release":
		x, err := coordinate(0)
		if err != nil {
			return err
		}
		y, err := coordinate(1)
		if err != nil {
			return err
		}
		pointed := pointer.Event{Source: pointer.Mouse, PointerID: 1, Position: f32.Pt(x, y)}
		switch event.command {
		case "move":
			pointed.Kind = pointer.Move
		case "press":
			pointed.Kind, pointed.Buttons = pointer.Press, pointer.ButtonPrimary
		case "press-secondary":
			pointed.Kind, pointed.Buttons = pointer.Press, pointer.ButtonSecondary
		default:
			pointed.Kind = pointer.Release
		}
		router.Queue(pointed)
	case "scroll":
		x, err := coordinate(0)
		if err != nil {
			return err
		}
		y, err := coordinate(1)
		if err != nil {
			return err
		}
		delta, err := coordinate(2)
		if err != nil {
			return err
		}
		router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, PointerID: 1, Position: f32.Pt(x, y), Scroll: f32.Pt(0, delta)})
	case "key":
		if len(event.args) != 1 {
			return fmt.Errorf("key needs one chord")
		}
		name, modifiers := storyboardKey(event.args[0])
		router.Queue(key.Event{Name: name, Modifiers: modifiers, State: key.Press})
	case "type":
		if len(event.args) != 1 {
			return fmt.Errorf("type needs text")
		}
		router.Queue(key.EditEvent{Text: event.args[0]})
	case "route":
		if len(event.args) != 1 {
			return fmt.Errorf("route needs a route")
		}
		shell.openStoryboardRoute(event.args[0])
	case "wait":
		time.Sleep(50 * time.Millisecond)
		shell.posts.Drain()
	default:
		return fmt.Errorf("unknown storyboard event %q", event.command)
	}
	return nil
}

func storyboardKey(chord string) (key.Name, key.Modifiers) {
	parts := strings.Split(strings.ToLower(chord), "+")
	var modifiers key.Modifiers
	name := parts[len(parts)-1]
	for _, modifier := range parts[:len(parts)-1] {
		switch modifier {
		case "ctrl":
			modifiers |= key.ModShortcut
		case "alt":
			modifiers |= key.ModAlt
		case "shift":
			modifiers |= key.ModShift
		}
	}
	switch name {
	case "escape", "esc":
		return key.NameEscape, modifiers
	case "enter", "return":
		return key.NameReturn, modifiers
	case "up":
		return key.NameUpArrow, modifiers
	case "down":
		return key.NameDownArrow, modifiers
	case "left":
		return key.NameLeftArrow, modifiers
	case "right":
		return key.NameRightArrow, modifiers
	default:
		return key.Name(strings.ToUpper(name)), modifiers
	}
}

func (s *uiShell) openStoryboardRoute(route string) {
	parts := strings.Split(strings.Trim(route, "/"), "/")
	if len(parts) == 0 {
		return
	}
	switch parts[0] {
	case protocol.TabAsk, protocol.TabAgent:
		session := ""
		if len(parts) > 1 && parts[1] != "new" {
			session = parts[1]
		}
		s.navigate(parts[0], session)
	case protocol.TabProjects:
		project, view, worktree := "", projectspage.TabOverview, ""
		if len(parts) > 1 {
			project = parts[1]
		}
		if len(parts) > 2 {
			view = parts[2]
		}
		if len(parts) > 3 {
			worktree = parts[3]
		}
		s.navigateWork(project, view, worktree)
	case protocol.TabIndex:
		s.navigate(parts[0], "")
	case protocol.TabSettings:
		place := Place{Tab: protocol.TabSettings}
		if len(parts) > 1 {
			place.SettingsSection = parts[1]
		}
		s.router.Push(place)
	}
}
