.pragma library

var title = "Power & battery";
var lead = "How the computer uses power: its power mode, the battery, power saver and what happens when you step away.";

var sections = [
    { id: "mode", title: "Power mode" },
    { id: "battery", title: "Battery" },
    { id: "saver", title: "Power saver" },
    { id: "idle", title: "When idle" },
    { id: "idle-battery", title: "Idle on battery" },
    { id: "idle-ac", title: "Idle when plugged in" },
    { id: "power-key", title: "Power button" }
];

var related = ["graphics", "performance", "sounds", "displays"];

function s(key, fallback) { return { store: "settings", key: key, fallback: fallback }; }
function perf(key) { return { store: "performance", key: key }; }

var modeOptions = [
    { value: "power-saver", label: "Power saver" },
    { value: "balanced", label: "Balanced" },
    { value: "performance", label: "Performance" }
];

// rows marked cpu, battery or idle take their options and limits from what this
// machine reports; the page drops the ones it does not offer
var rows = [
    { id: "power-mode", section: "mode", label: "Power mode",
      detail: "How the computer balances speed against battery life and heat",
      keywords: ["power profile", "power profiles daemon", "ppd", "balanced", "power saver", "performance mode", "profile"],
      control: "segmented", options: modeOptions, bind: { store: "powermode", key: "profile" } },

    { id: "cpu-mode", section: "mode", more: true, label: "Change what a mode does",
      detail: "The settings below belong to this mode. Processor boost and power limits stay with the firmware.",
      keywords: ["cpu tuning", "processor", "profile definition", "edit profile", "boost", "ppt", "tdp"],
      control: "custom" },
    { id: "cpu-governor", section: "mode", more: true, cpu: "governor", label: "Processor governor",
      detail: "How the processor chooses its speed in this mode",
      keywords: ["governor", "scaling governor", "cpufreq", "cpu", "powersave"], control: "segmented",
      options: [{ value: "performance", label: "Performance" }, { value: "powersave", label: "Power save" }] },
    { id: "cpu-energy", section: "mode", more: true, cpu: "epp", label: "Energy preference",
      detail: "A hint to the processor: favour speed or save energy",
      keywords: ["epp", "energy performance preference", "cpu", "hint"], control: "picker",
      options: [{ value: "default", label: "Default" }, { value: "performance", label: "Performance" },
                { value: "balance_performance", label: "Lean to performance" },
                { value: "balance_power", label: "Lean to saving" }, { value: "power", label: "Save the most" }] },
    { id: "cpu-max-speed", section: "mode", more: true, cpu: "maxFreqPct", label: "Highest processor speed",
      detail: "A ceiling on the processor's clock, as a share of its top speed",
      keywords: ["max frequency", "scaling max freq", "clock", "cap", "cpu", "limit"], control: "slider", step: 1, unit: "%" },
    { id: "cpu-thermal", section: "mode", more: true, cpu: "platformProfile", label: "Fan and heat profile",
      detail: "How hard the fans and the power budget may push in this mode",
      keywords: ["platform profile", "thermal", "fan", "quiet", "acpi"], control: "segmented",
      options: [{ value: "low-power", label: "Low power" }, { value: "quiet", label: "Quiet" }, { value: "cool", label: "Cool" },
                { value: "balanced", label: "Balanced" }, { value: "balanced-performance", label: "Lean to speed" },
                { value: "performance", label: "Performance" }] },
    { id: "pcie-power", section: "mode", more: true, battery: "aspm", label: "PCIe power saving",
      detail: "Lets graphics, storage and network cards idle deeper, at a small cost when they wake",
      keywords: ["aspm", "pcie", "pcie aspm", "link power", "powersupersave"], control: "picker",
      options: [{ value: "default", label: "Default" }, { value: "performance", label: "Off" },
                { value: "powersave", label: "Save power" }, { value: "powersupersave", label: "Save the most" }] },

    { id: "battery-alerts", section: "battery", label: "Low battery alerts",
      detail: "Warn while the battery runs down. Alerts respect Do not disturb.",
      keywords: ["battery warning", "low battery", "notification", "alert"],
      control: "toggle", bind: s("batteryAlerts.enabled", true), "default": true },
    { id: "battery-warning", section: "battery", label: "First alert",
      detail: "A reminder to plug in", keywords: ["warning", "reminder"],
      control: "toggle", bind: s("batteryAlerts.warningEnabled", true), "default": true },
    { id: "battery-warning-level", section: "battery", label: "First alert at",
      detail: "Must be above the urgent alert", keywords: ["warning percent", "threshold", "level"],
      control: "stepper", unit: "%", min: 5, max: 50, bind: s("batteryAlerts.warningPercent", 25), "default": 25 },
    { id: "battery-critical", section: "battery", label: "Urgent alert",
      detail: "A last warning before the battery runs out", keywords: ["critical", "urgent"],
      control: "toggle", bind: s("batteryAlerts.criticalEnabled", true), "default": true },
    { id: "battery-critical-level", section: "battery", label: "Urgent alert at",
      detail: "Must be below the first alert", keywords: ["critical percent", "threshold", "level"],
      control: "stepper", unit: "%", min: 1, max: 49, bind: s("batteryAlerts.criticalPercent", 10), "default": 10 },
    { id: "battery-test", section: "battery", label: "Test alert",
      detail: "Show a battery alert now, without touching the battery", keywords: ["preview", "try"],
      control: "custom" },
    { id: "charge-limit", section: "battery", battery: "chargeLimit", label: "Charge limit",
      detail: "Stop charging at this level to keep the battery healthy for longer",
      keywords: ["charge threshold", "battery health", "80%", "conservation", "stop charging"],
      control: "slider", step: 1, unit: "%" },
    { id: "battery-sounds", section: "battery", label: "Battery sounds",
      detail: "Sounds when you plug in, unplug or run low live in Sounds",
      keywords: ["plug in sound", "unplug sound", "low battery sound", "chime"],
      control: "link", link: "sounds/sound-battery-low", actionText: "Open" },

    { id: "saver-on-battery", section: "saver", onBattery: true, label: "Switch to power saver on battery",
      detail: "Unplugging switches to power saver, and plugging back in brings your mode back",
      keywords: ["auto power saver", "automatic", "unplug", "battery saver"],
      control: "toggle", bind: perf("autoPowerSaverOnBattery"), "default": false },
    { id: "saver-effects", section: "saver", label: "Fewer effects in power saver",
      detail: "Power saver also turns off motion, blur and shadows",
      keywords: ["follow the power profile", "eye candy", "animations", "blur", "shadows"],
      control: "toggle", bind: perf("powerProfileEffects"), "default": true },
    { id: "saver-dim-keys", section: "saver", onBattery: true, label: "Dim the keyboard light",
      detail: "In power saver on battery, the keyboard light drops to its lowest level",
      keywords: ["keyboard backlight", "kbd", "keys"],
      control: "toggle", bind: perf("saverDimKeyboard"), "default": true },
    { id: "saver-60hz", section: "saver", onBattery: true, label: "Run the built-in screen at 60 Hz",
      detail: "In power saver on battery, a faster screen slows to 60 Hz",
      keywords: ["refresh rate", "60hz", "hz", "panel"],
      control: "toggle", bind: perf("saverPanel60"), "default": true },
    { id: "saver-dim-screen", section: "saver", onBattery: true, label: "Dim the screen",
      detail: "In power saver on battery, the screen drops to 60% of its brightness",
      keywords: ["brightness", "backlight", "dim"],
      control: "toggle", bind: perf("saverDimScreen"), "default": true },

    { id: "idle-on", section: "idle", idle: "enabled", label: "Act when idle",
      detail: "Dim, lock, turn off the screen and sleep after the times below",
      keywords: ["idle", "timeout", "hypridle", "screen saver", "auto lock", "auto sleep", "suspend"],
      control: "toggle" },
    { id: "idle-desktops", section: "idle", idle: "onDesktops", noBattery: true, label: "Use on a computer without a battery",
      detail: "Without this, the idle times only apply to laptops",
      keywords: ["desktop computer", "tower", "pc"], control: "toggle" },

    { id: "idle-battery-dim", section: "idle-battery", idle: "battery.dimSec", label: "Dim the screen after",
      detail: "0 means never", keywords: ["dim", "battery"], control: "stepper", unit: "min" },
    { id: "idle-battery-lock", section: "idle-battery", idle: "battery.lockSec", label: "Lock after",
      detail: "0 means never", keywords: ["lock", "battery", "auto lock"], control: "stepper", unit: "min" },
    { id: "idle-battery-screen", section: "idle-battery", idle: "battery.screenOffSec", label: "Turn the screen off after",
      detail: "0 means never", keywords: ["screen off", "blank", "dpms", "battery"], control: "stepper", unit: "min" },
    { id: "idle-battery-sleep", section: "idle-battery", idle: "battery.suspendSec", label: "Sleep after",
      detail: "0 means never", keywords: ["sleep", "suspend", "battery"], control: "stepper", unit: "min" },

    { id: "idle-ac-dim", section: "idle-ac", idle: "ac.dimSec", label: "Dim the screen after",
      detail: "0 means never", keywords: ["dim", "plugged in", "ac"], control: "stepper", unit: "min" },
    { id: "idle-ac-lock", section: "idle-ac", idle: "ac.lockSec", label: "Lock after",
      detail: "0 means never", keywords: ["lock", "plugged in", "ac", "auto lock"], control: "stepper", unit: "min" },
    { id: "idle-ac-screen", section: "idle-ac", idle: "ac.screenOffSec", label: "Turn the screen off after",
      detail: "0 means never", keywords: ["screen off", "blank", "dpms", "plugged in", "ac"], control: "stepper", unit: "min" },
    { id: "idle-ac-sleep", section: "idle-ac", idle: "ac.suspendSec", label: "Sleep after",
      detail: "0 means never", keywords: ["sleep", "suspend", "plugged in", "ac"], control: "stepper", unit: "min" }
];
