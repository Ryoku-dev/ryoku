package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// recordStatePath holds the hash of what was published last and its stamp: a new record is
// stamped after it, so a clock that stepped back since can't get the record refused as older
// than the last
func recordStatePath() (string, error) {
	dir, e := badgeDataDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(dir, "record.sha256"), nil
}

// recordOnline is whether this machine keeps its card on the team service. Badge holders were
// always online, so without a choice of their own they stay so; everyone else opts in.
func recordOnline() (bool, error) {
	dir, e := badgeDataDir()
	if e != nil {
		return false, e
	}
	raw, e := readBoundedFile(filepath.Join(dir, "online"), 16)
	if e != nil && !os.IsNotExist(e) {
		return false, e
	}
	switch strings.TrimSpace(string(raw)) {
	case "on":
		return true, nil
	case "off":
		return false, nil
	}
	_, e = badgeActive()
	return e == nil, nil
}

func recordSetOnline(on bool) error {
	dir, e := badgeDataDir()
	if e != nil {
		return e
	}
	choice := "off"
	if on {
		choice = "on"
	}
	if e = atomicWrite(filepath.Join(dir, "online"), []byte(choice), 0600); e != nil || on {
		return e
	}
	return recordWithdraw()
}

func recordDeletePayload(id string, at int64) ([]byte, error) {
	raw, e := hex.DecodeString(id)
	if e != nil || len(raw) != 8 || at < 0 {
		return nil, errors.New("invalid card withdrawal")
	}
	return binary.BigEndian.AppendUint64(raw, uint64(at)), nil
}

// recordWithdraw takes this machine's card off the team service; friends keep the copy they have
func recordWithdraw() error {
	seed, e := badgeDeviceSeed()
	if e != nil {
		return e
	}
	defer clear(seed)
	id := cardIDText(badgeDevicePublic(seed))
	at := time.Now().Unix()
	for clocked := false; ; clocked = true {
		payload, e := recordDeletePayload(id, at)
		if e != nil {
			return e
		}
		body, e := json.Marshal(map[string]any{"id": id, "at": at, "sig": base64.StdEncoding.EncodeToString(signDomain(seed, "ryoku-card-delete-v1", payload))})
		if e != nil {
			return e
		}
		reply, e := teamExchange(http.MethodDelete, "/v1/cards/"+id, "application/json", body, nil, 4096)
		if e != nil {
			return e
		}
		if reply.status == http.StatusOK || reply.status == http.StatusNotFound {
			return recordForgetPublished()
		}
		// this computer's clock is more than ten minutes off: date the withdrawal in the gate's time
		if !clocked && !reply.date.IsZero() && (reply.date.Unix()-at > 600 || at-reply.date.Unix() > 600) {
			at = reply.date.Unix()
			continue
		}
		return fmt.Errorf("the team service kept the card online: %s", teamRefusal(reply))
	}
}

// recordForgetPublished makes the next sync publish afresh rather than trust a record the gate no
// longer holds. The last stamp stays: the gate refuses a withdrawn record's time for a while, so
// the next record is stamped after it.
func recordForgetPublished() error {
	path, e := recordStatePath()
	if e != nil {
		return e
	}
	state, e := readBoundedFile(path, 128)
	if os.IsNotExist(e) {
		return nil
	}
	if published := strings.Fields(string(state)); e == nil && len(published) > 1 {
		return atomicWrite(path, []byte("withdrawn "+published[1]), 0600)
	}
	return os.Remove(path)
}
