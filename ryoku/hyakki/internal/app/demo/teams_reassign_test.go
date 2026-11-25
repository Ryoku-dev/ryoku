package demo

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

func demoReassignState(t *testing.T, s *server) []byte {
	t.Helper()
	state := map[string]any{
		"teams": s.listTeams(), "tasks": s.data.teams.tasks, "approvals": s.data.teams.approvals,
		"sessions": s.data.teams.sessions, "events": s.data.events, "options": s.data.options,
		"messages": s.data.teams.messages, "activity": s.data.teams.activity, "told": s.data.teams.told,
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func demoReassignAnnouncements(t *testing.T, s *server) []protocol.Event {
	t.Helper()
	var wire bytes.Buffer
	s.announce(&outbox{encoder: protocol.NewEncoder(&wire)}, protocol.MTeamsTaskReassign)
	decoder := json.NewDecoder(&wire)
	var events []protocol.Event
	for {
		var event protocol.Event
		if err := decoder.Decode(&event); err == io.EOF {
			return events
		} else if err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
}

func demoReassignArchive(t *testing.T, s *server, task protocol.TeamTask) protocol.TeamTask {
	t.Helper()
	query := protocol.TeamMessagesParams{Team: task.Team, Task: task.ID, Limit: 1}
	var messages []protocol.TeamMessage
	seen := make(map[string]bool)
	for {
		page := call(t, s, protocol.MTeamsMessages, query).(protocol.TeamMessagesResult)
		for _, message := range page.Messages {
			if message.Kind != "reassign" || message.Task != task.ID || message.Team != task.Team || seen[message.ID] || len(message.Body) > 64<<10 || !utf8.ValidString(message.Body) {
				t.Fatalf("invalid or repeated execution history: %+v", message)
			}
			seen[message.ID] = true
			messages = append(messages, message)
		}
		if page.Next == "" {
			break
		}
		if page.Next == query.Before {
			t.Fatal("history cursor did not advance")
		}
		query.Before = page.Next
	}
	slices.SortFunc(messages, func(a, b protocol.TeamMessage) int { return strings.Compare(a.ID, b.ID) })
	var body strings.Builder
	for _, message := range messages {
		body.WriteString(message.Body)
	}
	var archived protocol.TeamTask
	if err := json.Unmarshal([]byte(body.String()), &archived); err != nil {
		t.Fatal(err)
	}
	return archived
}

func TestDemoTaskReassignValidatesBeforeChangingWork(t *testing.T) {
	for _, assignee := range []string{"", "not-a-member", "member-curator", "Researcher"} {
		t.Run(assignee, func(t *testing.T) {
			s := &server{data: makeFixtures()}
			before := demoReassignState(t, s)
			_, err := s.call(protocol.MTeamsTaskReassign, marshal(protocol.TeamTaskReassignParams{ID: "task-heartbeat", Member: assignee}))
			if err == nil {
				t.Fatal("accepted a non-current member")
			}
			if after := demoReassignState(t, s); !bytes.Equal(after, before) {
				t.Fatal("rejected reassignment stopped work, altered history or published state")
			}
		})
	}
	t.Run("missing team", func(t *testing.T) {
		s := &server{data: makeFixtures()}
		index := s.data.teams.taskIndex("task-heartbeat")
		s.data.teams.tasks[index].Team = "deleted-team"
		before := demoReassignState(t, s)
		_, err := s.call(protocol.MTeamsTaskReassign, marshal(protocol.TeamTaskReassignParams{ID: "task-heartbeat", Member: "member-editor"}))
		if err == nil || !bytes.Equal(demoReassignState(t, s), before) {
			t.Fatal("missing-team reassignment changed existing work")
		}
	})
}

func TestDemoTaskReassignStopsFamilyAndPublishesCurrentPolicy(t *testing.T) {
	s := &server{data: makeFixtures()}
	data := &s.data.teams
	rootIndex := data.taskIndex("task-heartbeat-docs")
	root := data.tasks[rootIndex]
	root.Goal = "Preserve the original comparison goal"
	root.Plan = &protocol.TeamPlan{Summary: "Prior approved strategy", Steps: []string{"Inspect", "Report"}, Revision: 7, Approved: true}
	root.Model, root.Options = demoPaidID, map[string]string{protocol.OptionEffort: "low"}
	data.tasks[rootIndex] = root
	old := call(t, s, protocol.MSessionAttach, protocol.SessionAttachParams{ID: root.Session}).(protocol.SessionAttachResult)

	child := call(t, s, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: root.Team, Member: root.Member, Brief: "Delegated open work"}).(protocol.TeamTask)
	data.tasks[data.taskIndex(child.ID)].Parent = root.ID
	finishedIndex := data.taskIndex("task-heartbeat-cron")
	data.tasks[finishedIndex].Parent = root.ID
	finished := data.tasks[finishedIndex]
	grandchild := call(t, s, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: root.Team, Member: root.Member, Brief: "Open work below a finished intermediate"}).(protocol.TeamTask)
	data.tasks[data.taskIndex(grandchild.ID)].Parent = finished.ID
	for _, task := range []protocol.TeamTask{root, child, grandchild} {
		data.approvals = append(data.approvals, protocol.Approval{ID: "old-" + task.ID, Team: root.Team, Task: task.ID, Status: protocol.ApprovalPending})
	}
	data.approvals = append(data.approvals, protocol.Approval{ID: "previous-decision", Team: root.Team, Task: root.ID, Status: protocol.ApprovalApproved, Decision: protocol.DecideOnce})
	teamIndex := data.teamIndex(root.Team)
	team := &data.teams[teamIndex]
	team.Approvals, team.Budget.TaskTurns = protocol.ApprovalsAuto, 6
	for i := range team.Members {
		if team.Members[i].ID == "member-editor" {
			team.Members[i].Model, team.Members[i].Options = demoFreeID, map[string]string{protocol.OptionEffort: "high"}
			team.Members[i].Trust = protocol.ApprovalsReadOnly
			team.Members[i].Budget = &protocol.TeamBudget{TaskTurns: 3, MonthTokens: 1000}
		}
	}
	policy, err := json.Marshal(*team)
	if err != nil {
		t.Fatal(err)
	}
	s.takeTeamsEvents()
	replacement := call(t, s, protocol.MTeamsTaskReassign, protocol.TeamTaskReassignParams{ID: root.ID, Member: "member-editor"}).(protocol.TeamTask)
	if replacement.ID != root.ID || replacement.Parent != root.Parent || replacement.Goal != root.Goal || replacement.Title != root.Title || replacement.Brief != root.Brief || !replacement.Created.Equal(root.Created) {
		t.Fatalf("reassignment changed task identity or original purpose: %+v", replacement)
	}
	if replacement.Status != protocol.TaskWorking || replacement.Member != "member-editor" || replacement.Session == root.Session || replacement.Model != demoFreeID || replacement.Options[protocol.OptionEffort] != "high" || replacement.MaxTurns != 3 || replacement.Runtime != protocol.MemberRuntimeNative || replacement.Plan != nil {
		t.Fatalf("replacement carried old execution policy or strategy: %+v", replacement)
	}
	wantSessions := append(append([]string(nil), root.Sessions...), replacement.Session)
	if !slices.Equal(replacement.Sessions, wantSessions) {
		t.Fatalf("conversation history = %v, want %v", replacement.Sessions, wantSessions)
	}
	current := call(t, s, protocol.MSessionAttach, protocol.SessionAttachParams{ID: replacement.Session}).(protocol.SessionAttachResult)
	if current.Info.Member != replacement.Member || current.Info.Approvals != protocol.ApprovalsReadOnly || current.Info.Mode != protocol.ApprovalsReadOnly || current.Info.Goal == nil || current.Info.Goal.MaxTurns != 3 {
		t.Fatalf("new conversation does not expose the current member ceiling: %+v", current.Info)
	}
	prior := call(t, s, protocol.MSessionAttach, protocol.SessionAttachParams{ID: root.Session}).(protocol.SessionAttachResult)
	if prior.Info.Alive || prior.Info.State != protocol.StateStopped || len(prior.Events) < len(old.Events) || !reflect.DeepEqual(prior.Events[:len(old.Events)], old.Events) {
		t.Fatal("old conversation was left running or its transcript was discarded")
	}
	for _, task := range []protocol.TeamTask{child, grandchild} {
		stopped := call(t, s, protocol.MTeamsTaskRead, protocol.TeamTaskRef{ID: task.ID}).(protocol.TeamTask)
		conversation := call(t, s, protocol.MSessionAttach, protocol.SessionAttachParams{ID: task.Session}).(protocol.SessionAttachResult)
		if stopped.Status != protocol.TaskStopped || stopped.Ended == nil || conversation.Info.Alive || conversation.Info.State != protocol.StateStopped {
			t.Fatalf("descendant execution survived reassignment: %+v", stopped)
		}
	}
	if !reflect.DeepEqual(data.tasks[data.taskIndex(finished.ID)], finished) {
		t.Fatal("finished descendant report/history was changed")
	}
	for _, approval := range data.approvals {
		if strings.HasPrefix(approval.ID, "old-") && (approval.Status != protocol.ApprovalDenied || approval.Decision != protocol.DecideDeny || approval.Decided == nil) {
			t.Fatalf("old action approval survived: %+v", approval)
		}
		if approval.ID == "previous-decision" && (approval.Status != protocol.ApprovalApproved || approval.Decision != protocol.DecideOnce) {
			t.Fatal("prior approval decision was erased")
		}
	}
	if _, err := s.call(protocol.MTeamsApprovalDecide, marshal(protocol.ApprovalDecideParams{ID: "old-" + root.ID, Decision: protocol.DecideOnce})); err == nil {
		t.Fatal("denied old execution approval could be reused")
	}
	archived := demoReassignArchive(t, s, root)
	if archived.Member != root.Member || archived.Goal != root.Goal || archived.Usage != root.Usage || !reflect.DeepEqual(archived.Plan, root.Plan) || !slices.Equal(archived.Sessions, root.Sessions) {
		t.Fatalf("previous execution details were lost: %+v", archived)
	}
	var published protocol.TeamTask
	var publishedTeam protocol.Team
	stoppedChildren := make(map[string]bool)
	denied := make(map[string]bool)
	for _, event := range demoReassignAnnouncements(t, s) {
		switch event.Event {
		case protocol.EvtTeamTask:
			var task protocol.TeamTask
			if err := json.Unmarshal(event.Data, &task); err != nil {
				t.Fatal(err)
			}
			if task.Status == protocol.TaskStopped {
				stoppedChildren[task.ID] = true
			}
			if task.ID == root.ID && task.Member == replacement.Member {
				if !stoppedChildren[child.ID] || !stoppedChildren[grandchild.ID] || !denied["old-"+root.ID] || !denied["old-"+child.ID] || !denied["old-"+grandchild.ID] {
					t.Fatal("replacement was published before descendants and approvals stopped")
				}
				published = task
			}
		case protocol.EvtApproval:
			var approval protocol.Approval
			if err := json.Unmarshal(event.Data, &approval); err != nil {
				t.Fatal(err)
			}
			denied[approval.ID] = approval.Status == protocol.ApprovalDenied
		case protocol.EvtTeam:
			var team protocol.Team
			if err := json.Unmarshal(event.Data, &team); err != nil {
				t.Fatal(err)
			}
			if team.ID == root.Team {
				publishedTeam = team
			}
		}
	}
	publishedRaw, _ := json.Marshal(published)
	replacementRaw, _ := json.Marshal(replacement)
	if !bytes.Equal(publishedRaw, replacementRaw) || publishedTeam.Stats.Doing[replacement.Member] != replacement.Title {
		t.Fatal("UI did not receive the current replacement task and team work state")
	}
	currentPolicy, err := json.Marshal(data.teams[teamIndex])
	if err != nil || !bytes.Equal(currentPolicy, policy) {
		t.Fatal("reassignment overwrote team/member tools, trust or budget ceilings")
	}
}

func TestDemoTaskReassignPreservesResultAndGoalWhilePaused(t *testing.T) {
	s := &server{data: makeFixtures()}
	call(t, s, protocol.MTeamsPause, protocol.TeamPauseParams{ID: demoTeamResearch, Paused: true})
	created := call(t, s, protocol.MTeamsTaskCreate, protocol.TeamTaskCreateParams{Team: demoTeamResearch, Brief: "Keep this creation-time purpose"}).(protocol.TeamTask)
	teamIndex := s.data.teams.teamIndex(created.Team)
	originalPurpose := s.data.teams.teams[teamIndex].Purpose
	if created.Goal != originalPurpose {
		t.Fatalf("task did not capture its creation-time goal: %+v", created)
	}
	s.data.teams.teams[teamIndex].Purpose = "A different current team purpose"
	index := s.data.teams.taskIndex("task-heartbeat-cron")
	before := s.data.teams.tasks[index]
	before.Goal = created.Goal
	before.Plan = &protocol.TeamPlan{Summary: "Finished strategy", Steps: []string{"Read", "Write"}, Revision: 4, Approved: true}
	before.Report = &protocol.TeamReport{Title: "Full prior result", Body: strings.Repeat("界", 64<<10), Files: []string{"result.txt"}, At: before.Created}
	before.Usage.Input = 1 << 54
	s.data.teams.tasks[index] = before
	old := call(t, s, protocol.MSessionAttach, protocol.SessionAttachParams{ID: before.Session}).(protocol.SessionAttachResult)
	replacement := call(t, s, protocol.MTeamsTaskReassign, protocol.TeamTaskReassignParams{ID: before.ID, Member: "member-editor"}).(protocol.TeamTask)
	if replacement.Status != protocol.TaskReady || replacement.Session != "" || replacement.Started != nil || replacement.Ended != nil || replacement.Plan != nil || replacement.Report != nil || replacement.Model != "" || replacement.Options != nil || replacement.Goal != originalPurpose || replacement.Parent != before.Parent || !slices.Equal(replacement.Sessions, before.Sessions) {
		t.Fatalf("paused replacement started or lost its original goal/history: %+v", replacement)
	}
	archived := demoReassignArchive(t, s, before)
	if !reflect.DeepEqual(archived.Report, before.Report) || !reflect.DeepEqual(archived.Plan, before.Plan) || archived.Usage != before.Usage || !slices.Equal(archived.Sessions, before.Sessions) {
		t.Fatalf("full previous result was not available through paginated history: %+v", archived)
	}
	prior := call(t, s, protocol.MSessionAttach, protocol.SessionAttachParams{ID: before.Session}).(protocol.SessionAttachResult)
	if !reflect.DeepEqual(prior, old) {
		t.Fatal("completed conversation was overwritten by a queued reassignment")
	}
	team := &s.data.teams.teams[teamIndex]
	for i := range team.Members {
		if team.Members[i].ID == replacement.Member {
			team.Members[i].Model = demoFreeID
			team.Members[i].Budget = &protocol.TeamBudget{TaskTurns: 2}
			team.Members[i].Trust = protocol.ApprovalsReadOnly
		}
	}
	call(t, s, protocol.MTeamsPause, protocol.TeamPauseParams{ID: team.ID, Paused: false})
	resumed := call(t, s, protocol.MTeamsTaskRead, protocol.TeamTaskRef{ID: replacement.ID}).(protocol.TeamTask)
	conversation := call(t, s, protocol.MSessionAttach, protocol.SessionAttachParams{ID: resumed.Session}).(protocol.SessionAttachResult)
	if resumed.Status != protocol.TaskWorking || resumed.Model != demoFreeID || resumed.MaxTurns != 2 || resumed.Goal != originalPurpose || resumed.Session == before.Session || conversation.Info.Approvals != protocol.ApprovalsReadOnly {
		t.Fatalf("unpause did not start under the current member policy: %+v", resumed)
	}
}
