package teams

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// The team is paused before tasks are seeded: these tests exercise storage and
// control transitions without starting workers, models or external processes.
func taskControlsFixture(t *testing.T) (*harness, protocol.Team, protocol.TeamTask) {
	t.Helper()
	h := newHarness(t, 0)
	team := h.newTeam(t, "Controls", nil)
	team.Paused = true
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	task := protocol.TeamTask{ID: "task-controls", Team: team.ID, Member: team.Members[0].ID, Title: "Root", Brief: "Work", Goal: "Keep the goal", Status: protocol.TaskReady, Created: at, Updated: at}
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	return h, team, task
}

func messageForTask(task protocol.TeamTask, id string) protocol.TeamMessage {
	return protocol.TeamMessage{ID: id, Team: task.Team, Task: task.ID, Kind: "comment", Title: "Note", Body: id, Created: task.Created}
}

func controlsErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var problem *protocol.Error
	if !errors.As(err, &problem) || problem.Code != code {
		t.Fatalf("got %v, want %s", err, code)
	}
}

func TestTeamMessagePagesSurviveTimestampTiesAndDeletedBoundary(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	for _, id := range []string{"message-a", "message-c", "message-b"} {
		if err := h.s.state.appendMessage(messageForTask(task, id)); err != nil {
			t.Fatal(err)
		}
	}
	page, err := h.s.teamMessages(protocol.TeamMessagesParams{Team: team.ID, Task: task.ID, Limit: 1})
	if err != nil || len(page.Messages) != 1 || page.Messages[0].ID != "message-c" || page.Next == "" {
		t.Fatalf("first page = %+v, %v", page, err)
	}
	// New records ahead of the cursor must not shift the next page. The cursor
	// is self-contained and remains usable after its boundary record is gone.
	if err := h.s.state.appendMessage(messageForTask(task, "message-d")); err != nil {
		t.Fatal(err)
	}
	h.s.state.mu.Lock()
	err = h.s.state.removeMessagesLocked(func(message protocol.TeamMessage) bool { return message.ID == "message-c" })
	h.s.state.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	next, err := h.s.teamMessages(protocol.TeamMessagesParams{Team: team.ID, Task: task.ID, Before: page.Next, Limit: 2})
	if err != nil || len(next.Messages) != 2 || next.Messages[0].ID != "message-b" || next.Messages[1].ID != "message-a" || next.Next != "" {
		t.Fatalf("next page = %+v, %v", next, err)
	}
	_, err = h.s.teamMessages(protocol.TeamMessagesParams{Team: team.ID, Before: page.Next})
	controlsErrorCode(t, err, protocol.CodeBadRequest)
	_, err = h.s.teamMessages(protocol.TeamMessagesParams{Before: "not-a-cursor"})
	controlsErrorCode(t, err, protocol.CodeBadRequest)
}

func TestTeamMessageDeliveryIsDurableAndRejectsStaleAppend(t *testing.T) {
	h, _, task := taskControlsFixture(t)
	message := messageForTask(task, "message-durable")
	if err := h.s.state.appendMessage(message); err != nil {
		t.Fatal(err)
	}
	if err := h.s.state.deliverMessage(message.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.s.state.deliverMessage(message.ID); err != nil {
		t.Fatal(err)
	}
	message.Body = "stale overwrite"
	controlsErrorCode(t, h.s.state.appendMessage(message), protocol.CodeConflict)
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	messages := reopened.messages(nil)
	if len(messages) != 1 || !messages[0].Delivered || messages[0].Body != "message-durable" {
		t.Fatalf("restarted messages = %+v", messages)
	}
	messages[0].Body = "mutated snapshot"
	if reopened.messages(nil)[0].Body != "message-durable" {
		t.Fatal("snapshot mutation reached the store")
	}
	data, err := os.ReadFile(filepath.Join(h.dir, "state.json"))
	if err != nil || strings.Contains(string(data), "message-durable") {
		t.Fatalf("message was embedded in state.json: %s, %v", data, err)
	}
	info, err := os.Stat(filepath.Join(h.dir, "messages", message.ID+".json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("message permissions = %v, %v", info, err)
	}
	if err := reopened.removeTask(task.ID); err != nil {
		t.Fatal(err)
	}
	controlsErrorCode(t, reopened.deliverMessage(message.ID), protocol.CodeNotFound)
	restarted, err := openState(h.dir)
	if err != nil || len(restarted.messages(nil)) != 0 {
		t.Fatalf("removed messages returned after restart: %v", err)
	}
}

func TestTeamMessageQueryLimitsAndReferences(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	h.s.state.mu.Lock()
	for i := range 205 {
		id := fmt.Sprintf("message-%04d", i)
		h.s.state.messageSet[id] = messageForTask(task, id)
	}
	h.s.state.mu.Unlock()
	for _, test := range []struct{ limit, want int }{{0, 50}, {1000, 200}} {
		result, err := h.s.teamMessages(protocol.TeamMessagesParams{Task: task.ID, Limit: test.limit})
		if err != nil || len(result.Messages) != test.want || result.Next == "" {
			t.Fatalf("limit %d: %+v, %v", test.limit, result, err)
		}
	}
	_, err := h.s.teamMessages(protocol.TeamMessagesParams{Team: "team-absent"})
	controlsErrorCode(t, err, protocol.CodeNotFound)
	_, err = h.s.teamMessages(protocol.TeamMessagesParams{Task: "task-absent"})
	controlsErrorCode(t, err, protocol.CodeNotFound)
	other := cloneTeam(team)
	other.ID = "team-other"
	if err := h.s.state.putTeam(other); err != nil {
		t.Fatal(err)
	}
	_, err = h.s.teamMessages(protocol.TeamMessagesParams{Team: other.ID, Task: task.ID})
	controlsErrorCode(t, err, protocol.CodeBadRequest)
	for _, alter := range []func(*protocol.TeamMessage){
		func(message *protocol.TeamMessage) { message.Team = other.ID },
		func(message *protocol.TeamMessage) { message.From = "absent" },
		func(message *protocol.TeamMessage) { message.To = "absent" },
		func(message *protocol.TeamMessage) { message.Member = "absent" },
	} {
		message := messageForTask(task, "message-invalid-ref")
		alter(&message)
		_, err := h.s.recordTeamMessage(context.Background(), message)
		controlsErrorCode(t, err, protocol.CodeNotFound)
	}
}

func TestTaskCommentsAreCanonicalBoundedAndDoNotReassign(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	message, err := h.s.taskComment(context.Background(), protocol.TeamTaskCommentParams{ID: task.ID, Body: "A note from the Summoner"})
	if err != nil || message.Kind != "comment" || message.From != "" || message.To != "" || message.ID == "" || message.Created.IsZero() || message.SourceSession != "" || message.Retired {
		t.Fatalf("comment = %+v, %v", message, err)
	}
	current, _ := h.s.state.task(task.ID)
	if !reflect.DeepEqual(current, task) {
		t.Fatalf("comment changed its task: %+v", current)
	}
	for _, body := range []string{" ", strings.Repeat("x", maxTeamMessageBody+1), string([]byte{0xff})} {
		_, err := h.s.taskComment(context.Background(), protocol.TeamTaskCommentParams{ID: task.ID, Body: body})
		controlsErrorCode(t, err, protocol.CodeBadRequest)
	}
	input := protocol.TeamMessage{ID: "forged", Team: team.ID, Task: task.ID, Kind: "help", From: team.Members[0].ID, To: team.Members[0].ID, Member: team.Members[1].ID, Title: "Help", Body: "Request, not an assignment", Created: time.Unix(1, 0), Delivered: true, SourceSession: "forged-source", Retired: true}
	routed, err := h.s.recordTeamMessage(context.Background(), input)
	if err != nil || routed.ID == input.ID || routed.Created.Equal(input.Created) || routed.Delivered || routed.SourceSession != "" || routed.Retired {
		t.Fatalf("canonical message = %+v, %v", routed, err)
	}
	current, _ = h.s.state.task(task.ID)
	if current.Member != task.Member {
		t.Fatal("requested peer was assigned directly")
	}
}

func TestTaskControlOwnerMethodsAreNotAgentRPCs(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	ctx := WithAgentSession(context.Background(), "session-untrusted")
	for _, test := range []struct {
		method string
		params any
	}{
		{protocol.MTeamsMessages, protocol.TeamMessagesParams{Task: task.ID}},
		{protocol.MTeamsTaskComment, protocol.TeamTaskCommentParams{ID: task.ID, Body: "forged comment"}},
		{protocol.MTeamsTaskReassign, protocol.TeamTaskReassignParams{ID: task.ID, Member: team.Members[1].ID}},
	} {
		raw, err := json.Marshal(test.params)
		if err != nil {
			t.Fatal(err)
		}
		_, err = h.s.Handle(ctx, test.method, raw)
		controlsErrorCode(t, err, protocol.CodeForbidden)
	}
	current, _ := h.s.state.task(task.ID)
	if !reflect.DeepEqual(current, task) || len(h.s.state.messages(nil)) != 0 {
		t.Fatal("rejected owner RPC mutated state")
	}
}

// Block the destination with a directory, independent of uid or chmod behavior.
func blockMessageWrite(t *testing.T, dir, id string) {
	t.Helper()
	path := filepath.Join(dir, "messages", id+".json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestMessageIOFailuresDoNotPublishOrMarkDelivery(t *testing.T) {
	h, _, task := taskControlsFixture(t)
	message := messageForTask(task, "message-io")
	blockMessageWrite(t, h.dir, message.ID)
	if err := h.s.state.appendMessage(message); err == nil || len(h.s.state.messages(nil)) != 0 {
		t.Fatalf("failed append published state: %v", err)
	}
	if err := os.Remove(filepath.Join(h.dir, "messages", message.ID+".json")); err != nil {
		t.Fatal(err)
	}
	if err := h.s.state.appendMessage(message); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(h.dir, "messages", message.ID+".json")); err != nil {
		t.Fatal(err)
	}
	blockMessageWrite(t, h.dir, message.ID)
	if err := h.s.state.deliverMessage(message.ID); err == nil || h.s.state.messages(nil)[0].Delivered {
		t.Fatalf("failed delivery marked state: %v", err)
	}
	controlsErrorCode(t, h.s.state.deliverMessage("message-absent"), protocol.CodeNotFound)
}

func TestMessagesFollowActualRemovalAndKeepParentage(t *testing.T) {
	h, team, root := taskControlsFixture(t)
	child := cloneTask(root)
	child.ID, child.Parent = "task-child", root.ID
	if err := h.s.state.putTask(child); err != nil {
		t.Fatal(err)
	}
	for _, task := range []protocol.TeamTask{root, child} {
		if err := h.s.state.appendMessage(messageForTask(task, "message-"+task.ID)); err != nil {
			t.Fatal(err)
		}
	}
	controlsErrorCode(t, h.s.state.removeTask(root.ID), protocol.CodeConflict)
	if len(h.s.state.messages(nil)) != 2 {
		t.Fatal("failed parent removal lost comments")
	}
	if err := h.s.state.removeTask(child.ID); err != nil {
		t.Fatal(err)
	}
	if messages := h.s.state.messages(nil); len(messages) != 1 || messages[0].Task != root.ID {
		t.Fatalf("child removal deleted unrelated messages: %+v", messages)
	}
	if err := h.s.state.removeTeam(team.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err := openState(h.dir)
	if err != nil || len(reopened.messages(nil)) != 0 {
		t.Fatalf("team removal failed to prune durable messages: %v", err)
	}
}

func TestMessageLoadToleratesDamagedRecordsButNotDirectoryErrors(t *testing.T) {
	h, _, task := taskControlsFixture(t)
	message := messageForTask(task, "message-good")
	if err := h.s.state.appendMessage(message); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(h.dir, "messages")
	if err := os.WriteFile(filepath.Join(dir, "message-broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	oversized := messageForTask(task, "message-oversized")
	oversized.Body = strings.Repeat("x", maxTeamMessageBody+1)
	if err := os.WriteFile(filepath.Join(dir, oversized.ID+".json"), mustJSON(t, oversized), 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := openState(h.dir)
	if err != nil || len(reopened.messages(nil)) != 1 || reopened.messages(nil)[0].ID != message.ID {
		t.Fatalf("damaged records hid a healthy message: %v", err)
	}
	if err := os.Rename(dir, dir+"-saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openState(h.dir); err == nil {
		t.Fatal("message directory error was hidden")
	}
}
