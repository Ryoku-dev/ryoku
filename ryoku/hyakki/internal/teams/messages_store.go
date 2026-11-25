package teams

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Messages have one bounded record per file, not an ever-growing state.json.
// Damaged individual files are ignored like damaged task files; directory and
// mutation errors are returned to the caller.
func (st *state) loadMessages() error {
	dir := filepath.Join(st.dir, "messages")
	if err := paths.EnsureDir(dir); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		file, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		// JSON escaping can expand one text byte into six encoded bytes.
		const maxFile = 6*(maxTeamMessageBody+maxTeamMessageTitle) + 16<<10
		raw, readErr := io.ReadAll(io.LimitReader(file, maxFile+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || len(raw) > maxFile {
			continue
		}
		var message protocol.TeamMessage
		if json.Unmarshal(raw, &message) != nil || !safeID(message.ID) || message.ID+".json" != entry.Name() || message.Created.IsZero() || !validMessageText(message) {
			continue
		}
		st.messageSet[message.ID] = message
	}
	return nil
}

func (st *state) messages(filter func(protocol.TeamMessage) bool) []protocol.TeamMessage {
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]protocol.TeamMessage, 0)
	for _, message := range st.messageSet {
		// TeamMessage is a value-only record: returning it cannot mutate state.
		if filter == nil || filter(message) {
			out = append(out, message)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Created.Equal(out[j].Created) {
			return out[i].Created.After(out[j].Created)
		}
		return out[i].ID > out[j].ID
	})
	return out
}

// appendMessage never overwrites a previously published record, including its
// delivery marker. References are checked under the same lock as the append.
func (st *state) appendMessage(message protocol.TeamMessage) error {
	if !safeID(message.ID) || !validMessageText(message) || message.Created.IsZero() {
		return protocol.Errorf(protocol.CodeBadRequest, "The message is not valid.")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, exists := st.messageSet[message.ID]; exists {
		return protocol.Errorf(protocol.CodeConflict, "This message already exists.")
	}
	team, exists := st.teamSet[message.Team]
	task, taskExists := st.taskSet[message.Task]
	if !exists || !taskExists || task.Team != team.ID {
		return protocol.Errorf(protocol.CodeNotFound, "The message's team or task no longer exists.")
	}
	for _, id := range []string{message.From, message.To, message.Member} {
		if id != "" {
			if _, ok := protocol.MemberByID(team.Members, id); !ok {
				return protocol.Errorf(protocol.CodeNotFound, "The message's member is not on this team.")
			}
		}
	}
	if err := st.writeMessageLocked(message); err != nil {
		return err
	}
	if st.messageSet == nil {
		st.messageSet = make(map[string]protocol.TeamMessage)
	}
	st.messageSet[message.ID] = message
	return nil
}

func (st *state) writeMessageLocked(message protocol.TeamMessage) error {
	raw, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		return err
	}
	return st.writer.write(filepath.Join(st.dir, "messages"), message.ID+".json", append(raw, '\n'))
}

func (st *state) deliverMessage(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	message, exists := st.messageSet[id]
	if !exists {
		return protocol.Errorf(protocol.CodeNotFound, "No message %s.", id)
	}
	if message.Retired {
		return protocol.Errorf(protocol.CodeConflict, "This request was retired without delivery.")
	}
	if message.Delivered {
		return nil
	}
	message.Delivered = true
	if err := st.writeMessageLocked(message); err != nil {
		return err
	}
	st.messageSet[id] = message
	return nil
}

func (st *state) retireMessage(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	message, exists := st.messageSet[id]
	if !exists {
		return protocol.Errorf(protocol.CodeNotFound, "No message %s.", id)
	}
	if message.Delivered || message.Retired {
		return nil
	}
	message.Retired = true
	if err := st.writeMessageLocked(message); err != nil {
		return err
	}
	st.messageSet[id] = message
	return nil
}

func (st *state) removeMessagesLocked(filter func(protocol.TeamMessage) bool) error {
	for id, message := range st.messageSet {
		if !filter(message) {
			continue
		}
		if err := os.Remove(filepath.Join(st.dir, "messages", id+".json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		delete(st.messageSet, id)
	}
	return nil
}

func (st *state) hasChildrenLocked(id string) bool {
	for _, task := range st.taskSet {
		if task.Parent == id {
			return true
		}
	}
	return false
}

// replaceTask rejects stale snapshots and cannot resurrect a deleted task.
// Service.mu serializes policy; this second check protects storage callers too.
func (st *state) replaceTask(expected, replacement protocol.TeamTask) error {
	if !safeID(expected.ID) || expected.ID != replacement.ID {
		return protocol.Errorf(protocol.CodeBadRequest, "A task's identity cannot change.")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	current, exists := st.taskSet[expected.ID]
	if !exists || !reflect.DeepEqual(current, expected) {
		return protocol.Errorf(protocol.CodeConflict, "This task changed while it was being edited.")
	}
	replacement = cloneTask(replacement)
	if current.Started != nil && replacement.Started != nil && current.Started.Equal(*replacement.Started) {
		appendClosedWaits(&replacement, current.PermissionWaits)
	} else if current.Started != nil {
		if err := st.preserveAncestorWaitsLocked(current); err != nil {
			return err
		}
	}
	return st.writeTaskLocked(replacement)
}
