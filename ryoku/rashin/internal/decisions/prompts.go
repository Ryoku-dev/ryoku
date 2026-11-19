package decisions

// Derived from oh-my-pi (MIT), packages/coding-agent/src/prompts/memories/sharpshooter-{extract,consolidate}-{system,input}.md@c7fff67
//
// Rewritten for small local models (omp's small-model rules): one task,
// few positive rules, examples in the runtime's own shape with the empty
// case first and the canonical shape last. What the prompts used to forbid
// is enforced in code: the quote must be found in the prompt, only friction
// is admitted, notes are cut to one line and to MaxLines per file, secrets
// are redacted.

const extractTool = "record_decisions"

const extractSystem = `You find lasting project decisions in one message a user sent to a coding assistant.
Call record_decisions once. Most messages hold no decision; then call it with "decisions": [].
Record a decision when the user corrects the assistant, says something that worked broke again, or states a rule a newcomer would get wrong.
Write "rule" as one timeless sentence about the project and copy "quote" word for word from <user-prompt>.
"area" is architecture (structure, components, storage), product (behaviour, defaults, naming, scope) or style (look, layout, wording).

<user-prompt>
fix the typo in the readme please
</user-prompt>
record_decisions {"decisions":[]}

<assistant-before>Should the export button go in the toolbar or in the menu?</assistant-before>
<user-prompt>
menu, like I said last time, the toolbar stays for editing only
</user-prompt>
record_decisions {"decisions":[{"area":"product","rule":"The export button lives in the menu; the toolbar holds editing actions only.","quote":"the toolbar stays for editing only","friction":"correction"}]}

<user-prompt>
the sidebar is grey again, it has to use the accent colour like we agreed
</user-prompt>
record_decisions {"decisions":[{"area":"style","rule":"The sidebar uses the accent colour.","quote":"it has to use the accent colour like we agreed","friction":"regression"}]}`

const extractSchema = `{"type":"object","properties":{"decisions":{"type":"array","items":{"type":"object","properties":{` +
	`"area":{"type":"string","enum":["architecture","product","style"]},` +
	`"rule":{"type":"string"},` +
	`"quote":{"type":"string"},` +
	`"friction":{"type":"string","enum":["correction","regression","subtle"]}},` +
	`"required":["area","rule","quote","friction"]}}},"required":["decisions"]}`

const consolidateTool = "update_notes"

const consolidateSystem = `You keep a project's decision notes up to date. <notes> lists the current notes, each with an id; <new> lists new decisions.
Call update_notes once with a list of changes. "add" needs area, section (a short topic) and text. "replace" needs the id and the merged text. "remove" needs the id of a note a new decision overturns.
Merge a new decision into the note on the same topic, and keep each text one short timeless rule.
Add only rules about the project; leave out the current task, file paths and code names, and what <docs> already says.
When nothing should change, call update_notes with "changes": [].

<notes>
A1 [architecture / Storage] Settings live in one file that only the daemon writes.
</notes>
<new>
- architecture: The daemon writes the settings file. (correction)
</new>
update_notes {"changes":[]}

<notes>
P1 [product / Defaults] New features start turned off.
</notes>
<new>
- product: Optional features stay off until the user turns them on. (correction)
- style: Buttons use square corners. (regression)
</new>
update_notes {"changes":[{"action":"replace","id":"P1","text":"Optional features stay off until the user turns them on (recurrently corrected)."},{"action":"add","area":"style","section":"Controls","text":"Buttons use square corners."}]}`

const consolidateSchema = `{"type":"object","properties":{"changes":{"type":"array","items":{"type":"object","properties":{` +
	`"action":{"type":"string","enum":["add","replace","remove"]},` +
	`"id":{"type":"string"},` +
	`"area":{"type":"string","enum":["architecture","product","style"]},` +
	`"section":{"type":"string"},` +
	`"text":{"type":"string"}},` +
	`"required":["action"]}}},"required":["changes"]}`

// blockIntro opens the system-prompt block.
const blockIntro = "Project decision memory. The user had to correct these before; follow them unless the user says otherwise."
