package presence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// PlaceVersion is the only placement file version this build reads.
const PlaceVersion = 1

const maxPlaceBytes = 64 << 10

// DefaultCorner is where Oni stands before it was ever placed.
const DefaultCorner = protocol.OniCornerBottomRight

// Place is where Oni stands: pinned to Corner, or with no corner at the
// saved spot of the Active output. Outputs keep each output's last spot so
// Oni returns to it when that output comes back.
type Place struct {
	Corner  string
	Outputs []feature.PlacementOutput
	Active  string
}

// PlacementFile is the file Oni's place is saved in.
func PlacementFile() string { return filepath.Join(paths.StateDir(), "oni-placement.json") }

type placeFile struct {
	Version int                       `json:"version"`
	Corner  string                    `json:"corner"`
	Outputs []feature.PlacementOutput `json:"outputs"`
	Active  string                    `json:"active,omitempty"`
}

var placeKeys = []string{"version", "corner", "outputs", "active"}

// Place saves p and sends the new placement. A pin keeps every saved spot;
// a free spot is saved for its output, which becomes the active one.
func (s *Service) Place(p protocol.OniPlaceParams) error {
	if _, err := feature.DecodeCall(protocol.MOniPlace, mustJSON(p)); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := Place{Corner: p.Corner, Outputs: slices.Clone(s.place.Outputs), Active: s.place.Active}
	if p.Output != "" {
		next.Active = p.Output
	}
	if p.Corner == "" {
		spot := feature.PlacementOutput{ID: p.Output, FX: p.FX, FY: p.FY}
		if i := slices.IndexFunc(next.Outputs, func(o feature.PlacementOutput) bool { return o.ID == p.Output }); i >= 0 {
			next.Outputs[i] = spot
		} else {
			next.Outputs = append(next.Outputs, spot)
			if len(next.Outputs) > feature.MaxPlacementOutputs {
				next.Outputs = next.Outputs[len(next.Outputs)-feature.MaxPlacementOutputs:]
			}
		}
	}
	if err := savePlace(s.file, next); err != nil {
		return err
	}
	s.place = next
	if !s.closed && s.last.on {
		s.send(s.placementFrame())
	}
	return nil
}

func (s *Service) placementFrame() *feature.PlacementFrame {
	s.placeSeq++
	return &feature.PlacementFrame{
		Seq:     s.placeSeq,
		Corner:  s.place.Corner,
		Size:    s.settings.Size,
		Outputs: append([]feature.PlacementOutput{}, s.place.Outputs...),
		Active:  s.place.Active,
	}
}

// loadPlace reads the saved place; a missing, unreadable or malformed file
// is the default, replaced by the next save.
func loadPlace(file string) Place {
	p, err := ReadPlace(file)
	if err != nil {
		return Place{Corner: DefaultCorner}
	}
	return p
}

// ReadPlace reads a placement file strictly: version PlaceVersion, exact
// keys, no nulls, duplicates or trailing data, and a place a placement frame
// could carry. A missing file is fs.ErrNotExist.
func ReadPlace(file string) (Place, error) {
	f, err := os.Open(file)
	if err != nil {
		return Place{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxPlaceBytes+1))
	if err != nil {
		return Place{}, err
	}
	if len(data) > maxPlaceBytes {
		return Place{}, fmt.Errorf("%s is larger than %d bytes", file, maxPlaceBytes)
	}
	if err := noDuplicateKeys(data); err != nil {
		return Place{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var top map[string]json.RawMessage
	if err := dec.Decode(&top); err != nil || top == nil {
		return Place{}, errors.New("the placement is not a JSON object")
	}
	if _, err := dec.Token(); err != io.EOF {
		return Place{}, errors.New("trailing data after the placement")
	}
	for key := range top {
		if !slices.Contains(placeKeys, key) {
			return Place{}, fmt.Errorf("unknown key %q", key)
		}
	}
	if string(top["version"]) != strconv.Itoa(PlaceVersion) {
		return Place{}, fmt.Errorf("placement version is not %d", PlaceVersion)
	}
	// the rest is a placement frame's data, checked as strictly
	delete(top, "version")
	top["seq"] = json.RawMessage("1")
	top["size"] = json.RawMessage(strconv.Quote(protocol.OniSizeNormal))
	frame, err := json.Marshal(feature.Frame{Kind: feature.KindPlacement, Data: mustJSON(top)})
	if err != nil {
		return Place{}, err
	}
	payload, err := feature.DecodeFrame(frame)
	if err != nil {
		return Place{}, err
	}
	pf := payload.(*feature.PlacementFrame)
	return Place{Corner: pf.Corner, Outputs: pf.Outputs, Active: pf.Active}, nil
}

func savePlace(file string, p Place) error {
	data, err := json.Marshal(placeFile{Version: PlaceVersion, Corner: p.Corner, Outputs: append([]feature.PlacementOutput{}, p.Outputs...), Active: p.Active})
	if err != nil {
		return err
	}
	if err := paths.EnsureDir(filepath.Dir(file)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".oni-placement-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// encoding/json keeps the last of two equal keys; a file with both is
// ambiguous and refused
func noDuplicateKeys(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		seen := map[string]bool{}
		for dec.More() {
			if delim == '{' {
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				key, _ := kt.(string)
				if seen[key] {
					return fmt.Errorf("duplicate key %q", key)
				}
				seen[key] = true
			}
			if err := walk(); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	return walk()
}
