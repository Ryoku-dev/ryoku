package teams

import (
	"bytes"
	"encoding/json"

	"ryoku-rashin/internal/protocol"
)

func strictCall[T any](raw json.RawMessage, run func(T) (any, error)) (any, error) {
	var params T
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&params); err != nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "The request's parameters are not valid.")
	}
	if len(bytes.TrimSpace(raw[decoder.InputOffset():])) != 0 {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "The request must contain exactly one parameter object.")
	}
	return run(params)
}
