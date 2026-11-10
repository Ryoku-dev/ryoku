package main

import "sort"

// Command is one rashin subcommand. Each command group registers its commands
// from an init function in its own file; main dispatches by name.
type Command struct {
	Name    string
	Group   string // lifecycle, ask, sessions, providers, index, tools
	Usage   string // the argument synopsis after the name
	Summary string // one line for the help listing
	Run     func(args []string) int
}

var commands = map[string]*Command{}

// register adds c; a duplicate name is a programming error.
func register(c *Command) {
	if _, dup := commands[c.Name]; dup {
		panic("rashin: command registered twice: " + c.Name)
	}
	commands[c.Name] = c
}

// lookup returns the command called name.
func lookup(name string) (*Command, bool) {
	c, ok := commands[name]
	return c, ok
}

// sortedCommands lists commands by group, then name.
func sortedCommands() []*Command {
	out := make([]*Command, 0, len(commands))
	for _, c := range commands {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		return out[i].Name < out[j].Name
	})
	return out
}
