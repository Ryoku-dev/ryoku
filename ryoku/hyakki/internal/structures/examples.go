package structures

import "ryoku-rashin/internal/protocol"

func workedExample(t protocol.StructureTemplate) protocol.StructureExample {
	e := protocol.StructureExample{SeatCount: t.DefaultSeats}
	step := func(action string, seats ...string) {
		ids := make([]string, len(seats))
		for i, id := range seats {
			ids[i] = "draft-" + id
		}
		e.Steps = append(e.Steps, protocol.StructureExampleStep{Members: ids, Action: action})
	}
	switch t.ID {
	case "goetia-court":
		e.Title = "Ship the export-to-CSV feature"
		e.Goal = "Add CSV export to the reports page, with tests, and open a PR."
		step("Read the goal; split it into schema, endpoint, UI button, tests and PR. Delegate implementation through the dukes and hold the acceptance criteria.", "sovereign")
		step("Parse the existing report format and CSV specification; find hidden coupling in the serializer and report the required contract.", "executor-formats")
		step("Implement the endpoint and UI; surface that the reports API paginates and escalate the export-completeness decision rather than silently dropping rows.", "executor-build")
		step("Review naming and edge cases; restore the null-handling case dropped in the first diff.", "specialist")
		step("Run the tests, catch the regression in the existing XLSX path and block PR readiness until it is fixed; provide the failing case and retest the correction.", "verifier")
		step("Write the PR description and changelog; record changes, evidence, ownership and unresolved decisions.", "recorder")
		step("Track spend against the token budget and flag over-thinking by the implementation seat; budget changes still require the Summoner's approval.", "treasurer")
		e.Result = "A reviewed, tested CSV-export PR with the existing XLSX behavior preserved and a clear record of who did what and why."
		e.Guardrail = "Roles do not bypass the sandbox, PR/tool approval or budget ceilings. Verification evidence is required before claiming readiness."
	case "seven-princes":
		e.Title = "Should we build a mobile app?"
		e.Goal = "Should Ryoku ship a mobile companion app this year?"
		step("Develop the ambitious vision: what a mobile app would make Ryoku become and how success would be recognizable.", "lucifer")
		step("Cost engineering hours, store fees, support load and credit burn; expose assumptions and opportunity costs.", "mammon")
		step("Determine what users actually desire: a mobile application, or simply notifications? Separate demand from the proposed solution.", "asmodeus")
		step("Survey competitors' mobile applications and identify what they do better, with evidence rather than imitation for its own sake.", "leviathan")
		step("Gather all relevant signals: forum requests, analytics and support tickets; identify missing or biased evidence.", "beelzebub")
		step("Red-team failure modes, security surface and the maintenance trap; state the strongest case against shipping.", "satan")
		step("Offer the least-effort effective alternative, such as a PWA or responsive web view, and compare its tradeoffs with native.", "belphegor")
		step("Weigh all seven lenses into one recommendation with evidence, tradeoffs and the strongest dissent preserved.", "synthesizer")
		e.Result = "A decision examined from seven honest angles, not one model's guess; the Summoner receives a recommendation, not an unauthorized product commitment."
		e.Guardrail = "Each lens offers advice only. Neither a prince nor the synthesizer may authorize spending, widen tool permissions or commit the product; the Summoner decides within the reviewed policy and finite budget."
	case "elemental-ring":
		e.Title = "Keep my project safe and running while I'm away"
		e.Goal = "Watch my repo and my backups for the week; act only within these rules."
		e.SeatCount = 5
		step("Verify the nightly backup exists and is restorable; alert if a snapshot is missing or recovery fails.", "ice")
		step("Check synchronization between the working copy and remote; flag drift without overwriting either side.", "water")
		step("Carry actionable alerts to the Summoner and escalate only what matters under the notification policy.", "wind")
		step("Apply only a narrow, pre-approved fix such as restarting a stuck CI job; anything outside that authorization waits for approval.", "fire")
		step("Optionally watch disk capacity and infrastructure health as the foundation; this fifth Paracelsian guardian is not part of the faithful four-seat default.", "earth")
		e.Result = "Four domains guarded by peers, with optional earth/infrastructure coverage and no single bottleneck."
		e.Guardrail = "The ring watches and acts only within user-configured rules. A week-long goal does not create scheduling, permissions or budget grants; configure routines explicitly."
	case "night-parade":
		e.Title = "Triage my whole inbox and draft replies"
		e.Goal = "Triage this week's inbox: label, summarize, and draft replies to anything needing one."
		e.SeatCount = 9
		// The documented workload needs eight specialists, not the generic cycling
		// roster. This is a real filled draft using canonical identities.
		p := t
		p.Variable = false
		p.MinSeats, p.MaxSeats, p.DefaultSeats = 9, 9, 9
		p.Seats = []protocol.StructureSeat{t.Seats[0]}
		for _, spec := range []struct{ prototype, id string }{
			{"kitsune", "kitsune-1"}, {"kitsune", "kitsune-2"}, {"kitsune", "kitsune-3"},
			{"bakeneko", "bakeneko-1"}, {"bakeneko", "bakeneko-2"},
			{"hitodama", "hitodama"}, {"tengu", "tengu"}, {"tanuki", "tanuki"},
		} {
			for _, s := range t.Seats {
				if s.ID == spec.prototype {
					s.ID = spec.id
					p.Seats = append(p.Seats, s)
					break
				}
			}
		}
		members, err := Instantiate(p, 9)
		if err != nil {
			panic("invalid built-in Night Parade example: " + err.Error())
		}
		e.Members = members
		step("Split the inbox into batches, assign specialists, and track coverage so every thread has an accountable owner.", "marshal")
		step("Handle complex threads: gather context and plan appropriate replies, preserving uncertainty for review.", "kitsune-1", "kitsune-2", "kitsune-3")
		step("Fast-label simple newsletters and receipts; route ambiguous items back for careful review rather than guessing.", "bakeneko-1", "bakeneko-2")
		step("Recall prior correspondence with each sender to supply tone and context without inventing history.", "hitodama")
		step("Route urgent threads to the Summoner and guard against sending without approval.", "tengu")
		step("Rewrite the draft replies into the Summoner's voice, keeping factual content and unresolved questions intact.", "tanuki")
		e.Result = "A fully triaged inbox and reviewable drafts, produced by Oni plus eight specialists: nine total seats."
		e.Guardrail = "Nothing sends without approval: the parade drafts, the Summoner dispatches. All seat counts include Oni."
	case "nine-orders":
		e.Title = "Harden this release before we ship"
		e.Goal = "Find everything wrong with this release candidate."
		step("Investigate the diff and changelog and gather exactly what changed.", "inquisitors")
		step("Stress and fuzz new endpoints in the approved test environment, with bounded load and reproducible inputs.", "creators-of-tempests")
		step("Assert each suspected defect with concrete evidence and a reproducible failure, distinguishing suspicion from a confirmed bug.", "accusers")
		step("Check documentation and model-generated copy for hallucinations, misleading claims and falsehoods; trace factual claims to evidence.", "deluding", "lying-spirits")
		step("Challenge release premises: does the software actually do what the release notes claim?", "false-gods")
		step("Compare with the last release and hunt regressions, retaining known working behavior as the baseline.", "vengeful")
		step("Apply agreed corrections within grants and return them for re-verification; do not suppress failing checks.", "furies")
		step("Rewrite release notes to be both accurate and compelling, including limitations established by the other orders.", "tempters")
		e.Result = "A release examined by nine functional orders, with evidenced findings, agreed fixes and honest release notes."
		e.Guardrail = "Stress-testing is bounded by the sandbox and approvals; the taxonomy is not authority to attack external systems or ship the release."
	case "pagan-council":
		e.Title = "We're stuck — give us a way forward"
		e.Goal = "Our onboarding is losing users and the team is out of ideas. Advise."
		step("Map the crossroads: paths not taken, a different first-run experience or a different audience.", "hecate")
		step("Identify what must be cut: feature bloat and the step that kills momentum; distinguish necessary transformation from novelty.", "kali")
		step("Ask what is natural: what users want to do rather than what the onboarding forces them to do.", "pan")
		step("Propose the unconventional angle, such as gamifying or inverting the flow, and explain how it could be evaluated.", "loki")
		step("Refuse groupthink and challenge the assumption every other voice shares.", "lilith")
		step("Hold the tension between simple and powerful, fast and thorough; balance the advice without erasing disagreement.", "baphomet")
		e.Result = "Six strong, distinct voices break the deadlock and expose practical alternatives. The council advises; the Summoner decides."
		e.Guardrail = "Respect living and historical traditions: Kali is a revered Hindu goddess, not a demon. These are mythic advisory roles, never religious claims. Any figure may be renamed or removed in a clone."
	}
	if e.Members == nil {
		members, err := Instantiate(t, e.SeatCount)
		if err != nil {
			panic("invalid built-in structure example: " + err.Error())
		}
		e.Members = members
	}
	return e
}
