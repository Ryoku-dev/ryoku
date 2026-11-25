package protocol

import (
	"path"
	"strings"
	"time"
)

// OniRecallSettings is Settings.oni.recall; every switch is off until the
// user turns it on. Enabled answers "what did I ask about" and "what do my
// notes say" from the chats the user imported, past conversations and the
// memory facts. Vault adds passages from the Ryoku vault, read only; they
// are never included while it is off.
type OniRecallSettings struct {
	Enabled bool `json:"enabled"`
	Vault   bool `json:"vault"`
}

// Rewrite styles: fix the grammar, make it friendlier, more formal, more
// casual, more polite or more confident, shorter, longer or simpler, turn
// it into an email or into bullets. Custom follows the user's own
// instruction and is never a setting.
const (
	OniRewriteGrammar   = "grammar"
	OniRewriteFriendly  = "friendly"
	OniRewriteFormal    = "formal"
	OniRewriteCasual    = "casual"
	OniRewritePolite    = "polite"
	OniRewriteConfident = "confident"
	OniRewriteShorter   = "shorter"
	OniRewriteLonger    = "longer"
	OniRewriteSimpler   = "simpler"
	OniRewriteEmail     = "email"
	OniRewriteBullets   = "bullets"
	OniRewriteCustom    = "custom"
)

// OniRewriteStyles are every style a setting may name; custom is not one.
var OniRewriteStyles = []string{
	OniRewriteGrammar, OniRewriteFriendly, OniRewriteFormal, OniRewriteCasual, OniRewritePolite, OniRewriteConfident,
	OniRewriteShorter, OniRewriteLonger, OniRewriteSimpler, OniRewriteEmail, OniRewriteBullets,
}

// OniRewriteSettings is Settings.oni.rewrite, off until the user turns it
// on. Style is the style used when none is named. Voice writes in the
// user's own voice from their "My voice" notes; nothing else from memory
// goes into a rewrite.
type OniRewriteSettings struct {
	Enabled bool   `json:"enabled"`
	Style   string `json:"style"`
	Voice   bool   `json:"voice"`
}

// Assignment styles: the answer alone, or the steps that lead to it.
const (
	OniAssignmentWalkthrough = "walkthrough"
	OniAssignmentAnswers     = "answers"
)

// OniAssignmentStyles are every assignment style.
var OniAssignmentStyles = []string{OniAssignmentWalkthrough, OniAssignmentAnswers}

// OniAssignmentSettings is Settings.oni.assignment, off until the user turns
// it on; it also needs oni.inspect. Model is the user's own pin for the
// solve: empty asks the smallest local model first and never a paid one.
// A hosted pin is still shown on a disclosure card before anything leaves.
type OniAssignmentSettings struct {
	Enabled bool   `json:"enabled"`
	Style   string `json:"style"`
	Model   string `json:"model"`
}

// ValidOniAssignmentModel reports whether m may be an assignment's pin:
// empty, or a model the gateway can name.
func ValidOniAssignmentModel(m string) bool {
	return m == "" || m == strings.TrimSpace(m) && ValidModel(m)
}

// Limits on Phase 16 call values, in runes.
const (
	// OniRewriteInstructionMax bounds a custom rewrite's instruction.
	OniRewriteInstructionMax = 300
	// OniRecallTextMax bounds a recall question.
	OniRecallTextMax = 2000
	// OniRecallTopicMax bounds what "what do you know about" names.
	OniRecallTopicMax = 300
	// OniFocusAppMax bounds the app a guard lasts until.
	OniFocusAppMax = 128
	// OniCardIDMax bounds a card or record id, in bytes.
	OniCardIDMax = 64
)

// What oni.rewrite does: read the selection and show it with the route on
// a card (start), answer that card yes, so the text is sent and the result
// typed back over the selection (apply), put the original back (undo), or
// drop the card (cancel). Apply, undo and cancel name the card or record.
const (
	OniRewriteStart  = "start"
	OniRewriteApply  = "apply"
	OniRewriteUndo   = "undo"
	OniRewriteCancel = "cancel"
)

// OniRewriteActions are every oni.rewrite action.
var OniRewriteActions = []string{OniRewriteStart, OniRewriteApply, OniRewriteUndo, OniRewriteCancel}

// OniRewriteParams is one oni.rewrite call. Style, one of OniRewriteStyles
// or custom, and Instruction, custom's alone, come with a start only.
type OniRewriteParams struct {
	Action      string `json:"action"`
	ID          string `json:"id,omitempty"`
	Style       string `json:"style,omitempty"`
	Instruction string `json:"instruction,omitempty"`
}

// What an oni.rewrite came to: a card waits for the user (consent); the
// result was typed over the selection (applied); it already read well and
// nothing was typed (same); the result is offered to copy instead of typed
// (copy), as a result of more than one paragraph always is; the original
// is back (undone); undo could not be checked, so the original is offered
// to copy (kept); the user said no or the card lapsed (declined); or it was
// refused before anything was sent or typed (refused).
const (
	OniRewriteConsent  = "consent"
	OniRewriteApplied  = "applied"
	OniRewriteSame     = "same"
	OniRewriteCopy     = "copy"
	OniRewriteUndone   = "undone"
	OniRewriteKept     = "kept"
	OniRewriteDeclined = "declined"
	OniRewriteRefused  = "refused"
)

// OniRewriteStatuses are every oni.rewrite status.
var OniRewriteStatuses = []string{OniRewriteConsent, OniRewriteApplied, OniRewriteSame, OniRewriteCopy, OniRewriteUndone, OniRewriteKept, OniRewriteDeclined, OniRewriteRefused}

// Why a rewrite was refused: the feature is off; no selection could be
// read; it is longer than the spec allows; it looks like a secret or a
// private number; the window is a private field, a password manager,
// Hyakki itself or a terminal; the selection or the window changed since
// it was read; the result added facts the selection did not hold; or the
// model failed.
const (
	OniRewriteRefusedOff       = "off"
	OniRewriteRefusedNoText    = "no-selection"
	OniRewriteRefusedTooLong   = "too-long"
	OniRewriteRefusedSecret    = "secret"
	OniRewriteRefusedPrivate   = "private-window"
	OniRewriteRefusedSelf      = "self"
	OniRewriteRefusedTerminal  = "terminal"
	OniRewriteRefusedChanged   = "changed"
	OniRewriteRefusedWindow    = "window"
	OniRewriteRefusedFacts     = "facts-added"
	OniRewriteRefusedModel     = "model"
	OniRewriteRefusedNotPress  = "needs-press"
	OniRewriteRefusedNoUndo    = "no-undo"
	OniRewriteRefusedUnknownID = "unknown-id"
)

// OniRewriteRefusals are every rewrite refusal.
var OniRewriteRefusals = []string{
	OniRewriteRefusedOff, OniRewriteRefusedNoText, OniRewriteRefusedTooLong, OniRewriteRefusedSecret, OniRewriteRefusedPrivate,
	OniRewriteRefusedSelf, OniRewriteRefusedTerminal, OniRewriteRefusedChanged, OniRewriteRefusedWindow, OniRewriteRefusedFacts,
	OniRewriteRefusedModel, OniRewriteRefusedNotPress, OniRewriteRefusedNoUndo, OniRewriteRefusedUnknownID,
}

// OniRewriteResult answers oni.rewrite. ID names the card (consent) or the
// record undo takes (applied). Chars is the selection's length in runes;
// Destination and Route say where it goes. Text goes only to the app or
// the CLI that asked, never on a frame: the selection on a consent, the
// result on a copy, the original on a kept. Landed runes were typed, Rest
// was not. UndoVerified is true only where putting the original back was
// checked for that kind of window. ExpiresMs is how long the card or the
// undo stays.
type OniRewriteResult struct {
	Status       string `json:"status"`
	ID           string `json:"id,omitempty"`
	Style        string `json:"style,omitempty"`
	Chars        int    `json:"chars,omitempty"`
	Destination  string `json:"destination,omitempty"`
	Route        string `json:"route,omitempty"`
	Text         string `json:"text,omitempty"`
	Landed       int    `json:"landed,omitempty"`
	Rest         string `json:"rest,omitempty"`
	Refused      string `json:"refused,omitempty"`
	UndoVerified bool   `json:"undoVerified,omitempty"`
	ExpiresMs    int64  `json:"expiresMs,omitempty"`
}

// What oni.recall does: answer a question from what the user kept and
// imported (ask), or say what Oni knows about the user or a topic (know).
const (
	OniRecallAsk  = "ask"
	OniRecallKnow = "know"
)

// OniRecallActions are every oni.recall action.
var OniRecallActions = []string{OniRecallAsk, OniRecallKnow}

// OniRecallParams is one oni.recall call: the question for ask, the
// optional topic for know.
type OniRecallParams struct {
	Action string `json:"action"`
	Text   string `json:"text,omitempty"`
}

// Where a recalled passage came from: a chat the user imported, the Ryoku
// vault, a past Hyakki conversation, or a memory fact.
const (
	OniRecallChat         = "chat"
	OniRecallVault        = "vault"
	OniRecallConversation = "conversation"
	OniRecallFact         = "fact"
)

// OniRecallKinds are every recalled source kind.
var OniRecallKinds = []string{OniRecallChat, OniRecallVault, OniRecallConversation, OniRecallFact}

// What an oni.recall came to: passages wait behind a hosted route's card
// (consent), an answer came back (answered), nothing matched (empty), or it
// was refused (refused).
const (
	OniRecallStatusConsent  = "consent"
	OniRecallStatusAnswered = "answered"
	OniRecallStatusEmpty    = "empty"
	OniRecallStatusRefused  = "refused"
)

// OniRecallStatuses are every oni.recall status.
var OniRecallStatuses = []string{OniRecallStatusConsent, OniRecallStatusAnswered, OniRecallStatusEmpty, OniRecallStatusRefused}

// OniRecallSource is one passage an answer drew on: its kind, title, the
// app or place it came from and when. Never the passage itself.
type OniRecallSource struct {
	Kind  string    `json:"kind"`
	Title string    `json:"title"`
	From  string    `json:"from,omitempty"`
	At    time.Time `json:"at,omitzero"`
}

// OniRecallConsent is the card shown before passages go to a hosted route:
// the route and the passages' titles. oni.consent.answer answers ID within
// ExpiresMs. A local route sends nothing off the computer and asks nothing.
type OniRecallConsent struct {
	ID        string   `json:"id"`
	Route     string   `json:"route"`
	Question  string   `json:"question"`
	Titles    []string `json:"titles"`
	ExpiresMs int64    `json:"expiresMs"`
}

// OniRecallResult answers oni.recall.
type OniRecallResult struct {
	Status      string            `json:"status"`
	Consent     *OniRecallConsent `json:"consent,omitempty"`
	Answer      string            `json:"answer,omitempty"`
	Sources     []OniRecallSource `json:"sources,omitempty"`
	Route       string            `json:"route,omitempty"`
	Destination string            `json:"destination,omitempty"`
	Refused     string            `json:"refused,omitempty"`
}

// Chat exports oni.recall.import reads: ChatGPT's, Claude's and Gemini's.
const (
	OniImportChatGPT = "chatgpt"
	OniImportClaude  = "claude"
	OniImportGemini  = "gemini"
)

// OniImportSources are every chat export oni.recall.import reads.
var OniImportSources = []string{OniImportChatGPT, OniImportClaude, OniImportGemini}

// Limits on oni.recall.import.
const (
	// OniImportPathsMax bounds the files one import names.
	OniImportPathsMax = 16
	// OniImportPathMax bounds one path, in bytes.
	OniImportPathMax = 4096
)

// OniRecallImportParams names the export files, .json or .zip, to import:
// absolute, clean paths, at most OniImportPathsMax of them. The files are
// read and left where they are; only the chats, scrubbed, are kept.
type OniRecallImportParams struct {
	Paths []string `json:"paths"`
}

// ValidOniImportPath reports whether p may name an export file: an
// absolute, clean path of at most OniImportPathMax bytes ending .json or
// .zip, free of control characters.
func ValidOniImportPath(p string) bool {
	if len(p) > OniImportPathMax || !strings.HasPrefix(p, "/") || !ValidOniText(p, OniImportPathMax, false) {
		return false
	}
	if path.Clean(p) != p {
		return false
	}
	ext := strings.ToLower(path.Ext(p))
	return ext == ".json" || ext == ".zip"
}

// OniRecallImported is how many chats one export source gave.
type OniRecallImported struct {
	From  string `json:"from"`
	Chats int    `json:"chats"`
}

// OniRecallImportResult answers oni.recall.import: the chats imported and
// skipped, per source, the index root they are kept under, and the
// refusal, if any.
type OniRecallImportResult struct {
	Chats   int                 `json:"chats"`
	Skipped int                 `json:"skipped"`
	Sources []OniRecallImported `json:"sources"`
	Root    string              `json:"root"`
	Refused string              `json:"refused,omitempty"`
}

// What oni.focus does: report the guard (status), arm it for Minutes or
// until UntilApp closes (arm), stop it (stop), or reopen what it closed
// within the spec's reopen window (back).
const (
	OniFocusStatus = "status"
	OniFocusArm    = "arm"
	OniFocusStop   = "stop"
	OniFocusBack   = "back"
)

// OniFocusActions are every oni.focus action.
var OniFocusActions = []string{OniFocusStatus, OniFocusArm, OniFocusStop, OniFocusBack}

// OniFocusParams is one oni.focus call. An arm names Minutes or UntilApp,
// never both; Words, cleaned as CleanOniFeedWords cleans them, replace the
// Settings feed words for this guard, and empty Words uses them.
type OniFocusParams struct {
	Action   string   `json:"action"`
	Words    []string `json:"words,omitempty"`
	Minutes  int      `json:"minutes,omitempty"`
	UntilApp string   `json:"untilApp,omitempty"`
}

// Why the guard would not arm or act: the feature is off; the desktop
// reports no window events; there is no desktop to watch; no feed words are
// set; or the time asked is past the spec's maximum.
const (
	OniFocusRefusedOff     = "off"
	OniFocusRefusedEvents  = "no-events"
	OniFocusRefusedDesktop = "no-desktop"
	OniFocusRefusedWords   = "no-words"
	OniFocusRefusedTooLong = "too-long"
)

// OniFocusRefusals are every focus refusal.
var OniFocusRefusals = []string{OniFocusRefusedOff, OniFocusRefusedEvents, OniFocusRefusedDesktop, OniFocusRefusedWords, OniFocusRefusedTooLong}

// OniFocusResult answers oni.focus: whether the guard is armed, how long
// it has left or the app it lasts until, its feed words, how many windows
// or tabs it closed and announced, how many "let me back" may still reopen,
// and the refusal, if any. No window title is ever in it.
type OniFocusResult struct {
	Armed     bool     `json:"armed"`
	LeftMs    int64    `json:"leftMs,omitempty"`
	UntilApp  string   `json:"untilApp,omitempty"`
	Words     []string `json:"words,omitempty"`
	Closed    int      `json:"closed,omitempty"`
	Announced int      `json:"announced,omitempty"`
	Reopen    int      `json:"reopen,omitempty"`
	Refused   string   `json:"refused,omitempty"`
}

// What oni.assignment does: start watching the perched window for
// problems, move to the next or back to the last solved view, finish
// (done), stop watching, or report today's spend (status).
const (
	OniAssignmentStart  = "start"
	OniAssignmentNext   = "next"
	OniAssignmentBack   = "back"
	OniAssignmentDone   = "done"
	OniAssignmentStop   = "stop"
	OniAssignmentStatus = "status"
)

// OniAssignmentActions are every oni.assignment action.
var OniAssignmentActions = []string{OniAssignmentStart, OniAssignmentNext, OniAssignmentBack, OniAssignmentDone, OniAssignmentStop, OniAssignmentStatus}

// OniAssignmentParams is one oni.assignment call; Style, one of
// OniAssignmentStyles, overrides the setting and comes with a start only.
type OniAssignmentParams struct {
	Action string `json:"action"`
	Style  string `json:"style,omitempty"`
}

// What an oni.assignment came to: a picture waits behind its card
// (consent), it is watching for a problem (watching), a problem was solved
// (answered), it stopped (stopped), today's cap is reached and nothing was
// sent (capped), it was refused (refused), or it is not running (idle).
const (
	OniAssignmentConsent  = "consent"
	OniAssignmentWatching = "watching"
	OniAssignmentAnswered = "answered"
	OniAssignmentStopped  = "stopped"
	OniAssignmentCapped   = "capped"
	OniAssignmentRefused  = "refused"
	OniAssignmentIdle     = "idle"
)

// OniAssignmentStatuses are every oni.assignment status.
var OniAssignmentStatuses = []string{OniAssignmentConsent, OniAssignmentWatching, OniAssignmentAnswered, OniAssignmentStopped, OniAssignmentCapped, OniAssignmentRefused, OniAssignmentIdle}

// OniAssignmentDay is one local day's assignment spend: Day as YYYY-MM-DD,
// the attempts and measured tokens, the caps from the spec, and whether any
// count was estimated or unmetered. Unmetered usage is flagged, never
// counted as free.
type OniAssignmentDay struct {
	Day         string `json:"day"`
	Attempts    int    `json:"attempts"`
	Tokens      int    `json:"tokens"`
	AttemptsCap int    `json:"attemptsCap"`
	TokensCap   int    `json:"tokensCap"`
	Estimated   bool   `json:"estimated,omitempty"`
	Unmetered   bool   `json:"unmetered,omitempty"`
}

// OniAssignmentResult answers oni.assignment. Consent shows the picture
// and route before it is sent. Answer and Steps come with answered; Step is
// the solved view shown, from 1, and Cached says it came back with no
// model request.
type OniAssignmentResult struct {
	Status  string           `json:"status"`
	Consent *OniConsent      `json:"consent,omitempty"`
	Answer  string           `json:"answer,omitempty"`
	Steps   []string         `json:"steps,omitempty"`
	Step    int              `json:"step,omitempty"`
	Cached  bool             `json:"cached,omitempty"`
	Day     OniAssignmentDay `json:"day"`
	Refused string           `json:"refused,omitempty"`
}

// The lanes of an assignment solve: the fast answer, the checker that
// streams the steps, and the judge that compares the two.
const (
	OniAssignmentLaneFast    = "fast"
	OniAssignmentLaneChecker = "checker"
	OniAssignmentLaneJudge   = "judge"
)

// OniAssignmentLanes are every lane.
var OniAssignmentLanes = []string{OniAssignmentLaneFast, OniAssignmentLaneChecker, OniAssignmentLaneJudge}

// OniAssignmentRow is one assignment ledger row: when, the lane, the route,
// the measured input and output tokens and whether they were estimated or
// unmetered, and the outcome code. Never a picture, a problem, an answer or
// a step.
type OniAssignmentRow struct {
	At        time.Time `json:"at"`
	Lane      string    `json:"lane"`
	Route     string    `json:"route"`
	Input     int       `json:"input"`
	Output    int       `json:"output"`
	Estimated bool      `json:"estimated,omitempty"`
	Unmetered bool      `json:"unmetered,omitempty"`
	Outcome   string    `json:"outcome"`
}

// OniAssignmentLogResult answers oni.assignment.log: today's spend and the
// ledger, newest first.
type OniAssignmentLogResult struct {
	Day  OniAssignmentDay   `json:"day"`
	Rows []OniAssignmentRow `json:"rows"`
}
