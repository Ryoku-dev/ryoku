package main

import (
	"fmt"
	"os"
	"strconv"
)

// runDefaults declares the store leaves this provider writes. The daemon uses
// it to compute deadKeys; keys owned only by other providers are hidden from
// the Hub, so omitting a leaf here would hide a working control.
func runDefaults() error {
	const defaults = `{
  "desktop": {
    "input": {
      "sensitivity": 0,
      "accelProfile": "",
      "leftHanded": false,
      "mouseNaturalScroll": false,
      "naturalScroll": false,
      "middleClickPaste": true,
      "kbLayout": "",
      "kbVariant": "",
      "kbOptions": "",
      "repeatRate": 25,
      "repeatDelay": 600,
      "numlockByDefault": false,
      "tapToClick": true,
      "tapAndDrag": false,
      "disableWhileTyping": false,
      "clickfinger": true,
      "touchScrollFactor": 1,
      "followMouse": 1
    },
    "cursor": {
      "hideOnKeyPress": false,
      "inactiveTimeout": 0
    }
  },
  "wm": {
    "umbriel": {
      "warpMouseToFocus": false,
      "focusFollowsMouseScroll": 0.5,
      "workspaceAutoBackAndForth": false,
      "modKey": "Super",
      "dndEdgeViewScrollTriggerWidth": 30,
      "dndEdgeViewScrollDelayMs": 100,
      "dndEdgeViewScrollMaxSpeed": 1500
    }
  }
}
`
	_, err := stdout.WriteString(defaults)
	return err
}

// runEnvironment exports the live socket handle from a verified session
// process. A stale handle is dropped rather than republished.
func runEnvironment(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("environment: expected one process id")
	}
	pid, err := strconv.Atoi(args[0])
	if err != nil || pid <= 0 {
		return fmt.Errorf("environment: invalid process id %q", args[0])
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return err
	}
	handle := processEnvironmentValue(data, "UMBRIEL_SOCKET")
	if !alive(handle) {
		return nil
	}
	_, err = fmt.Fprintf(stdout, "UMBRIEL_SOCKET=%s%c", handle, byte(0))
	return err
}
