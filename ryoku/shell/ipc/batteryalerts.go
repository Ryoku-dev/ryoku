package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type batteryAlertsSettings struct {
	WarningEnabled  bool `json:"warningEnabled"`
	CriticalEnabled bool `json:"criticalEnabled"`
	Enabled         bool `json:"enabled"`
	WarningPercent  int  `json:"warningPercent"`
	CriticalPercent int  `json:"criticalPercent"`
}

func (b *batteryAlertsSettings) normalize(v *validator) {
	v.rangeI("batteryAlerts.warningPercent", &b.WarningPercent, 5, 50)
	v.rangeI("batteryAlerts.criticalPercent", &b.CriticalPercent, 1, 49)
	if v.err != nil || b.CriticalPercent < b.WarningPercent {
		return
	}
	if v.strict {
		v.err = fmt.Errorf("batteryAlerts.criticalPercent: must be below warningPercent")
		return
	}
	b.CriticalPercent = b.WarningPercent - 1
}

var batteryAlertsTestIPC = func() string {
	return ipcCall("shell", "battery-alerts-native", "test", "")
}

func (d *daemon) registerBatteryAlertsCalls() {
	d.registerCall("battery-alerts.test", func(json.RawMessage) (any, error) {
		if result := batteryAlertsTestIPC(); result != "ok" {
			return nil, fmt.Errorf("%s", strings.TrimPrefix(result, "err "))
		}
		return nil, nil
	})
}
