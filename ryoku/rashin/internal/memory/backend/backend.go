// Package backend is the shape every engine that keeps memory has: the
// built-in store and the optional Hindsight and Honcho engines. It sits apart
// from package memory so the engines can implement it without importing the
// engine shell that opens them.
//
// A fact is a protocol.Memory. Its Scope is protocol.MemoryYou,
// protocol.MemoryProjectPrefix plus a project key, or protocol.MemoryTeamPrefix
// plus a team id. An engine fills ID, Scope, Text, Created and as much of
// Source as it keeps; the engine shell adds names, uses and versions.
package backend

import (
	"context"
	"time"

	"ryoku-rashin/internal/protocol"
)

// Caps are what an engine can do. MemoryCaps is what the app greys out;
// Learns says the engine extracts facts from Observe by itself, so Rashin's
// own learner stands aside, and Card says it writes its own card about the
// user.
type Caps struct {
	protocol.MemoryCaps
	Learns bool
	Card   bool
}

// Query asks an engine for the facts that bear on Text, in Scopes (every
// scope when empty), at most Limit of them, best first.
type Query struct {
	Text   string
	Scopes []string
	Limit  int
}

// Turn is one turn of a conversation handed to an engine that learns: the
// user's words, the final reply and the user's answers to questions and
// permission requests. The caller has already scrubbed it; tool output and
// web text never reach it.
type Turn struct {
	Seq     int64
	At      time.Time
	User    string
	Reply   string
	Answers []string
}

// Transcript is what Rashin hands an engine that learns: the new turns of
// one conversation. Kind is protocol.MemoryFromAsk or MemoryFromAgent and
// Scope the scope what is learned belongs to.
type Transcript struct {
	Session string
	Kind    string
	Title   string
	Scope   string
	// Model is the provider/model that answered the conversation, for an
	// engine that learns through Rashin's own model choice.
	Model string
	Turns []Turn
}

// Status is an engine's state for MemoryStatus: State is one of the
// protocol.Engine* values, Learning counts work the engine still has queued,
// and Detail says in words what is wrong. Detail never holds a secret.
type Status struct {
	State    string
	Detail   string
	Learning int
}

// Backend keeps memory. Every method may be called from several goroutines.
type Backend interface {
	// Caps says what the engine can do.
	Caps() Caps
	// Remember stores facts as they are and returns them, in the same order,
	// with the engine's ids.
	Remember(ctx context.Context, facts []protocol.Memory) ([]protocol.Memory, error)
	// Edit changes a fact's text. An engine that cannot edit in place
	// forgets the fact and remembers the new text, so the id may change.
	Edit(ctx context.Context, id, text string) (protocol.Memory, error)
	// Recall returns the facts that bear on a query, best first.
	Recall(ctx context.Context, q Query) ([]protocol.Memory, error)
	// Observe hands the engine new turns to learn from. It returns once the
	// engine holds them; on an error the caller hands the same turns again
	// later.
	Observe(ctx context.Context, t Transcript) error
	// List returns one page of facts and how many there are in all. Scope is
	// empty for every scope, protocol.MemoryYou, MemoryProjects, MemoryTeams
	// or one exact scope. A limit of 0 returns only the count.
	List(ctx context.Context, scope string, offset, limit int) ([]protocol.Memory, int, error)
	// Forget removes facts by id.
	Forget(ctx context.Context, ids []string) error
	// ForgetScope removes every fact in one exact scope, in every scope of a
	// group (protocol.MemoryProjects or MemoryTeams), or everything when
	// scope is empty.
	ForgetScope(ctx context.Context, scope string) error
	// Card returns the card the engine wrote about the user, empty when it
	// has none.
	Card(ctx context.Context) (string, error)
	// Status says how the engine is.
	Status(ctx context.Context) Status
	// Busy is true while a call is in flight or the engine was asked by
	// Settle to finish its work.
	Busy() bool
	// Settle is called once at the daemon's idle deadline. An engine that
	// still has work keeps Busy true for one bounded drain window; it never
	// polls.
	Settle(ctx context.Context)
	// Close stops anything the engine runs. A closed backend is not used
	// again.
	Close() error
}

// Secrets reads and keeps values in Rashin's credential vault by credential
// name. A value never reaches a client, a log or an error.
type Secrets interface {
	// Secret returns the value of the credential called name.
	Secret(name string) (string, error)
	// Keep seals value under name, replacing the value of a credential that
	// already has the name.
	Keep(name, value string) error
}
