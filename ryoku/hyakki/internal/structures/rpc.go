package structures

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// IsMethod identifies the local catalogue API, shared by Oni and demo.
func IsMethod(method string) bool {
	switch method {
	case protocol.MTeamsStructures, protocol.MTeamsStructureDraft, protocol.MTeamsStructureSuggest,
		protocol.MTeamsStructureSave, protocol.MTeamsStructureClone, protocol.MTeamsStructureDelete, protocol.MTeamsStructureRestore:
		return true
	}
	return false
}

// Handle serves recipes only. Caller authorization belongs to the host; no
// method creates a team, grants tools, selects a model or contacts an account.
// changed is an empty invalidation after a durable successful mutation.
func Handle(store *Store, method string, raw json.RawMessage, changed func()) (any, error) {
	if store == nil {
		return nil, errors.New("structure store is not open")
	}
	var result any
	var err error
	mutated := false
	switch method {
	case protocol.MTeamsStructures:
		result, err = catalogueCall(raw, func(protocol.Empty) (any, error) { return store.Catalogue() })
	case protocol.MTeamsStructureDraft:
		result, err = catalogueCall(raw, func(p protocol.StructureDraftParams) (any, error) {
			if err := validateRef(p.ID); err != nil {
				return nil, err
			}
			list, err := store.List()
			if err != nil {
				return nil, err
			}
			for _, t := range list {
				if t.ID != p.ID || t.Hidden {
					continue
				}
				members, err := Instantiate(t, p.Count)
				if err != nil {
					return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", err)
				}
				return boundedPreview(protocol.StructureDraftResult{Template: t, Members: members})
			}
			return nil, ErrNotFound
		})
	case protocol.MTeamsStructureSuggest:
		result, err = catalogueCall(raw, func(p protocol.StructureAnswers) (any, error) {
			if err := ValidateAnswers(p); err != nil {
				return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", err)
			}
			list, err := store.List()
			if err != nil {
				return nil, err
			}
			// Rank before expanding at most three rosters. Escaped custom text
			// can still exceed a frame, which must be an actionable RPC error.
			suggestions := SuggestTop(p, list, 3)
			return boundedPreview(protocol.StructureSuggestionsResult{Suggestions: suggestions})
		})
	case protocol.MTeamsStructureSave:
		result, err = catalogueCall(raw, func(p protocol.StructureSaveParams) (any, error) { return store.Save(p.Template, p.ExpectedRevision) })
		mutated = err == nil
	case protocol.MTeamsStructureClone:
		result, err = catalogueCall(raw, func(p protocol.StructureCloneParams) (any, error) {
			if err := validateRef(p.ID); err != nil {
				return nil, err
			}
			return store.Clone(p.ID, p.NewID, p.Name)
		})
		mutated = err == nil
	case protocol.MTeamsStructureDelete:
		result, err = catalogueCall(raw, func(p protocol.StructureRef) (any, error) {
			if err := validateRef(p.ID); err != nil {
				return nil, err
			}
			return protocol.Empty{}, store.Delete(p.ID, p.ExpectedRevision)
		})
		mutated = err == nil
	case protocol.MTeamsStructureRestore:
		result, err = catalogueCall(raw, func(p protocol.StructureRef) (any, error) {
			if err := validateRef(p.ID); err != nil {
				return nil, err
			}
			return store.Restore(p.ID, p.ExpectedRevision)
		})
		mutated = err == nil
	default:
		return nil, protocol.Errorf(protocol.CodeUnsupported, "Unknown structure method %s.", method)
	}
	switch {
	case errors.Is(err, ErrStaleRevision):
		err = protocol.Errorf(protocol.CodeConflict, "%s", err)
	case errors.Is(err, ErrNotFound):
		err = protocol.Errorf(protocol.CodeNotFound, "%s", err)
	case errors.Is(err, ErrImmutable), errors.Is(err, ErrInvalid):
		err = protocol.Errorf(protocol.CodeBadRequest, "%s", err)
	}
	if mutated && changed != nil {
		changed()
	}
	return result, err
}

func catalogueCall[T any](raw json.RawMessage, run func(T) (any, error)) (any, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if raw[0] != '{' {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "Structure parameters must be an object.")
	}
	var params T
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&params); err != nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "Structure parameters have trailing content.")
	}
	return run(params)
}

func validateRef(id string) error {
	if !keyPattern.MatchString(strings.TrimPrefix(id, "custom:")) {
		return protocol.Errorf(protocol.CodeBadRequest, "A valid structure ID is required.")
	}
	return nil
}

func boundedPreview(result any) (any, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	// Reserve space for the Response envelope and request ID.
	if len(encoded) > protocol.MaxLine-1024 {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "The expanded preview exceeds the protocol frame limit; choose fewer seats or shorten the custom recipe text.")
	}
	return result, nil
}
