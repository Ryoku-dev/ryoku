package shell

import (
	"context"
	"image"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gioui.org/io/key"

	"ryoku-rashin/internal/app/demo"
	"ryoku-rashin/internal/protocol"
)

// newReflexStage navigates to Quick commands, rather than rendering every
// Oni control in an oversized stage.
func newReflexStage(t *testing.T, session string) *stage {
	t.Helper()
	stage := newStage(t, image.Pt(1440, 2200), Options{Route: protocol.TabSettings, SettingsSection: "oni-commands", Session: session})
	stage.open("settings/oni-commands")
	stage.await("the quick command switches are drawn", func() bool { return stage.mentions("Let a decision model sort commands") })
	return stage
}

// reminders and routines as the demo daemon now keeps them
func (s *stage) reflexReminders() []protocol.OniReminderInfo {
	s.t.Helper()
	var result protocol.OniRemindersResult
	if err := s.runtime.client.Call(context.Background(), protocol.MOniReminders, protocol.OniRemindersParams{Action: protocol.OniRemindersList}, &result); err != nil {
		s.t.Fatal(err)
	}
	return result.Reminders
}

func (s *stage) reflexRoutines() []protocol.OniRoutineInfo {
	s.t.Helper()
	var result protocol.OniRoutinesResult
	if err := s.runtime.client.Call(context.Background(), protocol.MOniRoutines, protocol.OniRoutinesParams{Action: protocol.OniRoutinesList}, &result); err != nil {
		s.t.Fatal(err)
	}
	return result.Routines
}

// Each switch writes its own setting and says what it needs while that is
// off: quick commands need talking to Oni, since they are only spoken, and
// each of the four needs quick commands. Turning a list's switch on reads
// its list.
func TestOniReflexSwitchesPatchSettingsWithTheirDependencies(t *testing.T) {
	stage := newReflexStage(t, demo.FeatureOni)
	if oni := stage.daemonOni(); oni.Reflex || oni.ReflexSettings != (protocol.OniReflexSettings{}) || oni.Voice {
		t.Fatalf("quick commands start on: %+v", oni)
	}
	needsVoice, needsQuick := "Needs Talk to Oni: quick commands are only ever spoken", "Needs Quick commands; until it is on, this does nothing."
	if !stage.mentions(needsVoice) || !stage.mentions(needsQuick) || !stage.mentions("Turn on Quick commands and Reminders and timers") {
		t.Fatalf("the off switches do not say what they need; on screen: %v", stage.names())
	}
	if stage.mentions("Memory is off, so Oni says it cannot keep a note") {
		t.Fatal("notes say memory is off while the demo's memory is on")
	}
	stage.setMemory(map[string]any{"use": false})
	stage.await("notes say memory is off", func() bool { return stage.mentions("Memory is off, so Oni says it cannot keep a note") })

	stage.toggle("Quick commands")
	stage.await("quick commands turn on", func() bool { return stage.daemonOni().Reflex })
	stage.await("the four stop asking for quick commands", func() bool { return !stage.mentions(needsQuick) })
	if !stage.mentions(needsVoice) {
		t.Fatal("with talking off, quick commands no longer say they need it")
	}
	stage.open("settings/oni-voice")
	stage.toggle("Talk to Oni")
	stage.await("talking turns on", func() bool { return stage.daemonOni().Voice })
	stage.open("settings/oni-commands")
	stage.await("quick commands stop asking for talking", func() bool { return !stage.mentions(needsVoice) })

	stage.toggle("Reminders and timers")
	stage.await("reminders turn on", func() bool { return stage.daemonOni().ReflexSettings.Reminders })
	stage.await("the reminders are read", func() bool { return stage.shows("call the dentist") })
	stage.toggle("Routines")
	stage.await("routines turn on", func() bool { return stage.daemonOni().ReflexSettings.Routines })
	stage.await("the routines are read", func() bool { return stage.shows("Wind down") })
	stage.toggle("Notes")
	stage.await("notes turn on", func() bool { return stage.daemonOni().ReflexSettings.Notes })
	if oni := stage.daemonOni(); oni.ReflexSettings.Decisions || !oni.Inspect || oni.Guide.Enabled || oni.VoiceSettings.OpenMic {
		t.Fatalf("the switches reached settings they do not own: %+v", oni)
	}

	stage.toggle("Reminders and timers")
	stage.await("reminders turn off", func() bool { return !stage.daemonOni().ReflexSettings.Reminders })
	stage.await("the reminders are no longer listed", func() bool { return !stage.shows("call the dentist") })
	stage.toggle("Quick commands")
	stage.await("quick commands turn off", func() bool { return !stage.daemonOni().Reflex })
	stage.await("the routines are no longer listed", func() bool { return !stage.shows("Wind down") && stage.mentions(needsQuick) })
}

// Letting a decision model sort commands opens its question first, which
// says what is sent and where without naming any outside model; Cancel or
// Escape leaves it off, only the yes turns it on, and turning it off never
// asks.
func TestOniReflexDecisionsNeedTheirDisclosure(t *testing.T) {
	stage := newReflexStage(t, demo.FeatureOni)
	name, title := "Let a decision model sort commands", "Let a decision model sort commands?"
	before := stage.names()
	stage.toggle(name)
	stage.await("the question opens", func() bool { return stage.mentions(title) })
	if stage.daemonOni().ReflexSettings.Decisions {
		t.Fatal("decisions turned on before the question was answered")
	}
	for _, want := range []string{"Only the words of that one command and the names of the commands are sent", "Built-in rules", "Ryoku Ember", "never grants a permission"} {
		if !stage.mentions(want) {
			t.Fatalf("the question does not say %q; there is: %v", want, stage.names())
		}
	}
	// a copy audit of what the question added to the screen and of the
	// switch's own words: no outside product or model is named
	shown := slices.DeleteFunc(stage.names(), func(n string) bool { return slices.Contains(before, n) })
	for _, n := range stage.names() {
		if strings.Contains(n, "the Fast decisions route under Providers") {
			shown = append(shown, n)
		}
	}
	outside := regexp.MustCompile(`(?i)\b(jev|typesafe|ollaya|openai|anthropic|claude|gpt|gemini|llama|qwen|mistral|deepseek|ollama|phi|grok)\b`)
	for _, n := range shown {
		if outside.MatchString(n) {
			t.Fatalf("the decisions copy names an outside model: %q", n)
		}
	}
	stage.click("Cancel")
	stage.await("the question closes", func() bool { return !stage.mentions(title) })
	stage.toggle(name)
	stage.await("the question opens again", func() bool { return stage.mentions(title) })
	stage.press(key.NameEscape, 0)
	stage.await("escape closes the question", func() bool { return !stage.mentions(title) })
	if stage.daemonOni().ReflexSettings.Decisions {
		t.Fatal("cancelling or escaping turned decisions on")
	}
	stage.toggle(name)
	stage.await("the question opens a third time", func() bool { return stage.mentions(title) })
	stage.click("Let it sort commands")
	stage.await("decisions turn on", func() bool { return stage.daemonOni().ReflexSettings.Decisions })
	stage.toggle(name)
	stage.await("decisions turn off without a question", func() bool { return !stage.daemonOni().ReflexSettings.Decisions })
	if stage.mentions(title) {
		t.Fatal("turning decisions off asked first")
	}
}

// Reminders list soonest first, a reminder by its own words and a timer by
// when it is due; Cancel drops one and Cancel all every one.
func TestOniReflexRemindersListAndCancel(t *testing.T) {
	stage := newReflexStage(t, demo.FeatureOniReflex)
	stage.await("the reminders are read", func() bool { return stage.shows("call the dentist") })
	if !stage.shows("Timer") || !stage.showsPrefix("Due ") || !stage.showsPrefix("Reminder · due ") {
		t.Fatalf("a reminder does not say what it is and when; on screen: %v", stage.names())
	}
	if stage.mentions("Oni stays running while a reminder is set") {
		t.Fatal("a machine that can wake Oni is told it stays running")
	}
	stage.click("Cancel “call the dentist”")
	stage.await("the reminder is cancelled", func() bool { return !stage.shows("call the dentist") })
	if left := stage.reflexReminders(); len(left) != 1 || left[0].Kind != protocol.OniReminderKindTimer {
		t.Fatalf("cancelling one left %+v", left)
	}
	stage.click("Cancel all")
	stage.await("every reminder is cancelled", func() bool { return stage.shows("Nothing set") })
	if left := stage.reflexReminders(); len(left) != 0 {
		t.Fatalf("cancelling all left %+v", left)
	}
}

// Where the wake unit cannot be armed, Settings says Oni stays running
// while a reminder is set.
func TestOniReflexSaysOniStaysRunningWithoutAWakeUnit(t *testing.T) {
	stage := newReflexStage(t, demo.FeatureOniReflexNoWake)
	stage.await("the residency notice is drawn", func() bool { return stage.shows("Oni stays running while a reminder is set") })
	if !stage.mentions("cannot start Oni on its own when a reminder comes due") {
		t.Fatalf("the notice does not say why; on screen: %v", stage.names())
	}
}

// Routines list by name and length, most recently used first; Run replays
// one, Forget drops one and Forget all every one.
func TestOniReflexRoutinesRunAndForget(t *testing.T) {
	stage := newReflexStage(t, demo.FeatureOniReflex)
	stage.settingsReveal("Wind down")
	stage.await("the routines are read", func() bool { return stage.shows("Wind down") })
	if !stage.shows("Morning") || !stage.showsPrefix("3 steps · saved ") || !stage.showsPrefix("2 steps · saved ") {
		t.Fatalf("a routine does not say its length; on screen: %v", stage.names())
	}
	stage.click("Run “Wind down”")
	stage.await("the routine ran", func() bool {
		r := stage.reflexRoutines()
		return len(r) == 2 && r[0].Name == "Wind down"
	})
	stage.click("Forget “Morning”")
	stage.await("Morning is forgotten", func() bool { return !stage.shows("Morning") })
	if left := stage.reflexRoutines(); len(left) != 1 || left[0].Name != "Wind down" {
		t.Fatalf("forgetting one left %+v", left)
	}
	stage.click("Forget all")
	stage.await("every routine is forgotten", func() bool { return stage.shows("No routine kept") })
	if left := stage.reflexRoutines(); len(left) != 0 {
		t.Fatalf("forgetting all left %+v", left)
	}
}

// The log names each command, its route, timing, confidence and outcome,
// newest first, and never the words said.
func TestOniReflexLogShowsCommandsNotWords(t *testing.T) {
	stage := newReflexStage(t, demo.FeatureOniReflex)
	stage.settingsReveal("Turn the volume up")
	stage.await("the log is read", func() bool { return stage.shows("Turn the volume up") })
	for _, want := range []string{"Open an app", "Minimize a window", "Turn the volume up"} {
		if !stage.shows(want) {
			t.Fatalf("%q is not in the log; on screen: %v", want, stage.names())
		}
	}
	for _, want := range []string{
		" · Built-in rules · 2 ms · 97% sure · Held by the launch pause",
		" · Built-in rules · 1 ms · 95% sure · Not possible on this desktop",
		" · Local decision model · 140 ms · 82% sure · Done",
	} {
		if !stage.mentions(want) {
			t.Fatalf("no row says %q; on screen: %v", want, stage.names())
		}
	}
	// every row of the log is a command's label over its facts; the
	// demo's routines and reminders hold words, which never appear there
	row := regexp.MustCompile(`^[A-Z][a-z]{2} \d\d:\d\d:\d\d · (Built-in rules|Local decision model|Ryoku Ember) · \d+ ms · \d+% sure · `)
	rows := 0
	for _, n := range stage.names() {
		if row.MatchString(n) {
			rows++
		}
	}
	if rows != 4 {
		t.Fatalf("the log shows %d rows, not 4: %v", rows, stage.names())
	}
	stage.click("Read the log again")
	stage.await("the log is read again", func() bool { return stage.shows("Turn the volume up") })
}
