package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

type askOptions struct {
	buffer bool
	json   bool
	cont   bool
	last   bool
	resume *string
	words  []string
}

func init() {
	register(&Command{Name: "ask", Group: "ask", Usage: "[options] <question>", Summary: "Ask a read-only system question.", Run: runAsk})
}

func parseAskArgs(args []string) (askOptions, error) {
	var out askOptions
	positional := false
	for i := 0; i < len(args); i++ {
		if positional {
			out.words = append(out.words, args[i])
			continue
		}
		switch args[i] {
		case "--":
			positional = true
		case "--buffer":
			out.buffer = true
		case "--json":
			out.json = true
		case "-c":
			out.cont = true
		case "--last":
			out.last = true
		case "--resume":
			value := ""
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				value = args[i+1]
				i++
			}
			out.resume = &value
		default:
			if strings.HasPrefix(args[i], "-") {
				return out, fmt.Errorf("unknown option %s", args[i])
			}
			out.words = append(out.words, args[i])
		}
	}
	return out, nil
}

func runAsk(args []string) int {
	opts, err := parseAskArgs(args)
	if err != nil {
		return report(err)
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	if opts.resume != nil || opts.last {
		return replayAsk(conn, opts)
	}
	question := strings.TrimSpace(strings.Join(opts.words, " "))
	if question == "" {
		return usageError("ask [options] <question>")
	}
	origin := protocol.OriginCLI
	terminal := (*protocol.TerminalContext)(nil)
	if stdinTTY() || opts.buffer {
		origin = protocol.OriginTerminal
		cwd, _ := os.Getwd()
		terminal = &protocol.TerminalContext{Cwd: cwd, Shell: os.Getenv("SHELL"), LastCommand: os.Getenv("RASHIN_LAST_CMD"), Buffer: opts.buffer}
		if raw := os.Getenv("RASHIN_LAST_STATUS"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				terminal.LastStatus = &n
			}
		}
	}
	sessionID := ""
	if opts.cont {
		sessionID = latestAsk(conn, protocol.OriginTerminal)
	}
	var result protocol.AskResult
	ctx := context.Background()
	if err := conn.Call(ctx, protocol.MAsk, protocol.AskParams{Question: question, Session: sessionID, Origin: origin, Terminal: terminal}, &result); err != nil {
		return report(err)
	}
	return streamAnswer(conn, result.Session.ID, result.Since, opts)
}

func latestAsk(conn *client.Conn, origin string) string {
	var list protocol.SessionListResult
	if conn.Call(context.Background(), protocol.MSessionList, protocol.SessionListParams{Kind: protocol.KindAsk, Limit: 50}, &list) != nil {
		return ""
	}
	for _, info := range list.Sessions {
		if origin == "" || info.Origin == origin {
			return info.ID
		}
	}
	return ""
}

func replayAsk(conn *client.Conn, opts askOptions) int {
	var list protocol.SessionListResult
	if err := conn.Call(context.Background(), protocol.MSessionList, protocol.SessionListParams{Kind: protocol.KindAsk, Limit: 50}, &list); err != nil {
		return report(err)
	}
	selector := ""
	if opts.resume != nil {
		selector = *opts.resume
	}
	if opts.resume != nil && selector == "" {
		for i, info := range list.Sessions {
			fmt.Fprintf(os.Stdout, "%d\t%s\t%s\n", i+1, info.ID, sessionTableTitle(info.Title))
		}
		return 0
	}
	if opts.last {
		if len(list.Sessions) == 0 {
			return report(fmt.Errorf("no Ask sessions are available"))
		}
		selector = list.Sessions[0].ID
	}
	id := selector
	if n, err := strconv.Atoi(selector); err == nil {
		if n < 1 || n > len(list.Sessions) {
			return report(fmt.Errorf("session number %d is not available", n))
		}
		id = list.Sessions[n-1].ID
	}
	attached, err := attachAll(conn, id, 0)
	if err != nil {
		return report(err)
	}
	return printAskEvents(attached.Events, opts)
}

func attachAll(conn *client.Conn, id string, since int64) (protocol.SessionAttachResult, error) {
	var all protocol.SessionAttachResult
	for {
		var page protocol.SessionAttachResult
		if err := conn.Call(context.Background(), protocol.MSessionAttach, protocol.SessionAttachParams{ID: id, Since: since}, &page); err != nil {
			return protocol.SessionAttachResult{}, err
		}
		all.Info = page.Info
		all.Events = append(all.Events, page.Events...)
		if len(page.Events) > 0 {
			since = page.Events[len(page.Events)-1].Seq
		}
		if !page.More {
			return all, nil
		}
		if len(page.Events) == 0 {
			return protocol.SessionAttachResult{}, fmt.Errorf("%s", i18n.T("The session replay did not advance."))
		}
	}
}

func streamAnswer(conn *client.Conn, id string, since int64, opts askOptions) int {
	if stdinTTY() && !opts.json {
		fmt.Fprintln(os.Stderr, i18n.T("Working..."))
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	var events []protocol.SessionEvent
	for {
		select {
		case <-sig:
			_ = conn.Call(context.Background(), protocol.MSessionCancel, protocol.SessionRef{ID: id}, &protocol.Empty{})
		case wire, ok := <-conn.Events():
			if !ok {
				return report(conn.Err())
			}
			if wire.Event != protocol.EvtSessionEvent {
				continue
			}
			var data protocol.SessionEventData
			if json.Unmarshal(wire.Data, &data) != nil || data.Session != id || data.Seq <= since {
				continue
			}
			events = append(events, data.SessionEvent)
			if opts.json {
				encoded, _ := json.Marshal(data.SessionEvent)
				fmt.Fprintln(os.Stdout, string(encoded))
			}
			if data.Kind == protocol.EvTurnEnd {
				if opts.json {
					return 0
				}
				return printAskEvents(events, opts)
			}
		}
	}
}

func printAskEvents(events []protocol.SessionEvent, opts askOptions) int {
	var prose strings.Builder
	var plan *protocol.CommandPlan
	for _, event := range events {
		if opts.json {
			encoded, _ := json.Marshal(event)
			fmt.Fprintln(os.Stdout, string(encoded))
			continue
		}
		switch event.Kind {
		case protocol.EvText:
			var data protocol.TextData
			if json.Unmarshal(event.Data, &data) == nil {
				prose.WriteString(data.Delta)
			}
		case protocol.EvAnswer:
			var data protocol.AnswerData
			if json.Unmarshal(event.Data, &data) == nil {
				plan = data.Plan
			}
		case protocol.EvError:
			var data protocol.ErrorData
			if json.Unmarshal(event.Data, &data) == nil {
				fmt.Fprintln(os.Stderr, data.Message)
			}
		}
	}
	if opts.json {
		return 0
	}
	proseOut := os.Stdout
	if opts.buffer {
		proseOut = os.Stderr
	}
	if prose.Len() > 0 {
		fmt.Fprintln(proseOut, prose.String())
	}
	if plan != nil {
		for i, command := range plan.Commands {
			if opts.buffer {
				fmt.Fprintln(os.Stdout, command.Command)
			} else {
				fmt.Fprintf(os.Stdout, "%d. [%s] %s\n", i+1, command.Tier, command.Command)
			}
		}
	}
	return 0
}

func stdinTTY() bool {
	stat, err := os.Stdin.Stat()
	return err == nil && stat.Mode()&os.ModeCharDevice != 0
}
