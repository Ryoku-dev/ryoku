package ask

import "ryoku-rashin/internal/protocol"

// a route picked is followed wherever it leads, a model picked is held to
func pinned(model string) bool {
	return model != "" && model != protocol.RouteAuto && model != protocol.RouteFast && model != protocol.RouteSmart
}
