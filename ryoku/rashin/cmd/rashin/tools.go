package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "call", Group: "tools", Usage: "<method> [json]", Summary: "Call one daemon method.", Run: runCall})
	register(&Command{Name: "events", Group: "tools", Usage: "[names...]", Summary: "Print subscribed daemon events.", Run: runEvents})
	register(&Command{Name: "methods", Group: "tools", Summary: "List wire methods.", Run: runMethods})
	register(&Command{Name: "version", Group: "tools", Summary: "Print the Rashin version.", Run: runVersion})
}

func runCall(args []string) int {
	if len(args) < 1 || len(args) > 2 {
		return usageError("call <method> [json]")
	}
	if _, ok := protocol.Lookup(args[0]); !ok {
		return report(fmt.Errorf("method %s is not supported", args[0]))
	}
	params := json.RawMessage(`{}`)
	if len(args) == 2 {
		if !json.Valid([]byte(args[1])) {
			return report(fmt.Errorf("parameters are not valid JSON"))
		}
		params = json.RawMessage(args[1])
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	var result json.RawMessage
	if err := conn.Call(context.Background(), args[0], params, &result); err != nil {
		return report(err)
	}
	if len(result) == 0 {
		result = json.RawMessage(`{}`)
	}
	var pretty any
	if json.Unmarshal(result, &pretty) == nil {
		return printJSON(pretty)
	}
	fmt.Fprintln(os.Stdout, string(result))
	return 0
}

func runEvents(args []string) int {
	wanted := make(map[string]bool)
	for _, name := range args {
		wanted[name] = true
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	var snapshot protocol.Snapshot
	if err := conn.Call(context.Background(), protocol.MSubscribe, protocol.Empty{}, &snapshot); err != nil {
		return report(err)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	for {
		select {
		case <-sig:
			return 0
		case event, ok := <-conn.Events():
			if !ok {
				if conn.Err() != nil {
					return report(conn.Err())
				}
				return 0
			}
			if len(wanted) > 0 && !wanted[event.Event] {
				continue
			}
			encoded, _ := json.Marshal(event)
			fmt.Fprintln(os.Stdout, string(encoded))
		}
	}
}

func runMethods(args []string) int {
	if len(args) != 0 {
		return usageError("methods")
	}
	for _, method := range protocol.Methods {
		fmt.Fprintf(os.Stdout, "%s\t%s\n", method.Name, method.Doc)
	}
	return 0
}

func runVersion(args []string) int {
	if len(args) != 0 {
		return usageError("version")
	}
	fmt.Fprintln(os.Stdout, version)
	return 0
}
