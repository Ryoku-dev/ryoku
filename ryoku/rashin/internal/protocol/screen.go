package protocol

// ScreenState is an Agent session's own screen: a hidden session the agent
// drives apps on, which the user can watch and take over.
type ScreenState struct {
	Session string `json:"session"`
	// State is starting, running or stopped.
	State  string `json:"state"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	// Frame is the path of the latest frame, a PNG the daemon replaces whole;
	// FrameSeq counts frames so a client knows to read it again.
	Frame    string `json:"frame,omitempty"`
	FrameSeq int64  `json:"frameSeq,omitempty"`
	// Holder is agent or user: whoever may act on the screen now.
	Holder string `json:"holder"`
	// Action is the agent's latest action, in words.
	Action string `json:"action,omitempty"`
	// Ask is what the agent needs the user to do on the screen while it
	// waits for them to hand it back.
	Ask   string `json:"ask,omitempty"`
	Error string `json:"error,omitempty"`
}

// Screen states and holders.
const (
	ScreenStarting = "starting"
	ScreenRunning  = "running"
	ScreenStopped  = "stopped"
	HolderAgent    = "agent"
	HolderUser     = "user"
)

// ScreenAction is one thing done on a screen. Kind is click, double, right,
// middle, move, drag, press, release, scroll, type, key, open or wait; X and
// Y are screen pixels, ToX and ToY a drag's end, DX and DY wheel notches,
// Text what to type, Keys a chord such as ctrl+l, Command a command line to
// open and Seconds how long to wait.
type ScreenAction struct {
	Kind    string  `json:"kind"`
	X       int     `json:"x,omitempty"`
	Y       int     `json:"y,omitempty"`
	ToX     int     `json:"toX,omitempty"`
	ToY     int     `json:"toY,omitempty"`
	DX      int     `json:"dx,omitempty"`
	DY      int     `json:"dy,omitempty"`
	Text    string  `json:"text,omitempty"`
	Keys    string  `json:"keys,omitempty"`
	Command string  `json:"command,omitempty"`
	Seconds float64 `json:"seconds,omitempty"`
}

// ScreenActParams acts on a session's screen. By is agent or user; an
// agent's first action asks the user to give it a screen, and its actions
// wait while the user holds the screen. Look asks for the frame after it.
type ScreenActParams struct {
	ID     string       `json:"id"`
	By     string       `json:"by"`
	Action ScreenAction `json:"action"`
	Look   bool         `json:"look,omitempty"`
}

// ScreenActResult is the screen after an action; Image is the settled
// frame as base64 PNG when Look was set. Waiting, for an agent's action, says
// it was not done because the call stopped waiting first: on consent, the
// user's answer to the ask for a screen, or on user, the user having the
// screen. Calling again goes on waiting.
type ScreenActResult struct {
	Screen  ScreenState `json:"screen"`
	Image   string      `json:"image,omitempty"`
	Waiting string      `json:"waiting,omitempty"`
}

// What an agent's screen action still waits on.
const (
	ScreenWaitConsent = "consent"
	ScreenWaitUser    = "user"
)

// ScreenControlParams changes who holds a screen or ends it. Op is take,
// release, ask, watch, unwatch or stop; Reason is what an ask needs from the
// user.
type ScreenControlParams struct {
	ID     string `json:"id"`
	Op     string `json:"op"`
	Reason string `json:"reason,omitempty"`
}

// Screen control operations.
const (
	ScreenTake    = "take"
	ScreenRelease = "release"
	ScreenAsk     = "ask"
	ScreenWatch   = "watch"
	ScreenUnwatch = "unwatch"
	ScreenStop    = "stop"
)

// ScreensResult lists the running screens.
type ScreensResult struct {
	Screens []ScreenState `json:"screens"`
}
