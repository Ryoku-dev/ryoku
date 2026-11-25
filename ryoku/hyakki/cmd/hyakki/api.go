package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/protocol"
)

const apiUsage = "api [status [--json]|on [--port n]|off|key [--rotate]]"

func init() {
	register(&Command{Name: "api", Group: "providers", Usage: "[status|on|off|key]", Summary: "Serve the model router to other programs.", Run: runAPI})
}

func runAPI(args []string) int {
	verb := "status"
	if len(args) > 0 {
		verb, args = args[0], args[1:]
	}
	switch verb {
	case "status":
		return apiStatus(args)
	case "on":
		return apiSwitch(args, true)
	case "off":
		return apiSwitch(args, false)
	case "key":
		return apiKey(args)
	}
	return usageError(apiUsage)
}

func apiStatus(args []string) int {
	asJSON := len(args) == 1 && args[0] == "--json"
	if len(args) != 0 && !asJSON {
		return usageError(apiUsage)
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	return apiReport(conn, asJSON)
}

func apiReport(conn *client.Conn, asJSON bool) int {
	var info protocol.APIInfo
	if err := conn.Call(context.Background(), protocol.MGatewayAPI, protocol.Empty{}, &info); err != nil {
		return report(err)
	}
	if asJSON {
		return printJSON(info)
	}
	switch {
	case !info.Enabled:
		fmt.Fprintf(os.Stdout, "%s\t%s\n", i18n.T("off"), info.URL)
	case info.Listening:
		fmt.Fprintf(os.Stdout, "%s\t%s\n", i18n.T("on"), info.URL)
	default:
		return report(fmt.Errorf("%s", info.Error))
	}
	return 0
}

func apiSwitch(args []string, on bool) int {
	patch := map[string]any{"enabled": on}
	switch {
	case on && len(args) == 2 && args[0] == "--port":
		port, err := strconv.Atoi(args[1])
		if err != nil {
			return usageError(apiUsage)
		}
		patch["port"] = port
	case len(args) != 0:
		return usageError(apiUsage)
	}
	raw, err := json.Marshal(map[string]any{"api": patch})
	if err != nil {
		return report(err)
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	if err := conn.Call(context.Background(), protocol.MSettingsSet, protocol.SettingsSetParams{Patch: raw}, &protocol.Settings{}); err != nil {
		return report(err)
	}
	return apiReport(conn, false)
}

func apiKey(args []string) int {
	rotate := len(args) == 1 && args[0] == "--rotate"
	if len(args) != 0 && !rotate {
		return usageError(apiUsage)
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	var key protocol.APIKey
	if err := conn.Call(context.Background(), protocol.MGatewayAPIKey, protocol.APIKeyParams{Rotate: rotate}, &key); err != nil {
		return report(err)
	}
	fmt.Fprintln(os.Stdout, key.Key)
	return 0
}
