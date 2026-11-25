package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

func init() {
	register(&Command{Name: "providers", Group: "providers", Usage: "[list|add|rm|test|enable|disable]", Summary: "Manage model providers: free, paid and on this computer.", Run: runProviders})
	register(&Command{Name: "models", Group: "providers", Usage: "[provider]", Summary: "List available models.", Run: runModels})
	register(&Command{Name: "usage", Group: "providers", Usage: "[--days n]", Summary: "Show model usage.", Run: runUsage})
}

func runProviders(args []string) int {
	if len(args) == 0 || args[0] == "list" {
		if len(args) > 1 {
			return usageError("providers [list]")
		}
		conn, cancel, err := dial(true)
		if err != nil {
			return report(err)
		}
		defer cancel()
		defer conn.Close()
		var result protocol.ProvidersResult
		if err := conn.Call(context.Background(), protocol.MProvidersList, protocol.Empty{}, &result); err != nil {
			return report(err)
		}
		for _, p := range result.Providers {
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%d/%d", p.ID, p.Tier, providerState(p), p.Available, p.Models)
			if p.Local {
				fmt.Fprintf(os.Stdout, "\t%s", p.Address)
			}
			fmt.Fprintln(os.Stdout)
		}
		return 0
	}
	switch args[0] {
	case "add":
		return providerAdd(args[1:])
	case "rm":
		return providerRemove(args[1:])
	case "test":
		return providerTest(args[1:])
	case "enable":
		return providerEnable(args[1:], true)
	case "disable":
		return providerEnable(args[1:], false)
	default:
		return usageError("providers [list|add|rm|test|enable|disable]")
	}
}

func providerState(p protocol.ProviderInfo) string {
	switch {
	case !p.Enabled:
		return i18n.T("disabled")
	case p.Local && !p.Running:
		return i18n.T("not running")
	case p.Available == 0:
		return i18n.T("unavailable")
	default:
		return i18n.T("available")
	}
}

func providerAdd(args []string) int {
	if len(args) != 1 && len(args) != 3 {
		return usageError("providers add <id> [--label label]")
	}
	label := ""
	if len(args) == 3 {
		if args[1] != "--label" {
			return usageError("providers add <id> [--label label]")
		}
		label = args[2]
	}
	key, err := readSecret(i18n.T("Provider key: "))
	if err != nil {
		return report(err)
	}
	if key == "" {
		return report(fmt.Errorf("the provider key is empty"))
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	var result protocol.ProviderInfo
	if err := conn.Call(context.Background(), protocol.MProvidersAddKey, protocol.AddKeyParams{Provider: args[0], Key: key, Label: label}, &result); err != nil {
		return report(err)
	}
	fmt.Fprintln(os.Stdout, result.Name)
	return 0
}

func providerRemove(args []string) int {
	if len(args) != 2 {
		return usageError("providers rm <id> <key-id>")
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	return callResult(conn.Call(context.Background(), protocol.MProvidersRemoveKey, protocol.RemoveKeyParams{Provider: args[0], Key: args[1]}, &protocol.ProviderInfo{}))
}

func providerTest(args []string) int {
	if len(args) != 1 {
		return usageError("providers test <id>")
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	var result protocol.ProviderTestResult
	if err := conn.Call(context.Background(), protocol.MProvidersTest, protocol.ProviderParams{Provider: args[0]}, &result); err != nil {
		return report(err)
	}
	if result.OK {
		fmt.Fprintln(os.Stdout, i18n.T("Provider test passed."))
		return 0
	}
	return report(fmt.Errorf("%s", result.Message))
}

func providerEnable(args []string, enabled bool) int {
	if len(args) != 1 {
		return usageError("providers enable|disable <id>")
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	return callResult(conn.Call(context.Background(), protocol.MProvidersEnable, protocol.EnableProviderParams{Provider: args[0], Enabled: enabled}, &protocol.ProviderInfo{}))
}

func runModels(args []string) int {
	if len(args) > 1 {
		return usageError("models [provider]")
	}
	provider := ""
	if len(args) == 1 {
		provider = args[0]
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	var result protocol.ModelsResult
	if err := conn.Call(context.Background(), protocol.MModelsList, protocol.ModelsParams{Provider: provider}, &result); err != nil {
		return report(err)
	}
	printModels(os.Stdout, result)
	return 0
}

func printModels(w io.Writer, result protocol.ModelsResult) {
	for _, route := range result.Routes {
		fmt.Fprintf(w, "%s\t%s\n", route.ID, route.Name)
	}
	for _, model := range result.Models {
		state := i18n.T("unavailable")
		if model.Available {
			state = i18n.T("available")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", model.ID, model.Tier, state)
	}
}

func runUsage(args []string) int {
	days := 7
	if len(args) == 2 && args[0] == "--days" {
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 1 {
			return usageError("usage [--days n]")
		}
		days = n
	} else if len(args) != 0 {
		return usageError("usage [--days n]")
	}
	conn, cancel, err := dial(true)
	if err != nil {
		return report(err)
	}
	defer cancel()
	defer conn.Close()
	var result protocol.GatewayUsage
	if err := conn.Call(context.Background(), protocol.MGatewayUsage, protocol.UsageParams{Days: days}, &result); err != nil {
		return report(err)
	}
	for _, row := range result.Rows {
		fmt.Fprintf(os.Stdout, "%s\t%s/%s\t%d\t%d/%d\n", row.Day, row.Provider, row.Model, row.Requests, row.InputTokens, row.OutputTokens)
	}
	return 0
}

func callResult(err error) int {
	if err != nil {
		return report(err)
	}
	return 0
}

func readSecret(prompt string) (string, error) {
	if !stdinTTY() {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		return strings.TrimSpace(line), errIfNotEOF(err)
	}
	fmt.Fprint(os.Stderr, prompt)
	fd := uintptr(os.Stdin.Fd())
	var old syscall.Termios
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&old)), 0, 0, 0); errno != 0 {
		return "", errno
	}
	quiet := old
	quiet.Lflag &^= syscall.ECHO
	if _, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(&quiet)), 0, 0, 0); errno != 0 {
		return "", errno
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL, fd, syscall.TCSETS, uintptr(unsafe.Pointer(&old)), 0, 0, 0)
	fmt.Fprintln(os.Stderr)
	return strings.TrimSpace(line), errIfNotEOF(err)
}

func errIfNotEOF(err error) error {
	if err != nil && err != io.EOF {
		return err
	}
	return nil
}
