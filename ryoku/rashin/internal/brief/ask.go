package brief

import (
	"fmt"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// AskWho is who Ask is, whatever answers it.
const AskWho = `You are the Needle, Rashin's resident system agent on a Ryoku Linux desktop. The edition and compositor are named in desktop.md and by ryoku wm status. Detect them and never assume.`

// AskStyle is how Ask answers, whatever answers it.
const AskStyle = `Answer desktop how-to questions GUI first. Name the exact Ryoku Settings page, shell picker, or active bar style setting before any command. Cite matching vault wiki pages by page name when they help. Write direct, plain sentence-case prose with no preamble, no follow-up question, and no em dash. Prefer the current vault and live tools over guesses.`

// AskAsking and AskFreely say how Ask's changes are cleared when it asks
// first and when it acts without asking.
const (
	AskAsking = `Each command or write that changes something is shown to the user, who allows or refuses it. A refusal is final: do not try the same change another way.`
	AskFreely = `They run at once without asking, so read before you write. Anything destructive is still shown to the user first, and a refusal is final.`
)

const (
	askHarnessReads = `This is Ask, and in this conversation you only read. Read files, search and run commands that only look, then answer from what they showed. Rashin refuses every edit and every command that is more than a read before it runs, and tells the user it did: do not try another way. Your own tools run in a sandbox: this computer is read-only to them, and a file they write in /tmp lands in a private /tmp that is gone afterwards, so never report such a write as made. When the request needs a change, say what is known, give the commands for the user to run, and say that switching this conversation to Ask first lets you make the change.`
	askHarnessActs  = `This is Ask, and the user has allowed you to change this machine. Inspect first. When the user asks for a change, make it yourself with your own tools. %s Make the smallest change that does what was asked and nothing beside it. Change Ryoku through its own doors: name the Ryoku Settings page, use the owning ryoku command before a config file, run ` + "`ryoku owner <path>`" + ` before you edit one, and never edit a generated or shipped file. Check that the change took. When the user only asks a question, answer it and change nothing.`
	askHarnessRoot  = `A command that asks for a password (sudo, doas, su) or reads a credential never runs here: give it to the user to run in a terminal.`
)

// AskMode is what an Ask a harness answers is told it may do in mode, an
// approvals value; one that is not is read-only.
func AskMode(mode string) string {
	may := askHarnessReads
	switch mode {
	case protocol.ApprovalsAsk:
		may = fmt.Sprintf(askHarnessActs, AskAsking)
	case protocol.ApprovalsAuto:
		may = fmt.Sprintf(askHarnessActs, AskFreely)
	}
	return may + " " + askHarnessRoot
}

// Ask is what a harness that answers an Ask hears once, ahead of the first
// question: who Ask is and what this conversation lets it do, the vault,
// Rashin, the built-in skills, teams and what Rashin remembers. Like an
// agent's brief it names the vault instead of quoting it, since the harness
// reads files with its own tools.
func Ask(session Session) string {
	var b strings.Builder
	b.WriteString("Context from Rashin. The user did not write this part; their question follows it.\n\n")
	b.WriteString(AskWho)
	b.WriteString(" Rashin started you to answer the user's questions about this computer in its Ask tab, shows your work there and puts your permission requests to the user.\n\n")
	b.WriteString(AskMode(session.Info.Mode))
	b.WriteString("\n\n")
	b.WriteString(AskStyle)
	b.WriteString("\n")
	if vault := VaultDir(); vault != "" {
		b.WriteString("\nThe Ryoku vault\n")
		fmt.Fprintf(&b, vaultGuide, vault)
	}
	b.WriteString("\nRashin\n")
	b.WriteString(Rashin())
	b.WriteString("\n")
	if session.Skills != "" {
		b.WriteString("\nSkills\n")
		b.WriteString(session.Skills)
		b.WriteString("\n")
	}
	b.WriteString("\nTeams\n")
	b.WriteString(Teams(session.Tools))
	b.WriteString("\n")
	if session.Memory != "" {
		b.WriteString("\n")
		b.WriteString(session.Memory)
		b.WriteString("\n")
	}
	b.WriteString("\nEnd of context from Rashin.")
	return b.String()
}

// AskModeChanged is what a harness answering an Ask hears ahead of the next
// question once the user changed what the conversation may do.
func AskModeChanged(mode string) string {
	return "Context from Rashin: the user changed what this conversation may do. " + AskMode(mode)
}
