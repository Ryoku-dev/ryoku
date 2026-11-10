package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	store "ryoku-rashin/internal/session"
)

func (d *Daemon) runMigrations(ctx context.Context) error {
	if err := d.migrateProwl(ctx); err != nil {
		return err
	}
	return d.migrateAskHistory()
}

func migrationPending(name string) bool {
	_, err := os.Stat(paths.MigrationMarker(name))
	return errors.Is(err, os.ErrNotExist)
}

func markMigrated(name string) error {
	path := paths.MigrationMarker(name)
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o600)
}

func (d *Daemon) migrateProwl(ctx context.Context) error {
	if !migrationPending("prowl-keys") {
		return nil
	}
	if _, err := os.Stat(paths.ProwlGatewayDir()); err == nil {
		if _, err := d.deps.Gateway.ImportProwl(ctx, paths.ProwlGatewayDir()); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return markMigrated("prowl-keys")
}

type legacyAsk struct {
	ID               string                `json:"id"`
	At               time.Time             `json:"at"`
	Kind             string                `json:"kind"`
	Question         string                `json:"q"`
	Answer           string                `json:"a"`
	Images           []string              `json:"images"`
	Actions          []protocol.Action     `json:"actions"`
	Plan             *protocol.CommandPlan `json:"plan"`
	Via              string                `json:"via"`
	NeedsAgent       bool                  `json:"needsAgent"`
	NeedsAgentReason string                `json:"needsAgentReason"`
}

func (d *Daemon) migrateAskHistory() error {
	if !migrationPending("ask-history") {
		return nil
	}
	f, err := os.Open(paths.LegacyAskHistory())
	if errors.Is(err, os.ErrNotExist) {
		return markMigrated("ask-history")
	}
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, protocol.MaxLine)
	for scanner.Scan() {
		var old legacyAsk
		if json.Unmarshal(scanner.Bytes(), &old) != nil || old.Question == "" {
			continue
		}
		created := old.At
		if created.IsZero() {
			created = time.Now()
		}
		id, err := store.NewID(protocol.KindAsk, created)
		if err != nil {
			return err
		}
		attachments := make([]protocol.Attachment, 0, len(old.Images))
		for _, path := range old.Images {
			attachments = append(attachments, protocol.Attachment{Path: path})
		}
		info := protocol.SessionInfo{ID: id, Kind: protocol.KindAsk, Title: old.Question, Origin: protocol.OriginImport, State: protocol.StateIdle, Created: created, Updated: created, Turns: 1}
		log, err := store.Create(info, nil)
		if err != nil {
			return err
		}
		if _, err = log.Append(protocol.EvUser, protocol.UserData{Text: old.Question, Images: attachments}); err == nil && old.Answer != "" {
			_, err = log.Append(protocol.EvText, protocol.TextData{Message: "import", Delta: old.Answer})
		}
		if err == nil {
			_, err = log.Append(protocol.EvAnswer, protocol.AnswerData{Actions: old.Actions, Plan: old.Plan, Via: old.Via, NeedsAgent: old.NeedsAgent, Reason: old.NeedsAgentReason})
		}
		if err == nil {
			_, err = log.Append(protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd})
		}
		if err == nil {
			err = log.Update(func(i *protocol.SessionInfo) { i.Preview = lastRunes(old.Answer, 140) }, nil)
		}
		closeErr := log.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return markMigrated("ask-history")
}
