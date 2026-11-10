package kit

// State forces an interactive visual state for static previews.
type State uint8

const (
	StateRest State = iota
	StateHover
	StatePressed
	StateFocus
)
