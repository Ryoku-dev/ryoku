package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

type glanceImages []protocol.Attachment

func (p *glanceImages) String() string { return "" }
func (p *glanceImages) Set(path string) error {
	*p = append(*p, protocol.Attachment{Path: path})
	return nil
}

type glanceOptions map[string]string

func (o *glanceOptions) String() string { return "" }
func (o *glanceOptions) Set(value string) error {
	key, value, ok := strings.Cut(value, "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return fmt.Errorf("%s", i18n.T("Use --option key=value."))
	}
	if *o == nil {
		*o = make(glanceOptions)
	}
	(*o)[key] = value
	return nil
}

func init() {
	register(&Command{Name: "glance", Group: "ask", Usage: "--mode <action> [--option key=value] [options]", Summary: "Read, explain or work with a capture in Theia.", Run: runGlance})
}

func runGlance(args []string) int {
	var p protocol.GlanceParams
	var images glanceImages
	var options glanceOptions
	flags := flag.NewFlagSet("glance", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&p.Mode, "mode", "explain", i18n.T("Theia action: explain, ask, read, translate, summarize, deepread, solve, learn, define, rewrite, tone, fluent, humanize, reactions, reply or aicheck."))
	flags.StringVar(&p.Question, "question", "", i18n.T("Question about the capture."))
	flags.StringVar(&p.Text, "text", "", i18n.T("Text to read or translate."))
	flags.Var(&images, "image", i18n.T("Image path; may be repeated."))
	flags.Var(&options, "option", i18n.T("Action option as key=value; may be repeated."))
	flags.StringVar(&p.Target, "target", "", i18n.T("Translation language."))
	flags.StringVar(&p.Model, "model", "", i18n.T("Model id; empty uses the default."))
	flags.BoolVar(&p.Local, "local", false, i18n.T("Use only models on this computer."))
	flags.StringVar(&p.Session, "session", "", i18n.T("Continue a conversation."))
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		return usageError("glance --mode <action> [--option key=value] [options]")
	}
	p.Images = images
	p.Options = options
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	var result protocol.AskResult
	if err := conn.Call(context.Background(), protocol.MGlance, p, &result); err != nil {
		return report(err)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	failed, wrote := false, false
	for {
		select {
		case <-signals:
			_ = conn.Call(context.Background(), protocol.MSessionCancel, protocol.SessionRef{ID: result.Session.ID}, &protocol.Empty{})
			return 1
		case wire, ok := <-conn.Events():
			if !ok {
				return report(conn.Err())
			}
			if wire.Event != protocol.EvtSessionEvent {
				continue
			}
			var event protocol.SessionEventData
			if json.Unmarshal(wire.Data, &event) != nil || event.Session != result.Session.ID || event.Seq <= result.Since {
				continue
			}
			switch event.Kind {
			case protocol.EvText:
				var text protocol.TextData
				if json.Unmarshal(event.Data, &text) == nil {
					fmt.Fprint(os.Stdout, text.Delta)
					wrote = wrote || text.Delta != ""
				}
			case protocol.EvError:
				var failure protocol.ErrorData
				if json.Unmarshal(event.Data, &failure) == nil {
					fmt.Fprintln(os.Stderr, failure.Message)
				}
				failed = true
			case protocol.EvTurnEnd:
				if wrote {
					fmt.Fprintln(os.Stdout)
				}
				var end protocol.TurnEndData
				_ = json.Unmarshal(event.Data, &end)
				if failed || end.Stop == protocol.StopError || end.Stop == protocol.StopCancelled {
					return 1
				}
				return 0
			}
		}
	}
}
