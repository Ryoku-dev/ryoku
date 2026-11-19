package fs

import (
	"fmt"
	"strconv"
	"strings"
)

// Derived from oh-my-pi (MIT), crates/edit/grammars/hashline.lark@c7fff67

type patchSection struct {
	Path, Tag string
	Ops       []patchOp
}

type patchOp struct {
	Verb, Locator, Register, Dest string
	Body                          []string
	Row                           int
}

// parsePatch rejects malformed input before any file is touched.
func parsePatch(input string) ([]patchSection, error) {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	rows := strings.Split(strings.TrimSuffix(input, "\n"), "\n")
	if len(rows) == 0 || rows[0] == "" {
		return nil, fmt.Errorf("patch is empty. Read a file and send its [PATH#TAG] header with PUT/CUT/REM/MV operations")
	}
	if rows[0] == "*** Begin Patch" {
		rows = rows[1:]
		if len(rows) == 0 || rows[len(rows)-1] != "*** End Patch" {
			return nil, fmt.Errorf("patch needs *** End Patch as its last line")
		}
		rows = rows[:len(rows)-1]
	}
	var sections []patchSection
	seen := make(map[string]bool)
	for n := 0; n < len(rows); {
		p, tag, ok := parseHeader(rows[n])
		if !ok {
			return nil, fmt.Errorf("patch line %d: expected a [PATH#TAG] header copied from read", n+1)
		}
		if tag == "" {
			return nil, fmt.Errorf("patch line %d: %s needs a current four-digit hashline tag. Read the file again", n+1, p)
		}
		if seen[p] {
			return nil, fmt.Errorf("patch line %d: %s appears twice. Put all its operations under one header", n+1, p)
		}
		seen[p] = true
		section := patchSection{Path: p, Tag: tag}
		n++
		for n < len(rows) {
			if _, _, header := parseHeader(rows[n]); header {
				break
			}
			row := rows[n]
			op := patchOp{Row: n + 1}
			switch {
			case row == "REM":
				op.Verb = "REM"
			case strings.HasPrefix(row, "MV "):
				op.Verb = "MV"
				op.Dest = strings.Trim(strings.TrimSpace(strings.TrimPrefix(row, "MV ")), "\"")
				if op.Dest == "" {
					return nil, fmt.Errorf("patch line %d: MV needs a destination path", n+1)
				}
			case strings.HasPrefix(row, "CUT "):
				op.Verb = "CUT"
				fields := strings.Fields(strings.TrimPrefix(row, "CUT "))
				if len(fields) < 1 || len(fields) > 2 {
					return nil, fmt.Errorf("patch line %d: CUT takes an inclusive N.=M range and optional @register", n+1)
				}
				op.Locator = fields[0]
				if len(fields) == 2 {
					op.Register = fields[1]
				}
			case strings.HasPrefix(row, "PUT "):
				op.Verb = "PUT"
				where := strings.TrimPrefix(row, "PUT ")
				if strings.HasSuffix(where, ":") {
					op.Locator = strings.TrimSuffix(where, ":")
					for n+1 < len(rows) && strings.HasPrefix(rows[n+1], "+") {
						n++
						op.Body = append(op.Body, rows[n][1:])
					}
					if len(op.Body) == 0 {
						return nil, fmt.Errorf("patch line %d: PUT needs +text body lines; use CUT to delete", op.Row)
					}
				} else {
					fields := strings.Fields(where)
					if len(fields) < 1 || len(fields) > 2 {
						return nil, fmt.Errorf("patch line %d: PUT takes a locator and optional @register", n+1)
					}
					op.Locator = fields[0]
					if len(fields) == 2 {
						op.Register = fields[1]
					}
				}
			default:
				return nil, fmt.Errorf("patch line %d: expected PUT, CUT, REM or MV, got %q", n+1, row)
			}
			if op.Verb == "CUT" || op.Verb == "PUT" {
				if err := checkLocator(op); err != nil {
					return nil, fmt.Errorf("patch line %d: %w", op.Row, err)
				}
			}
			section.Ops = append(section.Ops, op)
			n++
		}
		if len(section.Ops) == 0 {
			return nil, fmt.Errorf("%s has no operations; add PUT/CUT/REM/MV beneath its header", p)
		}
		sections = append(sections, section)
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("patch has no [PATH#TAG] file sections")
	}
	return sections, nil
}

// loc is a half-open original line range, or an insertion gap between lines.
type loc struct{ Start, End, Gap int }

func checkLocator(op patchOp) error {
	_, err := parseLocator(op.Locator, op.Verb == "PUT")
	if err != nil {
		return err
	}
	if op.Verb == "CUT" && len(op.Body) > 0 {
		return fmt.Errorf("CUT has no body")
	}
	if op.Register != "" {
		if !validRegister(op.Register) {
			return fmt.Errorf("register %q must be @name with letters, digits, underscore or hyphen", op.Register)
		}
		if len(op.Body) != 0 {
			return fmt.Errorf("PUT with @register takes no +body lines")
		}
	} else if op.Verb == "PUT" && len(op.Body) == 0 {
		if !strings.HasPrefix(op.Locator, "<") && !strings.HasPrefix(op.Locator, ">") {
			return fmt.Errorf("a range paste needs @register")
		}
	}
	return nil
}

func validRegister(name string) bool {
	if !strings.HasPrefix(name, "@") || len(name) < 2 {
		return false
	}
	for _, r := range name[1:] {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func parseLocator(s string, put bool) (loc, error) {
	if strings.HasSuffix(s, "*") {
		return loc{}, fmt.Errorf("block anchor %q needs a syntax tree; use an explicit N.=M range for this block", s)
	}
	if put && s == ">$" {
		return loc{Gap: -1}, nil
	}
	if put && len(s) > 1 && (s[0] == '<' || s[0] == '>') {
		n, err := positiveLine(s[1:])
		if err != nil {
			return loc{}, err
		}
		if s[0] == '<' {
			return loc{Gap: n - 1}, nil
		}
		return loc{Gap: n}, nil
	}
	from, to, ok := strings.Cut(s, ".=")
	if !ok {
		return loc{}, fmt.Errorf("locator %q needs N.=M (inclusive) or %s", s, map[bool]string{true: "<N, >N or >$", false: "an N.=M range"}[put])
	}
	a, err := positiveLine(from)
	if err != nil {
		return loc{}, err
	}
	b, err := positiveLine(to)
	if err != nil {
		return loc{}, err
	}
	if b < a || b-a > 100_000 {
		return loc{}, fmt.Errorf("range %q must be ordered and at most 100,000 lines", s)
	}
	return loc{Start: a - 1, End: b}, nil
}

func positiveLine(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 100_000_000 {
		return 0, fmt.Errorf("line %q must be a positive number", s)
	}
	return n, nil
}
