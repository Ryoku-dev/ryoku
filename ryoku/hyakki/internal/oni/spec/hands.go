package spec

import "time"

// Hands holds the numbers for Hands in each of its exclusive stages: one
// browser tab the user authorized, one live window the user picked, or the
// separate nested desktop Hyakki owns. Hands never drives the user's whole
// live desktop, so no number here describes one. The section holds the
// step budget, the halt and rejection limits, the pause windows, the
// carried history, the consent card's lifetime, the per-step time budgets,
// the bounds on what a page may put in front of the model, the edge glow,
// the ledger, the live-window rules, the nested desktop, the task budget
// and the journal's retention. Consumers read these values; none repeats
// them as literals.
//
// A task takes at most Budget.Steps steps, and Budget.Extra more only when
// no step was refused or rejected. The same action on the same control
// RepeatLimit times in a row halts the task. A "done" that the screen or
// the read-back contradicts is rejected; DoneRejections of them end the
// task. A paused task may be resumed for ResumeS; after a finished task a
// follow-up within FollowUpS continues it without an anchor phrase. The
// last HistoryCarry steps go with each model request. A consent card
// lapses after ConsentTTLS. One model request gets ReadTimeoutS, one
// read-back of a filled value ReadbackMs, one action ActMs, and a stop must
// abort the model stream and remove every visible indication within
// StopBudgetMs.
type Hands struct {
	Budget         HandsBudget    `json:"budget"`
	RepeatLimit    int            `json:"repeatLimit"`
	DoneRejections int            `json:"doneRejections"`
	ResumeS        float64        `json:"resumeS"`
	FollowUpS      float64        `json:"followUpS"`
	HistoryCarry   int            `json:"historyCarry"`
	ConsentTTLS    float64        `json:"consentTtlS"`
	ReadTimeoutS   float64        `json:"readTimeoutS"`
	ReadbackMs     int            `json:"readbackMs"`
	ActMs          int            `json:"actMs"`
	StopBudgetMs   int            `json:"stopBudgetMs"`
	Page           HandsPage      `json:"page"`
	Glow           HandsGlow      `json:"glow"`
	Ledger         HandsLedger    `json:"ledger"`
	App            HandsApp       `json:"app"`
	Desktop        HandsDesktop   `json:"desktop"`
	Task           HandsTask      `json:"task"`
	Retention      HandsRetention `json:"retention"`
}

// HandsBudget: a task takes Steps steps, plus Extra when every step so far
// went through without a refusal or a rejection.
type HandsBudget struct {
	Steps int `json:"steps"`
	Extra int `json:"extra"`
}

// HandsPage bounds what one page read may hand the model: at most
// MaxControls controls, each name cut to MaxNameRunes, the visible text cut
// to MaxTextRunes, and one fill at most MaxFillRunes long.
type HandsPage struct {
	MaxControls  int `json:"maxControls"`
	MaxNameRunes int `json:"maxNameRunes"`
	MaxTextRunes int `json:"maxTextRunes"`
	MaxFillRunes int `json:"maxFillRunes"`
}

// HandsGlow is the edge glow shown while Hands works: four StripPx strips
// whose opacity breathes between Low and High over PeriodS, repainted FPS
// times a second, scaled by WaitingFactor while a card waits. It is removed
// at once on stop, with no fade.
type HandsGlow struct {
	FPS           int     `json:"fps"`
	Low           float64 `json:"low"`
	High          float64 `json:"high"`
	PeriodS       float64 `json:"periodS"`
	StripPx       int     `json:"stripPx"`
	WaitingFactor float64 `json:"waitingFactor"`
}

// HandsLedger: the Hands ledger keeps at most MaxRows rows, oldest out
// first.
type HandsLedger struct {
	MaxRows int `json:"maxRows"`
}

// HandsApp holds the rules for one live window the user picked. Before and
// after every action the window is checked against its baseline within
// VerifyMs; a box that moves or grows by more than GeometryTolPx pauses the
// task. Text goes in chunks of at most TypeChunkRunes runes, each typing
// call waiting SettleMs for the keymap before its first key and taking at
// most ChunkMs; one type action carries at most MaxTypeRunes runes. The OCR
// read-back of typed text gets ReadbackMs.
type HandsApp struct {
	GeometryTolPx  int `json:"geometryTolPx"`
	VerifyMs       int `json:"verifyMs"`
	SettleMs       int `json:"settleMs"`
	ChunkMs        int `json:"chunkMs"`
	ReadbackMs     int `json:"readbackMs"`
	TypeChunkRunes int `json:"typeChunkRunes"`
	MaxTypeRunes   int `json:"maxTypeRunes"`
}

// HandsDesktop holds the limits of the separate nested desktop Hyakki owns:
// one WidthPx×HeightPx output that must report ready within ReadyS. A stop
// asks it to end and kills it after StopGraceS; with no task it ends after
// IdleS.
type HandsDesktop struct {
	WidthPx    int     `json:"widthPx"`
	HeightPx   int     `json:"heightPx"`
	ReadyS     float64 `json:"readyS"`
	StopGraceS float64 `json:"stopGraceS"`
	IdleS      float64 `json:"idleS"`
}

// HandsTask is the task supervisor's budget: at most Turns model turns and
// Ms milliseconds a task, and HostedUsd of measured usage, enforced only on
// a hosted route that reports usage. QueueMax tasks may wait behind the
// running one.
type HandsTask struct {
	Turns     int     `json:"turns"`
	Ms        int     `json:"ms"`
	HostedUsd float64 `json:"hostedUsd"`
	QueueMax  int     `json:"queueMax"`
}

// HandsRetention: the task journal and projects keep a row at most Days
// days and at most Rows rows, oldest out first.
type HandsRetention struct {
	Days int `json:"days"`
	Rows int `json:"rows"`
}

func millis(ms int) time.Duration { return time.Duration(ms) * time.Millisecond }

// Max is the most steps one task may take.
func (b HandsBudget) Max() int { return b.Steps + b.Extra }

// Resume is how long a paused task may be resumed.
func (h Hands) Resume() time.Duration { return seconds(h.ResumeS) }

// FollowUp is how long after a task a follow-up needs no anchor phrase.
func (h Hands) FollowUp() time.Duration { return seconds(h.FollowUpS) }

// ConsentTTL is how long a Hands consent card waits before it lapses.
func (h Hands) ConsentTTL() time.Duration { return seconds(h.ConsentTTLS) }

// ReadTimeout is the time one model request gets.
func (h Hands) ReadTimeout() time.Duration { return seconds(h.ReadTimeoutS) }

// Readback is the time one read-back of a filled value gets.
func (h Hands) Readback() time.Duration { return millis(h.ReadbackMs) }

// Act is the time one action gets.
func (h Hands) Act() time.Duration { return millis(h.ActMs) }

// StopBudget is the longest a stop may take to abort the model stream and
// remove every visible indication.
func (h Hands) StopBudget() time.Duration { return millis(h.StopBudgetMs) }

// Period is one full breath of the edge glow.
func (g HandsGlow) Period() time.Duration { return seconds(g.PeriodS) }

// Frame is the time between two glow repaints.
func (g HandsGlow) Frame() time.Duration { return time.Second / time.Duration(g.FPS) }

// Verify is the time one before- or after-check of a live window gets.
func (a HandsApp) Verify() time.Duration { return millis(a.VerifyMs) }

// Settle is how long one typing call waits for the keymap before its
// first key.
func (a HandsApp) Settle() time.Duration { return millis(a.SettleMs) }

// Chunk is the time one typing call gets.
func (a HandsApp) Chunk() time.Duration { return millis(a.ChunkMs) }

// Readback is the time one OCR read-back of typed text gets.
func (a HandsApp) Readback() time.Duration { return millis(a.ReadbackMs) }

// Ready is how long the nested desktop may take to report ready.
func (d HandsDesktop) Ready() time.Duration { return seconds(d.ReadyS) }

// StopGrace is how long a stopping nested desktop gets before it is killed.
func (d HandsDesktop) StopGrace() time.Duration { return seconds(d.StopGraceS) }

// Idle is how long the nested desktop outlives its last task.
func (d HandsDesktop) Idle() time.Duration { return seconds(d.IdleS) }

// Time is the longest one task may run.
func (t HandsTask) Time() time.Duration { return millis(t.Ms) }

// Age is how long a journal or project row is kept.
func (r HandsRetention) Age() time.Duration { return time.Duration(r.Days) * 24 * time.Hour }

// handsGlowMaxFPS is the most repaints a second the glow may ask for; the
// glow is a cheap breathing layer, never a full-rate animation.
const handsGlowMaxFPS = 30

// handsTypeChunkMaxRunes is the contract's ceiling on one typing call: no
// chunk is ever longer, whatever the spec says.
const handsTypeChunkMaxRunes = 40

// handsGeometryTolMaxPx is the widest box tolerance the loader accepts; a
// wider one would let a real move or resize through without a pause.
const handsGeometryTolMaxPx = 8

func (s *OniSpec) checkHands(bad func(string, ...any)) {
	h := s.Hands
	for _, d := range []struct {
		path string
		v    float64
	}{
		{"hands.resumeS", h.ResumeS},
		{"hands.followUpS", h.FollowUpS},
		{"hands.consentTtlS", h.ConsentTTLS},
		{"hands.readTimeoutS", h.ReadTimeoutS},
		{"hands.glow.periodS", h.Glow.PeriodS},
		{"hands.desktop.readyS", h.Desktop.ReadyS},
		{"hands.desktop.stopGraceS", h.Desktop.StopGraceS},
		{"hands.desktop.idleS", h.Desktop.IdleS},
		{"hands.task.hostedUsd", h.Task.HostedUsd},
	} {
		if !(d.v > 0) {
			bad("%s: must be positive", d.path)
		}
	}
	for _, d := range []struct {
		path string
		v    int
	}{
		{"hands.budget.steps", h.Budget.Steps},
		{"hands.readbackMs", h.ReadbackMs},
		{"hands.actMs", h.ActMs},
		{"hands.stopBudgetMs", h.StopBudgetMs},
		{"hands.page.maxControls", h.Page.MaxControls},
		{"hands.page.maxNameRunes", h.Page.MaxNameRunes},
		{"hands.page.maxTextRunes", h.Page.MaxTextRunes},
		{"hands.page.maxFillRunes", h.Page.MaxFillRunes},
		{"hands.glow.stripPx", h.Glow.StripPx},
		{"hands.ledger.maxRows", h.Ledger.MaxRows},
		{"hands.app.verifyMs", h.App.VerifyMs},
		{"hands.app.settleMs", h.App.SettleMs},
		{"hands.app.chunkMs", h.App.ChunkMs},
		{"hands.app.readbackMs", h.App.ReadbackMs},
		{"hands.app.maxTypeRunes", h.App.MaxTypeRunes},
		{"hands.desktop.widthPx", h.Desktop.WidthPx},
		{"hands.desktop.heightPx", h.Desktop.HeightPx},
		{"hands.task.turns", h.Task.Turns},
		{"hands.task.ms", h.Task.Ms},
		{"hands.task.queueMax", h.Task.QueueMax},
		{"hands.retention.days", h.Retention.Days},
		{"hands.retention.rows", h.Retention.Rows},
	} {
		if d.v < 1 {
			bad("%s: must be at least 1", d.path)
		}
	}

	steps := h.Budget.Steps
	if !(h.RepeatLimit >= 2 && h.RepeatLimit <= steps) {
		bad("hands.repeatLimit: must lie in [2, budget.steps]")
	}
	if !(h.DoneRejections >= 1 && h.DoneRejections < steps) {
		bad("hands.doneRejections: must lie in [1, budget.steps)")
	}
	if !(h.HistoryCarry >= 1 && h.HistoryCarry <= steps) {
		bad("hands.historyCarry: must lie in [1, budget.steps]")
	}
	if h.ConsentTTLS > 0 && !(h.ConsentTTLS <= h.ResumeS) {
		bad("hands.consentTtlS: a waiting card must lapse within the resume window (resumeS)")
	}
	if h.StopBudgetMs >= 1 && !(h.StopBudgetMs < h.ReadbackMs && h.ReadbackMs < h.ActMs) {
		bad("hands.stopBudgetMs: want stopBudgetMs < readbackMs < actMs, so a stop outruns every step")
	}
	if h.ReadTimeoutS > 0 && !(h.ReadTimeoutS*1000 > float64(h.ActMs) && h.ReadTimeoutS <= h.ResumeS) {
		bad("hands.readTimeoutS: want actMs < readTimeoutS·1000 and readTimeoutS <= resumeS")
	}

	p := h.Page
	if p.MaxNameRunes >= 1 && !(p.MaxNameRunes < p.MaxTextRunes) {
		bad("hands.page.maxNameRunes: a control name must be shorter than the page text bound")
	}
	if p.MaxFillRunes >= 1 && !(p.MaxFillRunes <= p.MaxTextRunes) {
		bad("hands.page.maxFillRunes: a fill must fit in the page text the read-back sees")
	}

	g := h.Glow
	if !(g.FPS >= 1 && g.FPS <= handsGlowMaxFPS) {
		bad("hands.glow.fps: must lie in [1, %d]", handsGlowMaxFPS)
	}
	if !(g.Low > 0 && g.Low < g.High && g.High <= 1) {
		bad("hands.glow: want 0 < low < high <= 1")
	}
	if g.FPS >= 1 && g.PeriodS > 0 && !(g.PeriodS*float64(g.FPS) >= 2) {
		bad("hands.glow.periodS: one breath must span at least two frames")
	}
	if !(g.WaitingFactor > 0 && g.WaitingFactor < 1) {
		bad("hands.glow.waitingFactor: must lie in (0, 1)")
	}

	a := h.App
	if a.GeometryTolPx > handsGeometryTolMaxPx {
		bad("hands.app.geometryTolPx: must lie in [0, %d]", handsGeometryTolMaxPx)
	}
	if !(a.TypeChunkRunes >= 1 && a.TypeChunkRunes <= handsTypeChunkMaxRunes) {
		bad("hands.app.typeChunkRunes: must lie in [1, %d]", handsTypeChunkMaxRunes)
	}
	if a.MaxTypeRunes >= 1 && a.TypeChunkRunes >= 1 && !(a.TypeChunkRunes <= a.MaxTypeRunes && a.MaxTypeRunes <= p.MaxFillRunes) {
		bad("hands.app.maxTypeRunes: want typeChunkRunes <= maxTypeRunes <= page.maxFillRunes")
	}
	if a.VerifyMs >= 1 && !(h.StopBudgetMs < a.VerifyMs && a.VerifyMs < a.ChunkMs) {
		bad("hands.app.verifyMs: want stopBudgetMs < verifyMs < chunkMs, so a stop outruns a check and a check fits a typing call")
	}
	if a.SettleMs >= 1 && !(a.SettleMs < a.ChunkMs) {
		bad("hands.app.settleMs: a typing call must outlast its keymap wait (settleMs < chunkMs)")
	}
	if a.ReadbackMs >= 1 && !(h.StopBudgetMs < a.ReadbackMs) {
		bad("hands.app.readbackMs: a stop must outrun a typed read-back (stopBudgetMs < readbackMs)")
	}

	d := h.Desktop
	if d.StopGraceS > 0 && !(d.StopGraceS < d.ReadyS) {
		bad("hands.desktop.stopGraceS: a stop must end the nested desktop sooner than a start may take (stopGraceS < readyS)")
	}
	if d.IdleS > 0 && !(h.ResumeS <= d.IdleS) {
		bad("hands.desktop.idleS: the nested desktop must outlive the resume window (resumeS <= idleS)")
	}

	t := h.Task
	if t.Turns >= 1 && !(t.Turns >= h.Budget.Max()) {
		bad("hands.task.turns: a task must allow one full step budget (turns >= budget.steps + budget.extra)")
	}
	if t.Ms >= 1 && !(h.ResumeS*1000 < float64(t.Ms)) {
		bad("hands.task.ms: a task must outlast the resume window (resumeS·1000 < ms)")
	}
	if t.QueueMax >= 1 && h.Retention.Rows >= 1 && !(t.QueueMax < h.Retention.Rows) {
		bad("hands.task.queueMax: the journal must hold the running task and a full queue (queueMax < retention.rows)")
	}
}
