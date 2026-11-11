// Package env defines the narrow environment shared by Rashin app pages.
package env

import (
	"context"
	"encoding/json"
	"io"
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
type TeamsNavigate func(place TeamsPlace)

// Env is what the frame hands a page each frame: the theme, the store, the
// daemon client, and the few frame facilities a page may use.
type Env struct {
	Theme        *look.Theme
	Store        *state.Store
	Client       *conn.Client
	Overlays     *look.Overlays
	Navigate     Navigate
	NavigateWork WorkNavigate
	// NavigateTeams opens a place in Teams mode, pushing it on the history.
	NavigateTeams TeamsNavigate
	// NewTeamTask opens the dialog that gives a team a task, with team picked
	// when it is not empty.
	NewTeamTask func(team string)
	// OpenSettings shows one section of Settings, named as in its index.
	OpenSettings func(section string)
	Post         func(func())
	Class        look.Class
	// PanelOpen says whether the side panel is shown; TogglePanel flips it.
	PanelOpen   bool
	TogglePanel func()
	// Notify shows a transient result at the foot of the content pane.
	Notify func(text string)
	// Zoom is the app's zoom in percent and SetZoom changes it; ReduceMotion
	// and SetReduceMotion do the same for the app's own motion setting.
	Zoom            int
	SetZoom         func(percent int)
	ReduceMotion    bool
	SetReduceMotion func(bool)
	// Contrast and Tint are the look's stored words ("standard" or "high",
	// "off", "low" or "medium"); SetLook changes both and restyles the window
	// at once.
	Contrast string
	Tint     string
	SetLook  func(contrast, tint string)
	// Notices says whether the desktop is told when an agent needs the reader.
	Notices    bool
	SetNotices func(bool)
	Desktop    Desktop
	// HiddenSteps are the next steps the user hid on each team's Overview,
	// step keys by team id; HideStep hides one more.
	HiddenSteps map[string][]string
	HideStep    func(team, step string)
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

// LoadTeamTasks loads a team's tasks, or every team's when team is empty,
// older than before when it is set, into the store.
func (e *Env) LoadTeamTasks(team, before string, done func(error)) {
	var result protocol.TeamTasksResult
	e.Call(protocol.MTeamsTasks, protocol.TeamTasksParams{Team: team, Before: before, Limit: 50}, &result, func(err error) {
		if err == nil {
			e.Store.SetTeamTasks(result)
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
		case chat.ActOpenPath:
			e.OpenPath(action.Value)
		case chat.ActOpenURL:
			e.Open(action.Value)
		case chat.ActRunCommand:
			e.RunInTerminal(action.Value)
		case chat.ActRunHere:
			e.runHere(id, action.Value)
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
		case chat.ActRetry:
			if action.Value != "" || len(action.Attached) > 0 {
				e.Submit(id, false, chat.Submission{Text: action.Value, Images: action.Attached})
			}
		case chat.ActUnqueue:
			e.Call(protocol.MSessionUnqueue, protocol.UnqueueParams{ID: id, Queue: action.Value}, new(protocol.SessionInfo), nil)
		}
	}
}

// Report shows a refused call's error at the foot of the content pane; pass
// e.Report as a call's done when only a refusal needs saying.
func (e *Env) Report(err error) {
	if err != nil && e.Notify != nil {
		e.Notify(err.Error())
	}
}

// Submit sends what the composer produced: a prompt to an idle session, a
// queued follow-up to a busy one, or a steer that replaces the running turn.
func (e *Env) Submit(id string, busy bool, submission chat.Submission) {
	report := e.Report
	images := make([]protocol.Attachment, len(submission.Images))
	for index, path := range submission.Images {
		images[index] = protocol.Attachment{Path: path}
	}
	if busy && !submission.Steer {
		e.Call(protocol.MSessionQueue, protocol.QueueParams{ID: id, Text: submission.Text, Images: images}, new(protocol.SessionInfo), report)
		return
	}
	method := protocol.MSessionPrompt
	if submission.Steer {
		method = protocol.MSessionSteer
	}
	e.Call(method, protocol.PromptParams{ID: id, Text: submission.Text, Images: images}, new(protocol.PromptResult), report)
}
