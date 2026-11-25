package teams

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"math"
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

// monthSpend is one team's tokens in one month. The members', routines' and
// engines' task counts are not kept here: they come from the tasks
// themselves. The days count the tasks that ended on them, which a task
// cannot tell once it is retried and ends again. Engines are keyed by
// engineKey.
type monthSpend struct {
	Days           map[string]daySpend `json:"days"`
	Members        map[string]int64    `json:"members,omitempty"`
	Routines       map[string]int64    `json:"routines,omitempty"`
	Engines        map[string]int64    `json:"engines,omitempty"`
	Estimated      bool                `json:"estimated,omitempty"`
	MemberWarnings map[string]int64    `json:"memberWarnings,omitempty"`
	// Unmetered records incomplete token accounting, not a measured zero.
	// The empty member key means usage could not be attributed to a seat.
	Unmetered         bool            `json:"unmetered,omitempty"`
	UnmeteredMembers  map[string]bool `json:"unmeteredMembers,omitempty"`
	UnmeteredRoutines map[string]bool `json:"unmeteredRoutines,omitempty"`
	UnmeteredEngines  map[string]bool `json:"unmeteredEngines,omitempty"`
}

// daySpend is one day's tokens and the tasks that ended done or blocked on
// it, keyed by "2026-11-11". A file from before the counts reads them as 0.
type daySpend struct {
	Input     int64 `json:"input"`
	Output    int64 `json:"output"`
	Tasks     int   `json:"tasks,omitempty"`
	Routines  int   `json:"routines,omitempty"`
	Unmetered bool  `json:"unmetered,omitempty"`
}

const (
	monthLayout = "2006-01"
	dayLayout   = "2006-01-02"
)

// engineKey keeps native gateway IDs and external runtime/model identities
// separate. Work that has not started has no attributed engine.
func engineKey(t protocol.TeamTask) string {
	if t.Started == nil {
		return ""
	}
	return runtimeEngineKey(t.Runtime, t.Model)
}

func runtimeEngineKey(kind, model string) string {
	if kind != "" && kind != protocol.MemberRuntimeNative {
		key := "runtime:" + kind
		if model != "" {
			key += ":" + model
		}
		return key
	}
	return model
}

// formerEngineSeparator joined a former external harness's id and its model
// in an engine key ("harness\x1fmodel"); such a key now counts for its model.
const formerEngineSeparator = "\x1f"

// modelEngines folds engine keys a former harness wrote into their models:
// a key whose model Rashin cannot route, or that named none, is dropped with
// its tokens, which the days and members still count. It reports whether it
// changed anything.
func modelEngines(file spendFile) bool {
	changed := false
	for _, teams := range file {
		for _, ms := range teams {
			if ms == nil {
				continue
			}
			for key, tokens := range ms.Engines {
				_, model, former := strings.Cut(key, formerEngineSeparator)
				if !former {
					continue
				}
				delete(ms.Engines, key)
				changed = true
				if model != "" && protocol.ValidEngineModel(model) {
					ms.Engines[model] = saturatedTokens(ms.Engines[model], tokens)
				}
			}
			if len(ms.Engines) == 0 {
				ms.Engines = nil
			}
		}
	}
	return changed
}

// addSpend counts usage in the existing durable account. Errors are sticky
// until restart, so a capped task cannot keep running on unrecorded usage.
func (s *Service) addSpend(team, member, routine, engine string, at time.Time, usage protocol.TokenUsage) error {
	if team == "" {
		return nil
	}
	st := s.state
	st.spendMu.Lock()
	defer st.spendMu.Unlock()
	file, err := st.spendLocked()
	if err != nil {
		return err
	}
	if usage.Input < 0 || usage.Output < 0 {
		st.spendErr = errors.New("negative token usage")
		return st.spendErr
	}
	if usage.Input == 0 && usage.Output == 0 && !usage.Estimated && !usage.Unmetered {
		return nil
	}
	ms := file.month(team, at)
	day := ms.Days[at.Format(dayLayout)]
	day.Input = saturatedTokens(day.Input, int64(usage.Input))
	day.Output = saturatedTokens(day.Output, int64(usage.Output))
	ms.Days[at.Format(dayLayout)] = day
	if usage.Unmetered {
		day.Unmetered = true
		ms.Days[at.Format(dayLayout)] = day
		ms.Unmetered = true
		if ms.UnmeteredMembers == nil {
			ms.UnmeteredMembers = make(map[string]bool)
		}
		ms.UnmeteredMembers[member] = true
		if routine != "" {
			if ms.UnmeteredRoutines == nil {
				ms.UnmeteredRoutines = make(map[string]bool)
			}
			ms.UnmeteredRoutines[routine] = true
		}
		if engine != "" {
			if ms.UnmeteredEngines == nil {
				ms.UnmeteredEngines = make(map[string]bool)
			}
			ms.UnmeteredEngines[engine] = true
		}
	}
	tokens := saturatedTokens(int64(usage.Input), int64(usage.Output))
	if member != "" {
		if ms.Members == nil {
			ms.Members = make(map[string]int64)
		}
		ms.Members[member] = saturatedTokens(ms.Members[member], tokens)
	}
	if routine != "" {
		if ms.Routines == nil {
			ms.Routines = make(map[string]int64)
		}
		ms.Routines[routine] = saturatedTokens(ms.Routines[routine], tokens)
	}
	if engine != "" {
		if ms.Engines == nil {
			ms.Engines = make(map[string]int64)
		}
		ms.Engines[engine] = saturatedTokens(ms.Engines[engine], tokens)
	}
	ms.Estimated = ms.Estimated || usage.Estimated
	return st.saveSpendLocked(file)
}

// countEnded counts a task that ended done or blocked on the day it ended,
// and as a routine run when it was one. A retried task that ends again
// counts again. A spend file that cannot be written loses the count.
func (s *Service) countEnded(t protocol.TeamTask) {
	if t.Team == "" || t.Ended == nil || (t.Status != protocol.TaskDone && t.Status != protocol.TaskBlocked) {
		return
	}
	st := s.state
	st.spendMu.Lock()
	defer st.spendMu.Unlock()
	file, err := st.spendLocked()
	if err != nil {
		return
	}
	ms := file.month(t.Team, *t.Ended)
	key := t.Ended.Format(dayLayout)
	day := ms.Days[key]
	day.Tasks = saturatedCount(day.Tasks, 1)
	if t.Routine != "" {
		day.Routines = saturatedCount(day.Routines, 1)
	}
	ms.Days[key] = day
	st.saveSpendLocked(file)
}

// month is a team's spend in the month of at, made with its days when the
// team has none there yet.
func (file spendFile) month(team string, at time.Time) *monthSpend {
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
		ms.Days = make(map[string]daySpend)
	}
	return ms
}

// saveSpendLocked retains the first failure. In-memory counts are not a
// substitute for a durable write when deciding whether more spend is safe.
func (st *state) saveSpendLocked(file spendFile) error {
	if st.spendErr != nil {
		return st.spendErr
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err == nil {
		err = st.writer.write(st.dir, "spend.json", append(data, '\n'))
	}
	if err != nil {
		st.spendErr = err
	}
	return err
}

// monthTokens is what a team used in the month of at.
func (s *Service) monthTokens(team string, at time.Time) int64 {
	st := s.state
	st.spendMu.Lock()
	defer st.spendMu.Unlock()
	file, err := st.spendLocked()
	if err != nil {
		return math.MaxInt64
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
	byTeam, err := s.spendOfMonth(month)
	if err != nil {
		return protocol.TeamSpend{}, err
	}
	ran := s.state.tasks(func(t protocol.TeamTask) bool {
		return t.Started != nil && t.Started.Format(monthLayout) == month
	})
	out := protocol.TeamSpend{Team: p.Team, Month: month, Days: []protocol.DaySpend{}}
	days := make(map[string]daySpend)
	limited := true
	for _, team := range teams {
		ms := byTeam[team.ID]
		tokens := ms.tokens()
		out.Tokens = saturatedTokens(out.Tokens, tokens)
		out.Cap = saturatedTokens(out.Cap, team.Budget.MonthTokens)
		limited = limited && team.Budget.MonthTokens > 0
		if ms != nil {
			out.Estimated = out.Estimated || ms.Estimated
			out.Unmetered = out.Unmetered || ms.Unmetered
			for day, d := range ms.Days {
				sum := days[day]
				sum.Input = saturatedTokens(sum.Input, d.Input)
				sum.Output = saturatedTokens(sum.Output, d.Output)
				sum.Tasks = saturatedCount(sum.Tasks, d.Tasks)
				sum.Routines = saturatedCount(sum.Routines, d.Routines)
				sum.Unmetered = sum.Unmetered || d.Unmetered
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
			out.Teams = append(out.Teams, protocol.NamedSpend{ID: team.ID, Name: team.Name, Tokens: tokens, Tasks: count, Unmetered: ms != nil && ms.Unmetered})
		}
	}
	if !limited {
		// one team without a limit leaves the teams together without one
		out.Cap = 0
	}
	for day, d := range days {
		out.Days = append(out.Days, protocol.DaySpend{Day: day, Input: d.Input, Output: d.Output, Tasks: d.Tasks, Routines: d.Routines, Unmetered: d.Unmetered})
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
func (s *Service) spendOfMonth(month string) (map[string]*monthSpend, error) {
	st := s.state
	st.spendMu.Lock()
	defer st.spendMu.Unlock()
	file, err := st.spendLocked()
	if err != nil {
		return nil, err
	}
	out := make(map[string]*monthSpend, len(file[month]))
	for team, ms := range file[month] {
		if ms == nil {
			continue
		}
		clone := &monthSpend{
			Days:              maps.Clone(ms.Days),
			Members:           maps.Clone(ms.Members),
			Routines:          maps.Clone(ms.Routines),
			Engines:           maps.Clone(ms.Engines),
			MemberWarnings:    maps.Clone(ms.MemberWarnings),
			Estimated:         ms.Estimated,
			Unmetered:         ms.Unmetered,
			UnmeteredMembers:  maps.Clone(ms.UnmeteredMembers),
			UnmeteredRoutines: maps.Clone(ms.UnmeteredRoutines),
			UnmeteredEngines:  maps.Clone(ms.UnmeteredEngines),
		}
		out[team] = clone
	}
	return out, nil
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
				entry(key).Tokens = saturatedTokens(entry(key).Tokens, tokens)
			}
			for key, unmetered := range ms.UnmeteredEngines {
				entry(key).Unmetered = entry(key).Unmetered || unmetered
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
		for id, unmetered := range ms.UnmeteredMembers {
			if id != "" {
				entry(id).Unmetered = unmetered
			}
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
		for id, unmetered := range ms.UnmeteredRoutines {
			entry(id).Unmetered = unmetered
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
		total = saturatedTokens(total, saturatedTokens(d.Input, d.Output))
	}
	return total
}

// spendLocked loads spend.json on first use. A file that does not parse is
// an error, so a later write never replaces it with less.
func (st *state) spendLocked() (spendFile, error) {
	if st.spendErr != nil {
		return nil, st.spendErr
	}
	if st.spendData != nil {
		return st.spendData, nil
	}
	data, err := os.ReadFile(filepath.Join(st.dir, "spend.json"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		st.spendData = make(spendFile)
		return st.spendData, nil
	case err != nil:
		st.spendErr = err
		return nil, err
	}
	file := make(spendFile)
	if err := json.Unmarshal(data, &file); err != nil {
		st.spendErr = err
		return nil, err
	}
	if file == nil {
		st.spendErr = errors.New("null spending account")
		return nil, st.spendErr
	}
	if err := validateSpendFile(file); err != nil {
		st.spendErr = err
		return nil, err
	}
	st.spendData = file
	if modelEngines(file) {
		if err := st.saveSpendLocked(file); err != nil {
			return nil, err
		}
	}
	return file, nil
}

// A negative persisted counter is corruption or an old arithmetic wrap, not
// credit. Never replace that account with an apparently fresh zero balance.
func validateSpendFile(file spendFile) error {
	for _, teams := range file {
		for _, ms := range teams {
			if ms == nil {
				return errors.New("null monthly spending account")
			}
			for _, day := range ms.Days {
				if day.Input < 0 || day.Output < 0 || day.Tasks < 0 || day.Routines < 0 {
					return errors.New("negative spending counter")
				}
			}
			for _, counts := range []map[string]int64{ms.Members, ms.Routines, ms.Engines, ms.MemberWarnings} {
				for _, value := range counts {
					if value < 0 {
						return errors.New("negative spending counter")
					}
				}
			}
		}
	}
	return nil
}

func saturatedCount(a, b int) int {
	if a < 0 || b < 0 || a > math.MaxInt-b {
		return math.MaxInt
	}
	return a + b
}

// monthUnmetered accompanies the known-token total in team-list statistics.
// Missing accounting is handled as an error by budgetTokens/spend; this flag
// specifically identifies successfully recorded but unmetered runtime usage.
func (s *Service) monthUnmetered(team string, at time.Time) bool {
	st := s.state
	st.spendMu.Lock()
	defer st.spendMu.Unlock()
	file, err := st.spendLocked()
	if err != nil {
		return false
	}
	ms := file[at.Format(monthLayout)][team]
	return ms != nil && ms.Unmetered
}
