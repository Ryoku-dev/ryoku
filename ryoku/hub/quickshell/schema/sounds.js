.pragma library

var title = "Sounds";
var lead = "The sounds the desktop itself plays, and a sound on every key press.";

var sections = [
    { id: "desktop", title: "Desktop sounds" },
    { id: "notifications", title: "Notifications" },
    { id: "power", title: "Power and battery" },
    { id: "devices", title: "Devices and screenshots" },
    { id: "key-sounds", title: "Key sounds", missing: "Key sounds need a window manager with a key sound engine." }
];

function s(key, fallback) {
    return { store: "settings", key: key, fallback: fallback };
}

var on = { bind: s("sounds.enabled", true), is: [true] };

// one desktop cue, stored as { enabled, file }: the row's switch turns it on or off,
// its control plays it and swaps its file
function cue(event, section, label, changeLabel, detail, keywords, enabled) {
    var stock = { enabled: enabled, file: "" };
    return {
        id: "sound-" + event, section: section, label: label, detail: detail, keywords: keywords,
        control: "custom", activatable: true, event: event, changeLabel: changeLabel,
        bind: s("sounds.events." + event, stock), "default": stock, when: on
    };
}

var rows = [
    { id: "sounds-on", section: "desktop", label: "Desktop sounds",
      detail: "Turn every sound below on or off at once; apps and music are not affected",
      keywords: ["mute", "silence", "chimes", "alerts", "system sounds", "audio cues"],
      control: "toggle", bind: s("sounds.enabled", true), "default": true },
    { id: "sounds-volume", section: "desktop", label: "Volume",
      detail: "How loud these sounds play, next to your output volume",
      keywords: ["loudness", "level", "quieter", "louder"],
      control: "slider", min: 0, max: 100, step: 1, unit: "%", changeLabel: "Desktop sounds volume",
      bind: s("sounds.volume", 100), "default": 100, when: on },

    cue("notification", "notifications", "New notification", "New notification sound",
        "Plays when a notification arrives; Do not disturb keeps it quiet",
        ["notification sound", "chime", "ding", "message"], true),
    cue("notification-critical", "notifications", "Urgent notification", "Urgent notification sound",
        "Plays for notifications an app marks as urgent",
        ["critical", "important", "alarm"], true),

    cue("power-plug", "power", "Charger plugged in", "Charger plugged in sound",
        "Plays when the power cable is connected",
        ["charging", "ac", "power cable", "battery"], true),
    cue("power-unplug", "power", "Charger unplugged", "Charger unplugged sound",
        "Plays when the power cable is removed",
        ["on battery", "ac", "power cable", "battery"], true),
    cue("battery-low", "power", "Battery very low", "Battery very low sound",
        "Repeats every minute while the battery is under 4% and not charging",
        ["battery alert", "critical battery", "warning", "power"], true),
    cue("battery-full", "power", "Battery full", "Battery full sound",
        "Plays once when the battery finishes charging",
        ["charged", "battery", "power"], true),

    cue("device-added", "devices", "USB device connected", "USB device connected sound",
        "Plays when a USB device is plugged in",
        ["usb", "plug in", "drive", "hotplug"], false),
    cue("device-removed", "devices", "USB device removed", "USB device removed sound",
        "Plays when a USB device is unplugged",
        ["usb", "unplug", "eject", "hotplug"], false),
    cue("shutter", "devices", "Screenshot", "Screenshot sound",
        "A shutter sound when a screenshot is taken",
        ["camera", "capture", "screen capture"], true),
    cue("volume-change", "devices", "Volume change", "Volume change sound",
        "A tick when the volume keys change the volume",
        ["volume keys", "tick", "click", "feedback"], true),

    { id: "keysounds-more", section: "key-sounds", label: "Find more sound profiles", roster: "sounds",
      detail: "Mechvibes sound packs you can add as profiles",
      keywords: ["mechvibes", "download", "packs", "switches"], control: "handoff", actionText: "Open",
      needs: { cap: "plugins" }, more: true },
    // on every window manager with key sounds: from the plugin roster where plugins exist, else
    // the folder both engines read, made on open (the page decides; see Sounds.qml prepared())
    { id: "keysounds-mine", section: "key-sounds", label: "Your sound profiles", roster: "userSounds",
      detail: "The folder for profiles you add or import yourself; each one appears in the Sound profile list",
      keywords: ["folder", "custom", "import", "my sounds"], control: "handoff", actionText: "Open",
      more: true },
    { id: "keysounds-shipped", section: "key-sounds", label: "Profiles that come with Ryoku", roster: "shippedSounds",
      detail: "The folder holding the shipped profiles",
      keywords: ["folder", "shipped", "built in"], control: "handoff", actionText: "Open",
      needs: { cap: "plugins" }, more: true }
];

var related = ["notifications", "power", "keyboard"];
