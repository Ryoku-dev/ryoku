// Package importer is the `ryoku import` front door. It carries no migration
// logic of its own: the engine lives in ryoku-hub, and this is the headless
// orchestrator that scans a dropped config, auto-resolves every keybind clash
// by a single policy (--keep), applies, and prints the result. The Hub is the
// interactive twin; both drive the same ryoku-hub verbs so the logic exists
// once.
package importer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	i18n "ryoku-i18n"
)

func usageText() string {
	return i18n.T("Usage: ryoku import <path> [flags]\n\n  <path>              a config folder, a dropped ~/.config, or a dotfiles tree\n  --url <git-url>     clone a dotfiles repo instead of reading a local path\n  --keep mine|ryoku   how to settle every keybind clash (default: mine)\n  --undo [<ts>]       roll back the last import, or the one stamped <ts>\n\nHeadless: scan, resolve all conflicts to one side, apply, print the change set.\nRyoku Settings > System > Backup & import is the interactive path for reviewing clash by clash.\n")
}

// hubBin is the engine binary. It ships beside ryoku, so a bare name resolves
// on PATH; exec.Command does the lookup.
const hubBin = "ryoku-hub"

// runHub is the seam over execing the engine. stdin, when non-nil, is fed on
// the child's standard input (apply reads its decisions there); stdout is
// captured and returned; stderr streams straight through so a git clone's
// progress and any engine error reach the user live. Tests replace this to
// exercise the orchestration without ryoku-hub present.
var runHub = func(stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command(hubBin, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ryoku-hub %s: %w", strings.Join(args, " "), err)
	}
	return out.Bytes(), nil
}

// scanResult mirrors the ScanResult contract, narrowed to what headless
// resolution reads: the resolved local source (handed straight back to apply)
// and, per app, its id and each conflict's stable norm key. Everything else in
// the payload is for the Hub's review UI.
type scanResult struct {
	Source string    `json:"source"`
	Apps   []scanApp `json:"apps"`
}

type scanApp struct {
	ID        string         `json:"id"`
	Conflicts []scanConflict `json:"conflicts"`
}

type scanConflict struct {
	Norm string `json:"norm"`
}

// decisions is the Decisions contract: which apps to include and how each
// conflict (by norm) is settled. The CLI only ever picks a whole side, so the
// conflict value is always a plain "mine"/"ryoku" string, never a remap object.
type decisions struct {
	Source    string               `json:"source"`
	Apps      map[string]appChoice `json:"apps"`
	Conflicts map[string]string    `json:"conflicts"`
}

type appChoice struct {
	Include bool `json:"include"`
}

type applyResult struct {
	TS            string   `json:"ts"`
	BackupDir     string   `json:"backupDir"`
	FilesWritten  []string `json:"filesWritten"`
	BindsIngested int      `json:"bindsIngested"`
	RulesIngested int      `json:"rulesIngested"`
	Unbinds       int      `json:"unbinds"`
}

type undoResult struct {
	TS       string   `json:"ts"`
	Restored []string `json:"restored"`
}

// buildDecisions turns a scan into the apply payload: import every detected app
// and settle every conflict to keep. It is pure so the resolution policy is
// testable without the engine. "ryoku" is the engine's default for any norm it
// is not told about, but the CLI still names every norm so the applied choice
// is explicit and auditable in the payload.
func buildDecisions(scan scanResult, keep string) decisions {
	d := decisions{
		Source:    scan.Source,
		Apps:      make(map[string]appChoice, len(scan.Apps)),
		Conflicts: map[string]string{},
	}
	for _, app := range scan.Apps {
		d.Apps[app.ID] = appChoice{Include: true}
		for _, c := range app.Conflicts {
			d.Conflicts[c.Norm] = keep
		}
	}
	return d
}

type options struct {
	source string
	keep   string
	undo   bool
	ts     string
	help   bool
}

// parseArgs reads the flag surface. --url and the positional path are two ways
// to name one source; --undo takes an optional timestamp, given as the trailing
// positional so the same field carries the import path and the undo stamp.
func parseArgs(args []string) (options, error) {
	opts := options{keep: "mine"}
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--url":
			i++
			if i >= len(args) {
				return opts, errors.New(i18n.T("--url needs a git URL"))
			}
			opts.source = args[i]
		case "--keep":
			i++
			if i >= len(args) {
				return opts, errors.New(i18n.T("--keep needs mine or ryoku"))
			}
			opts.keep = args[i]
		case "--undo":
			opts.undo = true
		case "-h", "--help", "help":
			opts.help = true
		default:
			if strings.HasPrefix(args[i], "-") {
				return opts, fmt.Errorf(i18n.T("unknown flag: %s"), args[i])
			}
			positional = append(positional, args[i])
		}
	}
	if opts.keep != "mine" && opts.keep != "ryoku" {
		return opts, fmt.Errorf(i18n.T("--keep must be mine or ryoku, got %q"), opts.keep)
	}
	if opts.undo {
		if len(positional) > 0 {
			opts.ts = positional[0]
		}
	} else if opts.source == "" && len(positional) > 0 {
		opts.source = positional[0]
	}
	return opts, nil
}

// Run is the `ryoku import` entry point.
func Run(args []string) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	if opts.help {
		fmt.Print(usageText())
		return nil
	}
	if opts.undo {
		return runUndo(opts.ts)
	}
	if opts.source == "" {
		fmt.Print(usageText())
		return errors.New(i18n.T("import needs a path or --url <git-url>"))
	}
	return runImport(opts.source, opts.keep)
}

func runImport(source, keep string) error {
	out, err := runHub(nil, "import", "scan", source)
	if err != nil {
		return err
	}
	var scan scanResult
	if err := json.Unmarshal(out, &scan); err != nil {
		return fmt.Errorf(i18n.T("parsing scan result: %w"), err)
	}

	payload, err := json.Marshal(buildDecisions(scan, keep))
	if err != nil {
		return err
	}
	out, err = runHub(payload, "import", "apply", "-")
	if err != nil {
		return err
	}
	var res applyResult
	if err := json.Unmarshal(out, &res); err != nil {
		return fmt.Errorf(i18n.T("parsing apply result: %w"), err)
	}
	printApply(res, len(scan.Apps))
	return nil
}

func runUndo(ts string) error {
	args := []string{"import", "undo"}
	if ts != "" {
		args = append(args, ts)
	}
	out, err := runHub(nil, args...)
	if err != nil {
		return err
	}
	var res undoResult
	if err := json.Unmarshal(out, &res); err != nil {
		return fmt.Errorf(i18n.T("parsing undo result: %w"), err)
	}
	if len(res.Restored) == 0 {
		fmt.Println(i18n.T("Nothing to undo."))
		return nil
	}
	fmt.Printf(i18n.T("Undid import %s, restored:\n"), res.TS)
	for _, f := range res.Restored {
		fmt.Printf("  %s\n", f)
	}
	return nil
}

func printApply(res applyResult, apps int) {
	fmt.Printf(i18n.T("Imported %s.\n"), count(apps, i18n.T("app")))
	fmt.Printf(i18n.T("  binds ingested: %d\n"), res.BindsIngested)
	fmt.Printf(i18n.T("  window rules ingested: %d\n"), res.RulesIngested)
	fmt.Printf(i18n.T("  unbinds added: %d\n"), res.Unbinds)
	fmt.Printf(i18n.T("  files written: %s\n"), strings.Join(res.FilesWritten, ", "))
	fmt.Printf(i18n.T("  backup: %s\n"), res.BackupDir)
	fmt.Printf(i18n.T("\nUndo with: ryoku import --undo %s\n"), res.TS)
}

func count(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
