package doctor

import (
	"encoding/json"
	"os"
)

// The retired-key reconcilers edit single leaves of a JSON store in place: every
// other key keeps its own raw bytes, and objects missing on the way are created.

// readRawStore parses a store into its top-level keys. A missing file reads as
// an empty store with exists=false; one that does not parse errors.
func readRawStore(path string) (top map[string]json.RawMessage, exists bool, err error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]json.RawMessage{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, true, err
	}
	if top == nil {
		top = map[string]json.RawMessage{}
	}
	return top, true, nil
}

// writeRawStore writes a store indented with a trailing newline, atomically.
func writeRawStore(path string, top map[string]json.RawMessage) error {
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return err
	}
	return replaceFile(path, append(out, '\n'), 0o644)
}

// rawLeaf returns the value at path, nil when any step is absent.
func rawLeaf(top map[string]json.RawMessage, path ...string) (json.RawMessage, error) {
	v, ok := top[path[0]]
	if !ok {
		return nil, nil
	}
	if len(path) == 1 {
		return v, nil
	}
	var child map[string]json.RawMessage
	if err := json.Unmarshal(v, &child); err != nil {
		return nil, err
	}
	return rawLeaf(child, path[1:]...)
}

// setRawLeaf stores v at path.
func setRawLeaf(top map[string]json.RawMessage, v json.RawMessage, path ...string) error {
	if len(path) == 1 {
		top[path[0]] = v
		return nil
	}
	child := map[string]json.RawMessage{}
	if cur, ok := top[path[0]]; ok && string(cur) != "null" {
		if err := json.Unmarshal(cur, &child); err != nil {
			return err
		}
	}
	if err := setRawLeaf(child, v, path[1:]...); err != nil {
		return err
	}
	b, err := json.Marshal(child)
	if err != nil {
		return err
	}
	top[path[0]] = b
	return nil
}

// dropRawLeaf deletes the leaf at path, reporting whether it was there. The
// objects above it stay, even when left empty.
func dropRawLeaf(top map[string]json.RawMessage, path ...string) (bool, error) {
	if len(path) == 1 {
		_, ok := top[path[0]]
		delete(top, path[0])
		return ok, nil
	}
	cur, ok := top[path[0]]
	if !ok {
		return false, nil
	}
	var child map[string]json.RawMessage
	if err := json.Unmarshal(cur, &child); err != nil {
		return false, err
	}
	dropped, err := dropRawLeaf(child, path[1:]...)
	if err != nil || !dropped {
		return false, err
	}
	b, err := json.Marshal(child)
	if err != nil {
		return false, err
	}
	top[path[0]] = b
	return true, nil
}
