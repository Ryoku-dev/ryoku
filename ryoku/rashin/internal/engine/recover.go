package engine

// Derived from oh-my-pi (MIT), packages/coding-agent/src/session/turn-recovery.ts@c7fff67
// Derived from oh-my-pi (MIT), packages/ai/src/utils/tool-call-loop-guard.ts@c7fff67
// Derived from oh-my-pi (MIT), packages/ai/src/utils/thinking-loop.ts@c7fff67
// Derived from oh-my-pi (MIT), packages/coding-agent/src/prompts/system/{empty-stop-retry,length-stop-retry,malformed-function-call-retry,unexpected-stop-retry,tool-call-loop-redirect,thinking-loop-redirect}.md@c7fff67

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"ryoku-rashin/internal/judge/question"
	"ryoku-rashin/internal/llm"
)

// Caps, as omp has them: small models stall and loop far more than large
// ones, and every nudge is a full round on a slow local model.
const (
	retryMax       = 10
	retryBase      = 500 * time.Millisecond
	retryCap       = 8 * time.Second
	nudgeMax       = 3
	loopThreshold  = 5
	interruptMax   = 3
	interruptDelay = 50 * time.Millisecond
	judgeTimeout   = 15 * time.Second
)

// The recovery notes are written to the small-model rules: few
// constraints, said positively, one next step each.
const (
	emptyStopNote = "<system-reminder>\nYour last reply was empty. Reply now with the next tool call, or with your final answer for the user.\nAttempt %d of %d.\n</system-reminder>"

	lengthStopNote = "<system-reminder>\nYour last reply reached the %d-token output limit before it finished, so it was dropped.\n- Think only about the next step, then call its tool.\n- For a large file or answer, write a short first version now and extend it with further edits.\nAttempt %d of %d.\n</system-reminder>"

	lengthCallNote = "Not run: the output limit cut this call off before its arguments were complete. Split the work into smaller calls: write the first part now, then add the rest with edit."

	malformedNote = "<system-reminder>\nYour last tool call could not be read and did not run. Call the tool again through the tool interface, with its arguments as one JSON object that matches the schema. Write no other text.\nAttempt %d of %d.\n</system-reminder>"

	unexpectedStopNote = "<system-reminder>\nYou said you would take a step, then stopped. Call the tool for that step now. If no tool is needed, give your final answer now.\nAttempt %d of %d.\n</system-reminder>"

	stallNote = "<system-reminder>\nYour last reply was cut off by a connection error. Continue exactly where it stopped.\n</system-reminder>"

	loopRedirect = "<system-interrupt reason=\"tool_call_loop_detected\">\nYou called `%s` %d times in a row with the same arguments:\n`%s`\nLast result: `%s`\nCall it with different arguments, use another tool, or answer from what you already have.\n</system-interrupt>"

	loopSkipped = "Not run: the same call as the last ones. Change the arguments, use another tool, or answer."

	closingNote = "Stop calling tools now. Answer the user's question from the tool results above, in a few short sentences. If they were not enough, say what you found and what is still unknown."

	thinkingLoopRedirect = "<system-interrupt reason=\"thinking_loop_detected\">\nYour last reply repeated the same text in a loop and was stopped. Take one concrete step now: call one tool for the smallest next step, or give your final answer if the task is done.\n</system-interrupt>"
)

// recovery counts one turn's nudges. A turn that calls tools resets the
// stop counters: the model is moving again.
type recovery struct {
	empty, length, malformed, unexpected, stall, thinking, interrupts int
}

func (rc *recovery) progressed() {
	rc.empty, rc.length, rc.malformed, rc.unexpected, rc.stall, rc.thinking, rc.interrupts = 0, 0, 0, 0, 0, 0, 0
}

// llamaToolJSON is llama.cpp failing to parse the arguments of a tool call
// it generated. The failure is deterministic: retrying the same request
// fails the same way, so it becomes a malformed-call nudge instead.
var llamaToolJSON = regexp.MustCompile(`(?i)failed to parse tool call arguments as json|\[json\.exception\.parse_error\.101\]`)

// malformedCall reports a provider error that rejected a malformed tool
// call.
func malformedCall(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return llamaToolJSON.MatchString(msg) || strings.Contains(strings.ToLower(msg), "malformed function call")
}

// Only typed HTTP statuses distinguish a provider 429/5xx from numerals in
// an error body. Transport failures have their own narrow word list.
var transientWords = []string{
	"overloaded", "rate limit", "rate_limit", "too many requests",
	"timeout", "timed out", "deadline exceeded", "connection reset", "connection refused", "broken pipe",
	"eof", "temporarily", "network", "fetch failed", "terminated", "stream stall", "socket",
}

type providerStatus interface {
	StatusCode() int
	ContextTooLong() bool
}

func contextTooLong(err error) bool {
	var status providerStatus
	return errors.As(err, &status) && status.ContextTooLong()
}

// retryable reports a provider error worth the same request again.
func retryable(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil || errors.Is(err, context.Canceled) {
		return false
	}
	var refused *llm.NoRoute
	if errors.As(err, &refused) || malformedCall(err) {
		return false
	}
	var status providerStatus
	if errors.As(err, &status) {
		code := status.StatusCode()
		if status.ContextTooLong() || code >= 400 && code < 500 && code != 429 {
			return false
		}
		return code == 429 || code >= 500 && code <= 599
	}
	msg := strings.ToLower(err.Error())
	for _, word := range transientWords {
		if strings.Contains(msg, word) {
			return true
		}
	}
	return false
}

// backoff is the wait before retry attempt n (from 1): 500 ms doubling to
// 8 s, at 75 to 100 percent so retries from many sessions spread out.
func backoff(n int) time.Duration {
	d := retryBase << max(0, n-1)
	if d > retryCap || d <= 0 {
		d = retryCap
	}
	return time.Duration(float64(d) * (0.75 + 0.25*rand.Float64()))
}

// sleep waits d or until ctx ends, and reports whether it waited it out.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// loopGuard counts consecutive rounds whose tool calls are identical.
type loopGuard struct {
	last  string
	count int
	// redirected is set once the model was told to stop repeating; the
	// next identical round ends the turn.
	redirected bool
}

// observe records a round's calls and returns how many identical rounds in
// a row it makes.
func (g *loopGuard) observe(calls []llm.ToolCall) int {
	if len(calls) == 0 {
		g.last, g.count = "", 0
		return 0
	}
	key := callsKey(calls)
	if key == g.last {
		g.count++
	} else {
		g.last, g.count, g.redirected = key, 1, false
	}
	return g.count
}

// callsKey is a round's calls sorted, each as its name and arguments with
// keys sorted, so key order and spacing do not hide a repeat.
func callsKey(calls []llm.ToolCall) string {
	keys := make([]string, 0, len(calls))
	for _, c := range calls {
		keys = append(keys, c.Name+"\x00"+canonicalArgs(c.Arguments))
	}
	sort.Strings(keys)
	return strings.Join(keys, "\x01")
}

func canonicalArgs(raw string) string {
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return strings.TrimSpace(raw)
	}
	// encoding/json sorts map keys when it marshals
	b, err := json.Marshal(v)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return string(b)
}

func malformedArguments(calls []llm.ToolCall) bool {
	for _, call := range calls {
		if call.Name == "" || !strings.HasPrefix(strings.TrimSpace(call.Arguments), "{") || !json.Valid([]byte(call.Arguments)) {
			return true
		}
	}
	return false
}

// loopNote is the redirect for a call repeated count times.
func loopNote(call llm.ToolCall, count int, lastResult string) string {
	args, _ := clipText(canonicalArgs(call.Arguments), 400)
	result, _ := clipText(strings.TrimSpace(lastResult), 200)
	return fmt.Sprintf(loopRedirect, call.Name, count, args, result)
}

// thinkingLoop watches a reply's text and reasoning for an exact repeat
// cycle at its end: short units repeated four times over at least 180
// bytes, or a long unit covering 1024 bytes.
type thinkingLoop struct {
	buf       []byte
	unchecked int
}

const (
	loopTail   = 4096
	loopUnit   = 1024
	loopStride = 128
)

func (t *thinkingLoop) reset() { t.buf, t.unchecked = t.buf[:0], 0 }

// feed adds streamed text and reports a loop.
func (t *thinkingLoop) feed(s string) bool {
	if s == "" {
		return false
	}
	t.buf = append(t.buf, s...)
	if len(t.buf) > 2*loopTail {
		t.buf = append(t.buf[:0], t.buf[len(t.buf)-loopTail:]...)
	}
	t.unchecked += len(s)
	if t.unchecked < loopStride {
		return false
	}
	t.unchecked = 0
	return cycles(t.buf)
}

func cycles(buf []byte) bool {
	tail := buf
	if len(tail) > loopTail {
		tail = tail[len(tail)-loopTail:]
	}
	n := len(tail)
	for unit := 1; unit <= loopUnit && unit*2 <= n; unit++ {
		reps := 1
		for end := n - unit; end-unit >= 0; end -= unit {
			if !bytes.Equal(tail[end-unit:end], tail[end:end+unit]) {
				break
			}
			reps++
		}
		covered := reps * unit
		if unit <= 60 {
			// a ruler of dashes or a table border is not a loop; words are
			if reps >= 4 && covered >= 180 && hasLetter(tail[n-unit:]) {
				return true
			}
			continue
		}
		if reps >= 2 && covered >= 1024 {
			return true
		}
	}
	return false
}

func hasLetter(b []byte) bool {
	for _, r := range string(b) {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// promisesAction is the mechanical half of unexpected-stop detection: a
// reply whose last sentence announces a step it is about to take.
var promisesAction = regexp.MustCompile(`(?i)^(let me|let's|i'll|i will|i'm going to|i am going to|now i'll|next,? i'll|next i will|going to)\b`)

// stoppedShort decides whether a text-only stop left a promised step
// undone: the mechanical check first, then the judge when one is set.
func stoppedShort(ctx context.Context, judge Judge, request, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	last := text
	if i := strings.LastIndexAny(strings.TrimRight(text, ".:!… "), ".!?\n"); i >= 0 {
		last = text[i+1:]
	}
	sentence := strings.TrimSpace(last)
	promised := promisesAction.MatchString(sentence) && !strings.HasPrefix(strings.ToLower(sentence), "let me know")
	if promised || strings.HasSuffix(text, ":") {
		return true
	}
	if judge == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, judgeTimeout)
	defer cancel()
	verdict, err := judge.Ask(ctx, question.UnexpectedStop(request, text), question.YesNo)
	if err != nil {
		return false
	}
	return question.Stopped(verdict.JSON, verdict.Probability)
}

// clipText cuts s to max bytes on a rune boundary.
func clipText(s string, max int) (string, bool) {
	if len(s) <= max {
		return s, false
	}
	cut := max
	for cut > 0 && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut] + "…", true
}
