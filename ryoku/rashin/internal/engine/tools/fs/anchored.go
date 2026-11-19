package fs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
)

// Derived from oh-my-pi (MIT), crates/edit/src/modes/hashline/mod.rs@c7fff67

type plannedEdit struct {
	File                   editable
	Shown, Move, MoveShown string
	Original, Next         string
	Remove                 bool
	RemovePath             string
	RemoveVersion          string
	Diff                   *protocol.Diff
	First                  int
}

func anchoredEdit(ctx context.Context, call tool.Call, env tool.Env, input string) tool.Result {
	sections, err := parsePatch(input)
	if err != nil {
		return tool.Failed(err.Error())
	}
	changes := make([]plannedEdit, 0, len(sections))
	registers := map[string][]string{}
	var anonymous []string
	for _, section := range sections {
		abs := resolveFor(env, section.Path, env.Cwd)
		shown := display(abs, env.Cwd)
		file, err := readEditable(ctx, env, abs)
		if err != nil {
			return tool.Failed(describe(err, shown))
		}
		current := fileTag(file.Text.Text)
		if current != section.Tag {
			return tool.Failed(fmt.Sprintf("Edit rejected for %s: file changed between read and edit. Section is bound to #%s, but the current file hashes to #%s. Re-read it for the new [PATH#TAG] header.", shown, section.Tag, current))
		}
		if ok, other, collision := state(env.Session.ID).issuedFor(file.Path, section.Tag, file.Text.Text); !ok {
			if collision {
				return tool.Failed(fmt.Sprintf("Edit rejected for %s: the four-digit tag #%s identifies different contents in this session. Use compact find/replace or write to make this change safely.", shown, section.Tag))
			}
			if len(other) > 0 {
				return tool.Failed(fmt.Sprintf("Edit rejected for %s: #%s was read for another file. Read this file for its own [PATH#TAG].", shown, section.Tag))
			}
			return tool.Failed(fmt.Sprintf("Edit rejected for %s: hash #%s was not read in this session. Read %s before editing it.", shown, section.Tag, shown))
		}
		plan := plannedEdit{File: file, Shown: shown, Original: file.Text.Text}
		original := lines(file.Text.Text)
		spans := make([][]string, len(original)+1)
		deleted := make([]bool, len(original))
		used := make([]bool, len(original))
		gaps := make([]bool, len(spans))
		for _, op := range section.Ops {
			switch op.Verb {
			case "REM":
				if len(section.Ops) != 1 {
					return tool.Failed(fmt.Sprintf("REM in %s must stand alone under its file header", shown))
				}
				plan.RemovePath = file.Path
				plan.RemoveVersion = file.Info.Version
				if abs != file.Path {
					link, err := env.Exec.Stat(ctx, abs)
					if err != nil {
						return tool.Failed(describe(err, shown))
					}
					if link.Kind == exec.KindSymlink {
						plan.RemovePath = abs
						plan.RemoveVersion = link.Version
					}
				}
				plan.Remove = true
			case "MV":
				if plan.Move != "" {
					return tool.Failed(fmt.Sprintf("%s has two MV destinations; move it once", shown))
				}
				plan.MoveShown = resolveFor(env, op.Dest, env.Cwd)
				if err := secret(plan.MoveShown); err != nil {
					return tool.Failed(err.Error())
				}
				real, _, exists, err := target(ctx, env.Exec, plan.MoveShown)
				if err != nil {
					return tool.Failed(describe(err, plan.MoveShown))
				}
				if exists {
					return tool.Failed(fmt.Sprintf("MV destination %s already exists; pick a new destination", plan.MoveShown))
				}
				if real == file.Path {
					return tool.Failed(fmt.Sprintf("MV destination is the same file: %s", shown))
				}
				plan.Move = real
			case "CUT", "PUT":
				anchor, err := parseLocator(op.Locator, op.Verb == "PUT")
				if err != nil {
					return tool.Failed(fmt.Sprintf("%s line %d: %v", shown, op.Row, err))
				}
				if anchor.Gap == -1 {
					anchor.Gap = len(original)
				}
				body := op.Body
				if op.Verb == "PUT" && len(body) == 0 {
					if op.Register != "" {
						if r, ok := registers[op.Register[1:]]; ok {
							body = r
						} else if r, ok := state(env.Session.ID).register(op.Register[1:]); ok {
							body = r
						} else {
							return tool.Failed(fmt.Sprintf("%s line %d: register %s is empty. CUT into it first", shown, op.Row, op.Register))
						}
					} else if anonymous != nil {
						body = anonymous
					} else {
						return tool.Failed(fmt.Sprintf("%s line %d: nothing to paste; CUT a range first", shown, op.Row))
					}
				}
				if anchor.End > len(original) || (anchor.Start == 0 && anchor.End == 0 && anchor.Gap > len(original)) || anchor.Gap > len(original) {
					return tool.Failed(fmt.Sprintf("%s line %d: %s is outside the file (%d lines). Read it again", shown, op.Row, op.Locator, len(original)))
				}
				if anchor.End > 0 {
					for n := anchor.Start; n < anchor.End; n++ {
						if used[n] {
							return tool.Failed(fmt.Sprintf("%s line %d: %s overlaps a previous operation. Use disjoint original line ranges", shown, op.Row, op.Locator))
						}
					}
					if op.Verb == "CUT" {
						captured := append([]string(nil), original[anchor.Start:anchor.End]...)
						anonymous = captured
						if op.Register != "" {
							registers[op.Register[1:]] = captured
						}
					}
					for n := anchor.Start; n < anchor.End; n++ {
						used[n] = true
						deleted[n] = true
					}
					if op.Verb == "PUT" {
						spans[anchor.Start] = append(spans[anchor.Start], body...)
					}
					if plan.First == 0 || anchor.Start+1 < plan.First {
						plan.First = anchor.Start + 1
					}
				} else {
					if op.Verb == "CUT" {
						return tool.Failed(fmt.Sprintf("%s line %d: CUT needs an inclusive N.=M range", shown, op.Row))
					}
					if gaps[anchor.Gap] {
						return tool.Failed(fmt.Sprintf("%s line %d: the gap %s was already used; put the rows in one PUT", shown, op.Row, op.Locator))
					}
					gaps[anchor.Gap] = true
					spans[anchor.Gap] = append(spans[anchor.Gap], body...)
					if plan.First == 0 || anchor.Gap+1 < plan.First {
						plan.First = anchor.Gap + 1
					}
				}
			}
		}
		if !plan.Remove {
			var output []string
			for n, line := range original {
				output = append(output, spans[n]...)
				if !deleted[n] {
					output = append(output, line)
				}
			}
			output = append(output, spans[len(original)]...)
			plan.Next = strings.Join(output, "\n")
			if strings.HasSuffix(plan.Original, "\n") && len(output) > 0 {
				plan.Next += "\n"
			}
			if len(plan.Next) > exec.MaxRead {
				return tool.Failed(fmt.Sprintf("%s would exceed the %s edit limit; split the edit", shown, size(exec.MaxRead)))
			}
			if plan.Next == plan.Original && plan.Move == "" {
				return tool.Failed(fmt.Sprintf("The patch leaves %s unchanged. Read the file and change only lines that need changing", shown))
			}
		}
		plan.Diff = makeDiff(file.Path, plan.Original, plan.Next)
		changes = append(changes, plan)
	}
	for _, p := range changes {
		input := p.File.Path
		if p.Move != "" {
			input += " -> " + targetName(p.MoveShown, p.Move, env.Cwd)
		}
		if r, ok := allow(ctx, env, call, i18n.Tf("Editing %s", filepath.Base(p.File.Path)), input, p.Diff); !ok {
			return r
		}
	}
	for _, p := range changes {
		if _, err := backup(env, p.File.Path, p.File.Text.Raw); err != nil {
			return tool.Failed(i18n.Tf("Could not keep a copy before changing %s: %v", p.Shown, err))
		}
	}
	var summaries []string
	var diffs []*protocol.Diff
	var places []protocol.Location
	for _, p := range changes {
		if p.Remove {
			err = env.Exec.Remove(ctx, exec.RemoveRequest{Path: p.RemovePath, IfVersion: p.RemoveVersion})
		} else if p.Move != "" {
			_, err = env.Exec.Write(ctx, exec.WriteRequest{Path: p.Move, Content: p.File.Text.join(p.Next), IfAbsent: true, MakeDirs: true, Mode: p.File.Info.Mode})
			if err == nil {
				err = env.Exec.Remove(ctx, exec.RemoveRequest{Path: p.File.Path, IfVersion: p.File.Info.Version})
				if err != nil {
					return tool.Failed(fmt.Sprintf("Copied %s to %s but could not remove the source: %v. Both paths now exist; resolve them manually", p.Shown, display(p.Move, env.Cwd), err))
				}
			}
		} else {
			_, err = env.Exec.Write(ctx, exec.WriteRequest{Path: p.File.Path, Content: p.File.Text.join(p.Next), IfVersion: p.File.Info.Version})
		}
		if err != nil {
			return tool.Failed(fmt.Sprintf("Could not edit %s: %s. Earlier patch sections may have landed; read them before retrying", p.Shown, describe(err, p.Shown)))
		}
		if p.Remove {
			summaries = append(summaries, "Removed "+p.Shown)
		} else {
			final := p.File.Path
			if p.Move != "" {
				final = p.Move
				summaries = append(summaries, "Moved "+p.Shown+" to "+targetName(p.MoveShown, final, env.Cwd))
			} else {
				summaries = append(summaries, "Edited "+p.Shown)
			}
			tag := fileTag(p.Next)
			state(env.Session.ID).issue(final, tag, p.Next)
			summaries = append(summaries, header(display(final, env.Cwd), tag))
			places = append(places, protocol.Location{Path: final, Line: p.First})
		}
		diffs = append(diffs, p.Diff)
	}
	state(env.Session.ID).keep(registers)
	result := tool.Result{Text: strings.Join(summaries, "\n"), Shown: i18n.T("Edited files."), Locations: places}
	if len(diffs) == 1 {
		result.Diff = diffs[0]
	} else if len(diffs) > 1 {
		result.Diff = combinedDiff(diffs)
	}
	return result
}

func combinedDiff(diffs []*protocol.Diff) *protocol.Diff {
	var before, after strings.Builder
	truncated := false
	for _, d := range diffs {
		fmt.Fprintf(&before, "[%s]\n%s\n", d.Path, d.Old)
		fmt.Fprintf(&after, "[%s]\n%s\n", d.Path, d.New)
		truncated = truncated || d.Truncated
	}
	old, cutOld := tool.ClipRunes(before.String(), diffRunes)
	next, cutNew := tool.ClipRunes(after.String(), diffRunes)
	return &protocol.Diff{Path: "(multiple files)", Old: old, New: next, Truncated: truncated || cutOld || cutNew}
}
