// Package env defines the narrow environment shared by Rashin app pages.
package env

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/conn"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

type Navigate func(tab, session string)
type WorkNavigate func(project, tab, worktree string)

type Env struct {
	Theme        *look.Theme
	Store        *state.Store
	Client       *conn.Client
	Overlays     *look.Overlays
	Navigate     Navigate
	NavigateWork WorkNavigate
	Post         func(func())
}

func (e *Env) Snapshot() state.Snapshot { return e.Store.Snapshot() }

func (e *Env) Call(method string, params, result any, done func(error)) {
	e.Client.GoCall(method, params, result, func(err error) {
		if e.Post != nil {
			e.Post(func() {
				if done != nil {
					done(err)
				}
			})
		} else if done != nil {
			done(err)
		}
	})
}

func (e *Env) Attach(id string, done func(error)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := e.Client.Attach(ctx, id)
		if e.Post != nil {
			e.Post(func() {
				if done != nil {
					done(err)
				}
			})
		}
	}()
}

func (e *Env) SetSettings(patch any, done func(error)) {
	raw, err := json.Marshal(patch)
	if err != nil {
		if done != nil {
			done(err)
		}
		return
	}
	var settings protocol.Settings
	e.Call(protocol.MSettingsSet, protocol.SettingsSetParams{Patch: raw}, &settings, done)
}

// LoadWorktrees refreshes one project's worktree collection.
func (e *Env) LoadWorktrees(project string, done func(error)) {
	var result protocol.WorktreesResult
	e.Call(protocol.MWorktreesList, protocol.ProjectRef{Project: project}, &result, func(err error) {
		if err == nil {
			e.Store.SetWorktrees(result)
		}
		if done != nil {
			done(err)
		}
	})
}

// LoadTasks refreshes one project's task board.
func (e *Env) LoadTasks(project string, done func(error)) {
	var result protocol.TasksResult
	e.Call(protocol.MTasksList, protocol.TasksParams{Project: project}, &result, func(err error) {
		if err == nil {
			e.Store.SetTasks(project, result)
		}
		if done != nil {
			done(err)
		}
	})
}

// LoadReviews refreshes one worktree's review comments.
func (e *Env) LoadReviews(worktree string, done func(error)) {
	var result protocol.ReviewResult
	e.Call(protocol.MReviewList, protocol.WorktreeRef{Path: worktree}, &result, func(err error) {
		if err == nil {
			e.Store.SetReviews(result)
		}
		if done != nil {
			done(err)
		}
	})
}

// WatchProject and UnwatchProject are refcounted by the reconnecting client.
func (e *Env) WatchProject(project string)   { e.Client.WatchProject(project) }
func (e *Env) UnwatchProject(project string) { e.Client.UnwatchProject(project) }

// OnGitChanged registers a project-scoped refresh callback.
func (e *Env) OnGitChanged(project string, callback func()) func() {
	return e.Store.OnGitChanged(project, callback)
}

func HandleChatActions(gtx layout.Context, e *Env, id string, actions []chat.Action) {
	for _, action := range actions {
		switch action.Kind {
		case chat.ActPermission:
			e.Call(protocol.MSessionPermission, protocol.PermissionParams{ID: id, Request: action.Request, Option: action.Option}, new(protocol.Empty), nil)
		case chat.ActCopy:
			gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(action.Value))})
		case chat.ActOpenPath, chat.ActOpenURL:
			spawn("xdg-open", action.Value)
		case chat.ActRunCommand:
			spawn("ryoku-app", "terminal", "--", "sh", "-c", action.Value+`; exec "$SHELL"`)
		case chat.ActHandoff:
			var info protocol.SessionInfo
			e.Call(protocol.MSessionHandoff, protocol.HandoffParams{ID: id, Run: true, Text: action.Value}, &info, func(err error) {
				if err == nil {
					e.Attach(info.ID, func(error) {})
					e.Navigate(protocol.TabAgent, info.ID)
				}
			})
		case chat.ActCancel:
			e.Call(protocol.MSessionCancel, protocol.SessionRef{ID: id}, new(protocol.Empty), nil)
		}
	}
}

func Spawn(name string, args ...string) { spawn(name, args...) }

func spawn(name string, args ...string) {
	go func() {
		command := exec.Command(name, args...)
		_ = command.Start()
		if command.Process != nil {
			_ = command.Process.Release()
		}
	}()
}
