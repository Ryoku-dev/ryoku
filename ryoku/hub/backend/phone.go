package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/godbus/dbus/v5"
)

var phoneDeviceID = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

type hubPhoneStatus struct {
	Manager       string           `json:"manager"`
	Installed     bool             `json:"installed"`
	Configured    bool             `json:"configured"`
	Supported     bool             `json:"supported"`
	Reason        string           `json:"reason,omitempty"`
	Devices       []hubPhoneDevice `json:"devices"`
	PrimaryDevice *hubPhoneDevice  `json:"primaryDevice,omitempty"`
}

type hubPhoneDevice struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Paired    bool   `json:"paired"`
	Reachable bool   `json:"reachable"`
	Battery   int    `json:"battery"`
	Charging  bool   `json:"charging"`
}

func runPhone(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("phone needs status|on|off|plan-enable|plan-disable|devices|pair|unpair")
	}
	switch args[0] {
	case "status":
		status, err := readHubPhoneStatus()
		if err != nil {
			return err
		}
		body, err := json.Marshal(status)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	case "devices":
		devices, err := listHubPhoneDevices()
		if err != nil {
			return err
		}
		body, err := json.Marshal(devices)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	case "on":
		return privileged("phone", "enable")
	case "off":
		return privileged("phone", "disable")
	case "plan-enable", "plan-disable":
		cmd := exec.Command("ryoku-host", "phone", args[0])
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("phone %s: %s", args[0], strings.TrimSpace(string(out)))
		}
		fmt.Print(string(out))
		return nil
	case "pair", "unpair":
		if len(args) != 2 || !phoneDeviceID.MatchString(args[1]) {
			return fmt.Errorf("phone %s needs a valid device id", args[0])
		}
		out, err := exec.Command("kdeconnect-cli", "--device", args[1], "--"+args[0]).CombinedOutput()
		if err != nil {
			return fmt.Errorf("kdeconnect %s: %s", args[0], strings.TrimSpace(string(out)))
		}
		return nil
	default:
		return fmt.Errorf("unknown phone command: %s", args[0])
	}
}

func readHubPhoneStatus() (hubPhoneStatus, error) {
	out, err := exec.Command("ryoku-host", "phone", "status").CombinedOutput()
	if err != nil {
		return hubPhoneStatus{}, fmt.Errorf("phone status: %s", strings.TrimSpace(string(out)))
	}
	var status hubPhoneStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return hubPhoneStatus{}, fmt.Errorf("phone status: %w", err)
	}
	if status.Installed {
		status.Devices, err = listHubPhoneDevices()
		if err != nil {
			status.Reason = err.Error()
		}
	}
	for i := range status.Devices {
		if status.Devices[i].Paired {
			device := status.Devices[i]
			status.PrimaryDevice = &device
			break
		}
	}
	if status.Devices == nil {
		status.Devices = []hubPhoneDevice{}
	}
	return status, nil
}

func listHubPhoneDevices() ([]hubPhoneDevice, error) {
	return listHubPhoneDevicesDBus()
}

func listHubPhoneDevicesDBus() ([]hubPhoneDevice, error) {
	const (
		service     = "org.kde.kdeconnect"
		daemonPath  = dbus.ObjectPath("/modules/kdeconnect")
		daemonIface = "org.kde.kdeconnect.daemon"
		deviceIface = "org.kde.kdeconnect.device"
	)

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	var owned bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, service).Store(&owned); err != nil {
		return nil, err
	}
	if !owned {
		return []hubPhoneDevice{}, nil
	}

	var ids []string
	if err := conn.Object(service, daemonPath).Call(daemonIface+".devices", 0).Store(&ids); err != nil {
		return nil, err
	}
	devices := make([]hubPhoneDevice, 0, len(ids))
	for _, id := range ids {
		if !phoneDeviceID.MatchString(id) {
			continue
		}
		path := dbus.ObjectPath("/modules/kdeconnect/devices/" + id)
		props, propErr := hubPhoneProperties(conn, service, path, deviceIface)
		device := hubPhoneDevice{ID: id, Name: id, Battery: -1}
		if propErr == nil {
			device.Name = hubPhoneString(props, "name", id)
			device.Paired = hubPhoneBool(props, "isPaired")
			device.Reachable = hubPhoneBool(props, "isReachable")
		}
		batteryProps, batteryErr := hubPhoneProperties(conn, service, dbus.ObjectPath(string(path)+"/battery"), deviceIface+".battery")
		if batteryErr == nil {
			device.Battery = hubPhoneInt(batteryProps, "charge", -1)
			device.Charging = hubPhoneBool(batteryProps, "isCharging")
		}
		devices = append(devices, device)
	}
	sortHubPhoneDevices(devices)
	return devices, nil
}

func hubPhoneProperties(conn *dbus.Conn, service string, path dbus.ObjectPath, iface string) (map[string]dbus.Variant, error) {
	props := map[string]dbus.Variant{}
	err := conn.Object(service, path).
		Call("org.freedesktop.DBus.Properties.GetAll", 0, iface).
		Store(&props)
	return props, err
}

func hubPhoneString(props map[string]dbus.Variant, key, fallback string) string {
	if value, ok := props[key]; ok {
		if text, ok := value.Value().(string); ok && text != "" {
			return text
		}
	}
	return fallback
}

func hubPhoneBool(props map[string]dbus.Variant, key string) bool {
	if value, ok := props[key]; ok {
		result, _ := value.Value().(bool)
		return result
	}
	return false
}

func hubPhoneInt(props map[string]dbus.Variant, key string, fallback int) int {
	value, ok := props[key]
	if !ok {
		return fallback
	}
	switch number := value.Value().(type) {
	case int:
		return number
	case int16:
		return int(number)
	case int32:
		return int(number)
	case int64:
		return int(number)
	case uint:
		return int(number)
	case uint16:
		return int(number)
	case uint32:
		return int(number)
	case uint64:
		return int(number)
	default:
		return fallback
	}
}

func sortHubPhoneDevices(devices []hubPhoneDevice) {
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Reachable != devices[j].Reachable {
			return devices[i].Reachable
		}
		return strings.ToLower(devices[i].Name) < strings.ToLower(devices[j].Name)
	})
}

func privileged(args ...string) error {
	cmd := exec.Command("pkexec", append([]string{"ryoku-host"}, args...)...)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 126 || exit.ExitCode() == 127) {
			return fmt.Errorf("the password prompt was dismissed")
		}
		return fmt.Errorf("ryoku-host %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
