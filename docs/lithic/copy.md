# Copy

Lithic talks the way it looks: plain, flat, and certain. Words carry half of
every state (the mineral carries the other half), so the words are part of
the design system, not an afterthought.

Reference: `ui/pages/common/words.go`, `sentence.go`, `marks.go` in Ryoport.

## Rules

1. **Sentence case.** Titles, labels, buttons, group headers, tabs. Never
   all caps, never Title Case.
2. **Plain verbs on plaques.** Connect, Start, Stop, Create, Save, Sign in,
   Undo. One word when one word is enough. No "Click here", no "OK".
3. **The same word through the whole flow.** A plaque that says Start
   produces "Starting" and then "Running". Create leads to "Creating 62%"
   then "Running". Never Start on the button and "Booting" on the row.
4. **A claim always comes with its proof.** "Online" over "23 ms". "Needs
   you" over "sign-in failed 2 min ago". The claim is the state word in
   `label`; the proof is the fact in `caption`, with tabular figures.
5. **Errors say what happened and what to do.** "Could not reach
   mika-server: connection timed out. Check that it is on the network, then
   try again." Not "Error 110".
6. **Empty states offer the next step.** "No machines yet. Make one from the
   catalogue or import an ISO." with the plaques that do exactly that.
7. **Martian Mono only around literal data.** Names of hosts, addresses,
   ports, paths, versions. Counts, durations and percentages are numbers, not
   literals: they stay in the sans with tabular figures. A sentence may
   contain a mono span; a mono sentence is a mistake.
8. **Say who and what, not the system's internals.** "mika-server needs
   you", not "Resource 0x1f requires authentication".
9. **No exclamation marks, no emoji, no jokes in state.** Lithic is a control
   room.

## State words

The claim words, by mineral. Use these; add a new one only when the state is
genuinely new.

| Mineral | Claim | Proof shows |
|---|---|---|
| jade (alive) | Online, Running, Ready, Connected | latency, CPU, uptime |
| lapis (becoming) | Starting, Creating N%, Downloading, Importing, Installing | bytes left, time left, rate, stage |
| amber (needs you) | Needs you, Paused, Waiting | what it needs: "sign-in failed 2 min ago", "1 failed unit", "confirm the disk" |
| jasper (failed) | Unreachable, Failed, Stopped unexpectedly | the last error, when |
| idle | Stopped, Off, Not configured | "last run 2 d ago" |

A degraded resource keeps its alive claim and carries the problem as an
amber proof: "Running" over "1 failed unit".

## Sentences

The fleet sentence is built from spans so a literal part can be set in mono:

- nothing: "Nothing here yet."
- all alive: "Everything is up."
- otherwise: "Three running." then, when something needs attention,
  "mika-server needs you."

The sentence stays in one ink. Its state lives on the row, in the mineral
beside its word; a coloured phrase in a heading reads as decoration.

Numbers under ten are words in sentences ("Three running"); in proofs and
counts they are digits with tabular figures.

## Marks

Resource traits after the facts, set apart by space with no separator glyph:
Disposable (always first, in amber because it will be destroyed), Instant,
Sealed. When the line overflows, whole trailing marks drop before the facts
ellipsise. Facts read as words, not as a dotted list: "Sealed on ally",
"ally at 192.0.2.18".

## Times and sizes

Relative times in proofs: "2 min ago", "3 d", "just now". Absolute times in
Activity grouped by day. Sizes in binary units with one decimal: "1.2 GB",
"38 MB/s". Latency in ms. Percentages without a space: "62%".

## Destructive flows

A destructive dialog names the consequence in the body ("This deletes
windows-11 and its 48 GB disk. Snapshots go with it.") and may require typing
the name. The confirm plaque says the verb (Delete), never "Yes". After the
act, a placard offers Undo for five seconds where undo is possible, or states
the result where it is not ("windows-11 deleted").

