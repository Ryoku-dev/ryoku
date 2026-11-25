package demo

import (
	"encoding/json"
	"path/filepath"

	"ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/keep"
	"ryoku-rashin/internal/protocol"
)

// FeatureOniGuide is no feature: with it the demo starts with Oni on and
// perched, guides and Keep on, and three walkthroughs already kept.
const FeatureOniGuide = "oni-guide"

// demoNotesApp is the app of the notes window the demo perches on, as a
// keep names it
const demoNotesApp = "org.example.Notes"

// KeepsPath is where a demo serving on socket keeps walkthroughs: beside
// the socket, in the demo's own directory, never the user's state. The
// app's demo reads its keeps list from there.
func KeepsPath(socket string) string { return filepath.Join(filepath.Dir(socket), "oni-keeps.json") }

// demoKeepStep is a kept step: its sentence, the control's label and role,
// and where the control sat, in fractions of the window
func demoKeepStep(text, label, role string, box protocol.OniRect) keep.Step {
	return keep.Step{Text: text, Label: label, Role: role, Box: feature.Box{X: box.X, Y: box.Y, W: box.W, H: box.H}, Mode: feature.StrandModeLasso, Trusted: true}
}

// demoWalkthrough is the demo's one walkthrough for goal, as Keep writes it
func demoWalkthrough(goal string) keep.Entry {
	steps := make([]keep.Step, 0, demoGuideSteps)
	labels := [...][2]string{{"File", "menu"}, {"Save", "menu item"}}
	for n := 1; n <= demoGuideSteps; n++ {
		text, box := demoGuideStep(n)
		steps = append(steps, demoKeepStep(text, labels[n-1][0], labels[n-1][1], box))
	}
	return keep.Entry{App: demoNotesApp, Goal: goal, Steps: steps}
}

// demoKept are the walkthroughs the guide demo starts with: two in the
// notes app and one in the terminal
func demoKept() []keep.Entry {
	return []keep.Entry{
		{App: "org.example.Terminal", Goal: "clear the scrollback", Steps: []keep.Step{
			demoKeepStep(i18n.T("Open the Edit menu."), "Edit", "menu", protocol.OniRect{X: .08, Y: .01, W: .06, H: .04}),
			demoKeepStep(i18n.T("Click Clear Scrollback."), "Clear Scrollback", "menu item", protocol.OniRect{X: .08, Y: .2, W: .24, H: .045}),
		}},
		{App: demoNotesApp, Goal: "turn on spell checking", Steps: []keep.Step{
			demoKeepStep(i18n.T("Open the Tools menu."), "Tools", "menu", protocol.OniRect{X: .2, Y: .04, W: .07, H: .04}),
			demoKeepStep(i18n.T("Click Spelling."), "Spelling", "menu item", protocol.OniRect{X: .2, Y: .11, W: .14, H: .045}),
			demoKeepStep(i18n.T("Tick Check as you type."), "Check as you type", "check box", protocol.OniRect{X: .22, Y: .18, W: .2, H: .045}),
		}},
		{App: demoNotesApp, Goal: "print the packing list", Steps: []keep.Step{
			demoKeepStep(i18n.T("Open the File menu."), "File", "menu", protocol.OniRect{X: .02, Y: .04, W: .06, H: .04}),
			demoKeepStep(i18n.T("Click Print."), "Print", "menu item", protocol.OniRect{X: .02, Y: .25, W: .14, H: .045}),
		}},
	}
}

// useStore keeps walkthroughs in store, first writing the demo's own when
// seed is set
func (g *guideFixtures) useStore(store *keep.Store, seed bool) error {
	g.store = store
	if !seed {
		return nil
	}
	for _, e := range demoKept() {
		if err := store.Put(e); err != nil {
			return err
		}
	}
	return nil
}

// isKept says whether goal was kept: in the store when the demo has one,
// so forgetting it there is heard, else among the goals kept this run
func (g *guideFixtures) isKept(goal string) bool {
	if g.store != nil {
		_, _, ok := g.store.Match(demoNotesApp, goal)
		return ok
	}
	return g.kept[demoGoalKey(goal)]
}

// remember keeps goal's walkthrough and says how many are kept
func (g *guideFixtures) remember(goal string) (int, error) {
	g.kept[demoGoalKey(goal)] = true
	if g.store == nil {
		return len(g.kept), nil
	}
	if err := g.store.Put(demoWalkthrough(goal)); err != nil {
		return 0, err
	}
	entries, err := g.store.List()
	return len(entries), err
}

// show sends frame to whoever watches, numbered after the last; idle ends
// what the snapshot carries
func (g *guideFixtures) show(frame feature.GuideFrame) {
	g.frameSeq++
	frame.Seq = g.frameSeq
	g.frame = nil
	if frame.Phase != feature.GuideIdle {
		current := frame
		g.frame = &current
	}
	g.told = append(g.told, frame)
}

// idle ends the guide's frames and drops a keep offer still waiting
func (g *guideFixtures) idle() {
	if g.offer != nil {
		g.answerOffer(g.offer.ID, false)
	}
	if g.frame != nil {
		g.show(feature.GuideFrame{Phase: feature.GuideIdle})
	}
}

// stepFrame is the guide at step, pointing with strand target: the step,
// the checklist around it and what the card offers
func stepFrame(step int, replay bool, strand uint64) feature.GuideFrame {
	text, _ := demoGuideStep(step)
	kicker := i18n.Tf("Step %d", step)
	if replay {
		kicker = i18n.T("Kept from before")
	}
	plan := &feature.GuidePlan{Current: text}
	for n := 1; n <= demoGuideSteps; n++ {
		line, _ := demoGuideStep(n)
		switch {
		case n < step:
			plan.Done = append(plan.Done, line)
		case n > step:
			plan.Next = append(plan.Next, line)
		}
	}
	return feature.GuideFrame{Phase: feature.GuideWaiting, Step: step, Kicker: kicker, Text: text, Plan: plan,
		Actions: []string{feature.GuideActionNext, feature.GuideActionStop, feature.GuideActionCircle}, Replay: replay, Target: strand}
}

// readingFrame is the guide waiting on step's picture
func readingFrame(step int) feature.GuideFrame {
	return feature.GuideFrame{Phase: feature.GuideReading, Step: step, Kicker: i18n.Tf("Step %d", step), Actions: []string{feature.GuideActionStop}}
}

// offerKeep shows the finished guide offering to be kept, and the offer
func (g *guideFixtures) offerKeep(step int, id, question string) {
	g.show(feature.GuideFrame{Phase: feature.GuideKeeping, Step: step, Actions: []string{feature.GuideActionKeep, feature.GuideActionSkip}})
	g.offer = feature.KeepOffer(id, question)
	g.told = append(g.told, *g.offer)
}

// answerOffer says the keep offer id was answered, kept or not
func (g *guideFixtures) answerOffer(id string, yes bool) {
	answer := "no"
	if yes {
		answer = "yes"
	}
	if g.offer != nil && g.offer.ID == id {
		g.offer = nil
	}
	g.told = append(g.told, feature.ConsentAnswerFrame{ID: id, Answer: answer})
}

// opening are the frames a watcher starts from: the guide's and the keep
// offer waiting, while there are any
func (g *guideFixtures) opening() []json.RawMessage {
	frames := []json.RawMessage{}
	for _, p := range []feature.Payload{g.framePayload(), g.offerPayload()} {
		if p == nil {
			continue
		}
		if raw, err := feature.EncodeFrame(p); err == nil {
			frames = append(frames, raw)
		}
	}
	return frames
}

func (g *guideFixtures) framePayload() feature.Payload {
	if g.frame == nil {
		return nil
	}
	return *g.frame
}

func (g *guideFixtures) offerPayload() feature.Payload {
	if g.offer == nil {
		return nil
	}
	return *g.offer
}

// takeFrames are the frames shown since the last take, oldest first
func (g *guideFixtures) takeFrames() []feature.Payload {
	told := g.told
	g.told = nil
	return told
}

// watchOni makes the connection that opened the oni event hear its frames,
// and one that closed it stop
func (s *server) watchOni(out *outbox, method string) {
	switch method {
	case protocol.MOniOpen:
		s.mu.Lock()
		if s.oniWatchers == nil {
			s.oniWatchers = map[*outbox]bool{}
		}
		s.oniWatchers[out] = true
		s.mu.Unlock()
	case protocol.MOniClose:
		s.unwatchOni(out)
	}
}

func (s *server) unwatchOni(out *outbox) {
	s.mu.Lock()
	delete(s.oniWatchers, out)
	s.mu.Unlock()
}

// tellOni sends the frames the guide and Hands showed to every connection
// that opened the oni event, as the daemon's oni event does
func (s *server) tellOni() {
	s.mu.Lock()
	told := append(s.data.oni.guides.takeFrames(), s.data.oni.hands.takeFrames()...)
	watchers := make([]*outbox, 0, len(s.oniWatchers))
	for out := range s.oniWatchers {
		watchers = append(watchers, out)
	}
	s.mu.Unlock()
	if len(told) == 0 || len(watchers) == 0 {
		return
	}
	events := make([]protocol.Event, 0, len(told))
	for _, frame := range told {
		if raw, err := feature.EncodeFrame(frame); err == nil {
			events = append(events, protocol.Event{Event: protocol.EvtOni, Data: raw})
		}
	}
	for _, out := range watchers {
		for _, event := range events {
			if out.send(event) != nil {
				break
			}
		}
	}
}
