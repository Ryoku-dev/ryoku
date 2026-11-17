.pragma library

// GpuPage as data. Generated from the page it replaces.
// Descriptions are written by hand; the inventory carries engineering
// notes, which are not user copy.

function graphicsModeOptions(performanceAvailable) {
    return performanceAvailable
        ? ["Hybrid", "Performance", "Passthrough"]
        : ["Hybrid", "Passthrough"];
}

var rows = [
    {
        "tab": "",
        "group": "RYOKU RENDERS ON",
        "key": "AQ_DRM_DEVICES",
        "label": "Graphics mode",
        "desc": "Which GPU renders the desktop; next login",
        "gate": "ryoku-gpu status --json: performance_available, performance_route (external: pinned only while a dGPU port has a monitor at login)",
        "unavailable": "Switch the laptop's graphics MUX to discrete first.",
        "ctl": "seg",
        "src": "gpu.lua (override path via $RYOKU_GPU_CONF; base honours $XDG_CONFIG_HOME)",
        "opts": [
            "hybrid",
            "performance",
            "passthrough"
        ]
    },
    {
        "tab": "",
        "group": "CPU POWER PROFILES",
        "key": "",
        "label": "Editing profile",
        "desc": "Which profile you're editing, not the live one.",
        "ctl": "seg",
        "src": "ryoku-hub cpu active (ryoku-host power)",
        "opts": [
            "power-saver",
            "balanced",
            "performance"
        ]
    },
    {
        "tab": "",
        "group": "CPU POWER PROFILES",
        "key": "",
        "label": "Governor",
        "desc": "CPU scaling governor for the edited profile.",
        "ctl": "seg",
        "src": "ryoku-host power profile (scaling_governor, power.json)",
        "opts": [
            "performance",
            "powersave"
        ]
    },
    {
        "tab": "",
        "group": "CPU POWER PROFILES",
        "key": "",
        "label": "Energy preference",
        "desc": "Energy or performance hint",
        "ctl": "seg",
        "src": "ryoku-host power profile (energy_performance_preference, power.json)",
        "opts": [
            "default",
            "performance",
            "balance_performance",
            "balance_power",
            "power"
        ]
    },
    {
        "tab": "",
        "group": "CPU POWER PROFILES",
        "key": "",
        "label": "Max frequency",
        "desc": "Ceiling for the CPU clock, in percent",
        "ctl": "slid",
        "src": "ryoku-host power profile (scaling_max_freq, power.json)"
    },
    {
        "tab": "",
        "group": "CPU POWER PROFILES",
        "key": "",
        "label": "Thermal profile",
        "desc": "Fan and power envelope for the edited profile.",
        "ctl": "seg",
        "src": "ryoku-host power profile (platform_profile, power.json)",
        "opts": [
            "quiet",
            "balanced",
            "performance"
        ]
    },
    {
        "tab": "",
        "group": "CPU POWER PROFILES",
        "key": "",
        "label": "CPU boost and PPT/TDP limits",
        "desc": "Not exposed: firmware governs boost and PPT here.",
        "ctl": "readout",
        "src": "static copy"
    },
    {
        "tab": "",
        "group": "TUNING \u00b7 THIS SESSION",
        "key": "",
        "label": "Power limit / TDP",
        "desc": "GPU power budget in watts, live for this session.",
        "ctl": "slid",
        "src": "ryoku-hub gpu tune (runtime, resets on reboot)"
    },
    {
        "tab": "",
        "group": "TUNING \u00b7 THIS SESSION",
        "key": "",
        "label": "Performance level",
        "desc": "AMD performance level: auto, low, or high.",
        "ctl": "seg",
        "src": "ryoku-hub gpu tune (runtime, resets on reboot)"
    },
    {
        "tab": "",
        "group": "TUNING \u00b7 THIS SESSION",
        "key": "",
        "label": "Persistence mode",
        "desc": "Keeps the NVIDIA driver loaded",
        "ctl": "sw",
        "src": "ryoku-hub gpu tune (runtime, resets on reboot)"
    },
    {
        "tab": "",
        "group": "TUNING \u00b7 THIS SESSION",
        "key": "",
        "label": "Overclock, undervolt and fan control",
        "desc": "GPU clock and fan control; resets on reboot",
        "ctl": "slid",
        "src": "ryoku-hub gpu tune (runtime, resets on reboot)"
    },
    {
        "tab": "",
        "group": "TUNING \u00b7 THIS SESSION",
        "key": "",
        "label": "Tuning presets",
        "desc": "Save and apply named tuning bundles.",
        "ctl": "action",
        "src": "~/.config/ryoku/gpu-presets.json"
    },
    {
        "tab": "",
        "group": "BATTERY",
        "key": "",
        "label": "Charge limit",
        "desc": "Stop charging here to preserve battery health.",
        "ctl": "slid",
        "src": "ryoku-host power charge-limit (kernel or vendor battery threshold, power.json)"
    },
    {
        "tab": "",
        "group": "BATTERY",
        "key": "",
        "label": "PCIe ASPM",
        "desc": "PCIe power policy: trade idle power for latency.",
        "ctl": "seg",
        "src": "ryoku-host power aspm (pcie_aspm/parameters/policy, power.json)",
        "opts": [
            "default",
            "performance",
            "powersave",
            "powersupersave"
        ]
    },
    {
        "tab": "",
        "group": "GPU PASSTHROUGH \u00b7 ADVANCED",
        "key": "",
        "label": "Readiness checks",
        "desc": "",
        "ctl": "sw",
        "src": "none (transient page state: page.showChecks)"
    },
    {
        "tab": "",
        "group": "GPU PASSTHROUGH \u00b7 ADVANCED",
        "key": "",
        "label": "Disable passthrough",
        "desc": "",
        "ctl": "action",
        "src": "ryoku-host pkg available; ryoku-hub gpu apply disable"
    },
    {
        "tab": "",
        "group": "GPU PASSTHROUGH \u00b7 ADVANCED",
        "key": "",
        "label": "Review changes",
        "desc": "",
        "ctl": "action",
        "src": "ryoku-host capabilities + pkg available; ryoku-hub gpu apply enable --dry-run"
    },
    {
        "tab": "",
        "group": "GPU PASSTHROUGH \u00b7 ADVANCED",
        "key": "",
        "label": "Enable passthrough",
        "desc": "",
        "ctl": "action",
        "src": "ryoku-host capabilities + pkg install; kvm; enables libvirtd; kvmfr static_size_mb=128"
    },
    {
        "tab": "",
        "group": "GPU PASSTHROUGH \u00b7 ADVANCED",
        "key": "",
        "label": "Close",
        "desc": "",
        "ctl": "action",
        "src": "none"
    },
    {
        "tab": "",
        "group": "GPU PASSTHROUGH \u00b7 ADVANCED",
        "key": "",
        "label": "Recheck",
        "desc": "",
        "ctl": "action",
        "src": "none"
    },
    {
        "tab": "",
        "group": "(no SettingSection - floating error column under the hero card)",
        "key": "",
        "label": "Retry",
        "desc": "",
        "ctl": "action",
        "src": "none"
    },
    {
        "tab": "",
        "group": "GPU PASSTHROUGH \u00b7 ADVANCED",
        "key": "",
        "label": "Passthrough status",
        "desc": "",
        "ctl": "readout",
        "src": "`ryoku-hub gpu caps` -> caps.verdict"
    },
    {
        "tab": "",
        "group": "GPU PASSTHROUGH \u00b7 ADVANCED",
        "key": "",
        "label": "Readiness check details",
        "desc": "",
        "ctl": "readout",
        "src": "hwcaps.go buildChecks)"
    },
    {
        "tab": "",
        "group": "RYOKU RENDERS ON",
        "key": "",
        "label": "Graphics mode explainer",
        "desc": "",
        "ctl": "readout",
        "src": "derived from page.mode + page.dgpuName"
    },
    {
        "tab": "",
        "group": "GPU PASSTHROUGH \u00b7 ADVANCED",
        "key": "",
        "label": "Passthrough section intro",
        "desc": "",
        "ctl": "readout",
        "src": "static copy + page.dgpuName"
    },
    {
        "tab": "",
        "group": "IDLE",
        "key": "idle.enabled",
        "label": "Idle timeouts",
        "desc": "Dim, lock, blank and suspend the machine when it sits idle.",
        "ctl": "sw",
        "src": "ryoku-hub cpu set idle enabled (power.json, ryoku-idle apply)"
    },
    {
        "tab": "",
        "group": "IDLE",
        "key": "idle.onDesktops",
        "label": "Also on desktops",
        "desc": "Run these timeouts on this desktop too, not only on laptops.",
        "ctl": "sw",
        "src": "ryoku-hub cpu set idle onDesktops (power.json, ryoku-idle apply)"
    },
    {
        "tab": "",
        "group": "ON BATTERY",
        "key": "idle.battery.dimSec",
        "label": "Dim",
        "desc": "Minutes idle on battery before the backlight dims; 0 never dims.",
        "ctl": "step",
        "src": "ryoku-hub cpu set idle battery.dimSec (power.json, minutes)",
        "unit": "min",
        "lo": 0,
        "hi": 60
    },
    {
        "tab": "",
        "group": "ON BATTERY",
        "key": "idle.battery.lockSec",
        "label": "Lock",
        "desc": "Minutes idle on battery before the session locks; 0 never locks.",
        "ctl": "step",
        "src": "ryoku-hub cpu set idle battery.lockSec (power.json, minutes)",
        "unit": "min",
        "lo": 0,
        "hi": 120
    },
    {
        "tab": "",
        "group": "ON BATTERY",
        "key": "idle.battery.screenOffSec",
        "label": "Screen off",
        "desc": "Minutes idle on battery before the screen powers off; 0 keeps it on.",
        "ctl": "step",
        "src": "ryoku-hub cpu set idle battery.screenOffSec (power.json, minutes)",
        "unit": "min",
        "lo": 0,
        "hi": 120
    },
    {
        "tab": "",
        "group": "ON BATTERY",
        "key": "idle.battery.suspendSec",
        "label": "Suspend",
        "desc": "Minutes idle on battery before the machine suspends; 0 never suspends.",
        "ctl": "step",
        "src": "ryoku-hub cpu set idle battery.suspendSec (power.json, minutes)",
        "unit": "min",
        "lo": 0,
        "hi": 240
    },
    {
        "tab": "",
        "group": "PLUGGED IN",
        "key": "idle.ac.dimSec",
        "label": "Dim",
        "desc": "Minutes idle on AC before the backlight dims; 0 never dims.",
        "ctl": "step",
        "src": "ryoku-hub cpu set idle ac.dimSec (power.json, minutes)",
        "unit": "min",
        "lo": 0,
        "hi": 60
    },
    {
        "tab": "",
        "group": "PLUGGED IN",
        "key": "idle.ac.lockSec",
        "label": "Lock",
        "desc": "Minutes idle on AC before the session locks; 0 never locks.",
        "ctl": "step",
        "src": "ryoku-hub cpu set idle ac.lockSec (power.json, minutes)",
        "unit": "min",
        "lo": 0,
        "hi": 120
    },
    {
        "tab": "",
        "group": "PLUGGED IN",
        "key": "idle.ac.screenOffSec",
        "label": "Screen off",
        "desc": "Minutes idle on AC before the screen powers off; 0 keeps it on.",
        "ctl": "step",
        "src": "ryoku-hub cpu set idle ac.screenOffSec (power.json, minutes)",
        "unit": "min",
        "lo": 0,
        "hi": 120
    },
    {
        "tab": "",
        "group": "PLUGGED IN",
        "key": "idle.ac.suspendSec",
        "label": "Suspend",
        "desc": "Minutes idle on AC before the machine suspends; 0 never suspends.",
        "ctl": "step",
        "src": "ryoku-hub cpu set idle ac.suspendSec (power.json, minutes)",
        "unit": "min",
        "lo": 0,
        "hi": 240
    }
];
