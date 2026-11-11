package teams

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// spendFile is spend.json: per month ("2026-11"), per team id.
type spendFile map[string]map[string]*monthSpend

// monthSpend is one team's tokens in one month. Task counts are not kept
// here: they come from the tasks themselves, which addSpend is not told of.
// Engines are keyed by engineKey.
type monthSpend struct {
	Days      map[string]dayTokens `json:"days"`
	Members   map[string]int64     `json:"members,omitempty"`
	Routines  map[string]int64     `json:"routines,omitempty"`
	Engines   map[string]int64     `json:"engines,omitempty"`
	Estimated bool                 `json:"estimated,omitempty"`
}

// dayTokens is one day's tokens, keyed by "2026-11-11".
type dayTokens struct {
	Input  int64 `json:"input"`
	Output int64 `json:"output"`
}

const (
	monthLayout = "2006-01"
	dayLayout   = "2006-01-02"
)

// engineKey names the engine a task ran on in the spend: its harness and
// model joined by protocol.EngineSpendSeparator; empty before it started.
func engineKey(t protocol.TeamTask) string {
	if t.Harness == "" {
		return ""
	}
	return t.Harness + protocol.EngineSpendSeparator + t.Model
}

// addSpend counts a turn's tokens against its team, member, routine and
// engine. A spend file that cannot be written loses the count, never the
// turn.
func (s *Service) addSpend(team, member, routine, engine string, at time.Time, usage protocol.TokenUsage) {
	if team == "" || (usage.Input == 0 && usage.Output == 0 && !usage.Estimated) {
		return
	}
	st := s.state
	st.spendMu.Lock()
	defer st.spendMu.Unlock()
	file, err := st.spendLocked()
	if err != nil {
		return
	}
	month := at.Format(monthLayout)
	if file[month] == nil {
		file[month] = make(map[string]*monthSpend)
	}
	ms := file[month][team]
	if ms == nil {
		ms = &monthSpend{}
		file[month][team] = ms
	}
	if ms.Days == nil {
		ms.Days = make(map[string]dayTokens)
	}
	day := ms.Days[at.Format(dayLayout)]
	day.Input += int64(usage.Input)
	day.Output += int64(usage.Output)
	ms.Days[at.Format(dayLayout)] = day
	tokens := int64(usage.Input + usage.Output)
	if member != "" {
		if ms.Members == nil {
			ms.Members = make(map[string]int64)
		}
		ms.Members[member] += tokens
	}
	if routine != "" {
		if ms.Routines == nil {
			ms.Routines = make(map[string]int64)
		}
		ms.Routines[routine] += tokens
	}
	if engine != "" {
		if ms.Engines == nil {
			ms.Engines = make(map[string]int64)
		}
		ms.Engines[engine] += tokens
	}
	ms.Estimated = ms.Estimated || usage.Estimated
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return
	}
	_ = writeStateFile(st.dir, "spend.json", append(data, '\n'))
}

// monthTokens is what a team used in the month of at.
func (s *Service) monthTokens(team string, at time.Time) int64 {
	st := s.state
	st.spendMu.Lock()
	defer st.spendMu.Unlock()
	file, err := st.spendLocked()
	if err != nil {
		return 0
	}
	return file[at.Format(monthLayout)][team].tokens()
}

// spend reports a month of one team's spend against its limit, or of every
// team's when Team is empty.
func (s *Service) spend(p protocol.TeamSpendParams) (protocol.TeamSpend, error) {
	month := p.Month
	if month == "" {
		month = s.now().Format(monthLayout)
	} else if _, err := time.Parse(monthLayout, month); err != nil {
		return protocol.TeamSpend{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a month like 2026-11.", month))
	}
	var teams []protocol.Team
	if p.Team != "" {
		team, ok := s.state.team(p.Team)
		if !ok {
			return protocol.TeamSpend{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No team %s.", p.Team))
		}
		teams = []protocol.Team{team}
	} else {
		teams = s.state.teams()
	}
	byTeam := s.spendOfMonth(month)
	ran := s.state.tasks(func(t protocol.TeamTask) bool {
		return t.Started != nil && t.Started.Format(monthLayout) == month
	})
	out := protocol.TeamSpend{Team: p.Team, Month: month, Days: []protocol.DaySpend{}}
	days := make(map[string]dayTokens)
	limited := true
	for _, team := range teams {
		ms := byTeam[team.ID]
		tokens := ms.tokens()
		out.Tokens += tokens
		out.Cap += team.Budget.MonthTokens
		limited = limited && team.Budget.MonthTokens > 0
		if ms != nil {
			out.Estimated = out.Estimated || ms.Estimated
			for day, d := range ms.Days {
				sum := days[day]
				sum.Input += d.Input
				sum.Output += d.Output
				days[day] = sum
			}
		}
		if p.Team == "" {
			count := 0
			for _, t := range ran {
				if t.Team == team.ID {
					count++
				}
			}
			out.Teams = append(out.Teams, protocol.NamedSpend{ID: team.ID, Name: team.Name, Tokens: tokens, Tasks: count})
		}
	}
	if !limited {
		// one team without a limit leaves the teams together without one
		out.Cap = 0
	}
	for day, d := range days {
		out.Days = append(out.Days, protocol.DaySpend{Day: day, Input: d.Input, Output: d.Output})
	}
	sort.Slice(out.Days, func(i, j int) bool { return out.Days[i].Day < out.Days[j].Day })
	out.Engines = engineSpend(teams, byTeam, ran)
	if p.Team != "" {
		team := teams[0]
		out.Members = s.memberSpend(team, byTeam[team.ID], ran)
		out.Routines = s.routineSpend(team.ID, byTeam[team.ID], ran)
	}
	sortSpend(out.Teams)
	return out, nil
}

// spendOfMonth copies one month of spend so it can be read without the lock.
func (s *Service) spendOfMonth(month string) map[string]*monthSpend {
	st := s.state
	st.spendMu.Lock()
	defer st.spendMu.Unlock()
	file, err := st.spendLocked()
	if err != nil {
		return nil
	}
	out := make(map[string]*monthSpend, len(file[month]))
	for team, ms := range file[month] {
		clone := &monthSpend{
			Days:      maps.Clone(ms.Days),
			Members:   maps.Clone(ms.Members),
			Routines:  maps.Clone(ms.Routines),
			Engines:   maps.Clone(ms.Engines),
			Estimated: ms.Estimated,
		}
		out[team] = clone
	}
	return out
}

// engineSpend lists the engines the teams' tasks ran on in the month, with
// the tokens each used and the tasks that started on it; the app words
// their names.
func engineSpend(teams []protocol.Team, byTeam map[string]*monthSpend, ran []protocol.TeamTask) []protocol.NamedSpend {
	in := make(map[string]bool, len(teams))
	spends := make(map[string]*protocol.NamedSpend)
	entry := func(key string) *protocol.NamedSpend {
		if spends[key] == nil {
			spends[key] = &protocol.NamedSpend{ID: key}
		}
		return spends[key]
	}
	for _, team := range teams {
		in[team.ID] = true
		if ms := byTeam[team.ID]; ms != nil {
			for key, tokens := range ms.Engines {
				entry(key).Tokens += tokens
			}
		}
	}
	for _, t := range ran {
		if key := engineKey(t); key != "" && in[t.Team] {
			entry(key).Tasks++
		}
	}
	return flattenSpend(spends)
}

// memberSpend lists a team's members that used tokens or ran tasks in the
// month, named as they are now.
func (s *Service) memberSpend(team protocol.Team, ms *monthSpend, ran []protocol.TeamTask) []protocol.NamedSpend {
	spends := make(map[string]*protocol.NamedSpend)
	entry := func(id string) *protocol.NamedSpend {
		if spends[id] == nil {
			name := i18n.T("A former member")
			for _, m := range team.Members {
				if m.ID == id {
					name = m.Name
				}
			}
			spends[id] = &protocol.NamedSpend{ID: id, Name: name}
		}
		return spends[id]
	}
	if ms != nil {
		for id, tokens := range ms.Members {
			entry(id).Tokens = tokens
		}
	}
	for _, t := range ran {
		if t.Team == team.ID && t.Member != "" {
			entry(t.Member).Tasks++
		}
	}
	return flattenSpend(spends)
}

// routineSpend lists a team's routines that used tokens or ran tasks in the
// month.
func (s *Service) routineSpend(team string, ms *monthSpend, ran []protocol.TeamTask) []protocol.NamedSpend {
	spends := make(map[string]*protocol.NamedSpend)
	entry := func(id string) *protocol.NamedSpend {
		if spends[id] == nil {
			name := i18n.T("A deleted routine")
			if r, ok := s.state.routine(id); ok {
				name = r.Title
			}
			spends[id] = &protocol.NamedSpend{ID: id, Name: name}
		}
		return spends[id]
	}
	if ms != nil {
		for id, tokens := range ms.Routines {
			entry(id).Tokens = tokens
		}
	}
	for _, t := range ran {
		if t.Team == team && t.Routine != "" {
			entry(t.Routine).Tasks++
		}
	}
	return flattenSpend(spends)
}

func flattenSpend(spends map[string]*protocol.NamedSpend) []protocol.NamedSpend {
	out := make([]protocol.NamedSpend, 0, len(spends))
	for _, spend := range spends {
		out = append(out, *spend)
	}
	sortSpend(out)
	return out
}

// sortSpend puts the biggest spender first.
func sortSpend(list []protocol.NamedSpend) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Tokens != list[j].Tokens {
			return list[i].Tokens > list[j].Tokens
		}
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
}

func (ms *monthSpend) tokens() int64 {
	if ms == nil {
		return 0
	}
	var total int64
	for _, d := range ms.Days {
		total += d.Input + d.Output
	}
	return total
}

// spendLocked loads spend.json on first use. A file that does not parse is
// an error, so a later write never replaces it with less.
func (st *state) spendLocked() (spendFile, error) {
	if st.spendData != nil {
		return st.spendData, nil
	}
	data, err := os.ReadFile(filepath.Join(st.dir, "spend.json"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		st.spendData = make(spendFile)
		return st.spendData, nil
	case err != nil:
		return nil, err
	}
	file := make(spendFile)
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	st.spendData = file
	return file, nil
}
