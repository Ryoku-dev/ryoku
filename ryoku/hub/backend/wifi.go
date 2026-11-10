package main

// wifi.go: `ryoku-hub wifi connect <ssid>`, the Connections page's
// password join. The shell daemon owns Wi-Fi joins (the bar's popout uses the
// same call), so the Hub goes through it: the passphrase reaches
// NetworkManager over D-Bus with no secret agent, so a rejected one fails as
// soon as NetworkManager drops the activation, and it never lands in argv or a
// file. The SSID's saved profiles, which the new passphrase replaces, are
// deleted only once the daemon has answered, so a join that cannot run leaves
// them in place.

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// wifiExitDaemonDown is the exit status for a join that never reached the
// daemon. Nothing was deleted or sent, so the page can still join without it.
const wifiExitDaemonDown = 3

// joinTimeout outlasts the daemon's own 30-second wait for the activation.
const joinTimeout = 40 * time.Second

// replaceProfiles deletes every saved Wi-Fi profile that joins ssid, matched
// by the SSID it joins rather than its name: NetworkManager names a second
// profile `<ssid> 1`, and an imported one can carry any name. A variable so
// tests can see whether, and when, it ran.
var replaceProfiles = func(ssid string) {
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
		got, err := exec.Command("nmcli", "-e", "no", "-g", "802-11-wireless.ssid", "connection", "show", "uuid", uuid).Output()
		if err == nil && strings.TrimSuffix(string(got), "\n") == ssid {
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
	return callOn(conn, "network.wifiConnect", map[string]any{"ssid": ssid, "password": pw}, nil, joinTimeout)
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
