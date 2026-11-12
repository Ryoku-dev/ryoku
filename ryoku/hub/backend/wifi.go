package main

// wifi.go: `ryoku-hub wifi connect <ssid>`, the Connections page's
// password join. The shell daemon owns Wi-Fi joins (the bar's popout uses the
// same call), so the Hub goes through it: the passphrase reaches
// NetworkManager over D-Bus with no secret agent, so a rejected one fails as
// soon as NetworkManager drops the activation, and it never lands in argv or a
// file. The SSID's saved profiles, which the new passphrase replaces, are
// deleted only once the daemon's socket accepts the connection. A join the
// daemon could not run for any reason other than a rejected passphrase exits
// with wifiExitDaemonDown, so the page can still join through nmcli, as it did
// before the daemon owned the join.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// wifiExitDaemonDown is the exit status for a join the daemon did not run:
// it never answered (nothing was deleted or sent), or it answered with
// anything but a rejected passphrase (no Wi-Fi support in this daemon, no
// device, a profile NetworkManager would not take, a timeout).
const wifiExitDaemonDown = 3

// errJoinNotRun marks a daemon answer that is not a rejected passphrase.
var errJoinNotRun = errors.New("the shell daemon could not run the join")

// joinRejected is the daemon's errActivationFailed text in
// ryoku/shell/ipc/network.go, the one answer that means NetworkManager
// refused the passphrase.
const joinRejected = "could not connect (check the password)"

// joinTimeout outlasts the daemon's own 30-second wait for the activation.
const joinTimeout = 40 * time.Second

// replaceProfiles deletes every saved Wi-Fi profile that joins ssid, matched
// by the SSID it joins rather than its name: NetworkManager names a second
// profile `<ssid> 1`, and an imported one can carry any name. A variable so
// tests can see whether, and when, it ran.
var replaceProfiles = func(ssid string) {
	// a hotspot (mode ap) is not a saved network, even when it shares the name
	out, err := exec.Command("nmcli", "-t", "-f", "UUID,TYPE", "connection", "show").Output()
	if err != nil {
		return
	}
	args := []string{"connection", "delete"}
	for _, line := range strings.Split(string(out), "\n") {
		uuid, typ, ok := strings.Cut(line, ":")
		if !ok || typ != "802-11-wireless" {
			continue
		}
		got, err := exec.Command("nmcli", "-e", "no", "-g", "802-11-wireless.mode,802-11-wireless.ssid", "connection", "show", "uuid", uuid).Output()
		if err != nil {
			continue
		}
		mode, s, _ := strings.Cut(strings.TrimSuffix(string(got), "\n"), "\n")
		if mode != "ap" && s == ssid {
			args = append(args, "uuid", uuid)
		}
	}
	if len(args) > 2 {
		_ = exec.Command("nmcli", args...).Run()
	}
}

func runWifi(args []string, stdin io.Reader) error {
	if len(args) != 2 || args[0] != "connect" || args[1] == "" {
		return fmt.Errorf("wifi needs connect <ssid>")
	}
	ssid := args[1]
	pw, err := readPassphrase(stdin)
	if err != nil {
		return err
	}
	conn, err := dialDaemon()
	if err != nil {
		return err
	}
	defer conn.Close()
	replaceProfiles(ssid)
	err = callOn(conn, "network.wifiConnect", map[string]any{"ssid": ssid, "password": pw}, nil, joinTimeout)
	if err != nil && !strings.Contains(err.Error(), joinRejected) {
		return fmt.Errorf("%w: %v", errJoinNotRun, err)
	}
	return err
}

// readPassphrase takes the first line of stdin, where the page writes the
// passphrase before closing it.
func readPassphrase(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	pw := strings.TrimSuffix(line, "\n")
	if pw == "" {
		return "", fmt.Errorf("no passphrase on stdin")
	}
	return pw, nil
}
