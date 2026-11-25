package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// One bounded record per seat, not an append-only history. The task file's
// Trigger is the commit record if the daemon exits before advancing Next.
type heartbeatSchedule struct {
	Team         string    `json:"team"`
	Member       string    `json:"member"`
	EveryMinutes int       `json:"everyMinutes"`
	Next         time.Time `json:"next"`
}

func heartbeatKey(team, member string) string {
	sum := sha256.Sum256([]byte(team + "\x00" + member))
	return "heartbeat-" + hex.EncodeToString(sum[:])
}

func (st *state) heartbeatSchedule(team, member string) (heartbeatSchedule, bool, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	var schedule heartbeatSchedule
	raw, err := os.ReadFile(filepath.Join(st.dir, "heartbeats", heartbeatKey(team, member)+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return schedule, false, nil
	}
	if err != nil {
		return schedule, false, err
	}
	if err := json.Unmarshal(raw, &schedule); err != nil {
		return schedule, false, err
	}
	if schedule.Team != team || schedule.Member != member || schedule.Next.IsZero() || schedule.EveryMinutes < 1 || schedule.EveryMinutes > 31*24*60 {
		return schedule, false, fmt.Errorf("invalid interval schedule for %s/%s", team, member)
	}
	return schedule, true, nil
}

func (st *state) putHeartbeatSchedule(schedule heartbeatSchedule) error {
	raw, err := json.Marshal(schedule)
	if err != nil {
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.writer.write(filepath.Join(st.dir, "heartbeats"), heartbeatKey(schedule.Team, schedule.Member)+".json", append(raw, '\n'))
}
