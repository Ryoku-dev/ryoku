.pragma library

var entries = [
    { id: "logo", file: "Logo.qml", label: "Logo", icon: "adjust", card: "", desc: "Open Orpheus settings", home: "left" },
    { id: "workspaces", file: "Workspaces.qml", label: "Workspaces", icon: "view_carousel", card: "", desc: "Browse and switch workspaces", home: "left" },
    { id: "window", file: "Window.qml", label: "Window", icon: "web_asset", card: "", desc: "Show the focused window", home: "left" },
    { id: "voice", file: "../voice/Voice.qml", label: "Voice", icon: "graphic_eq", card: "", desc: "Show what needs attention", centerOnly: true },
    { id: "media", file: "Media.qml", label: "Media", icon: "music_note", card: "media", desc: "Control current media" },
    { id: "weather", file: "Weather.qml", label: "Weather", icon: "partly_cloudy_day", card: "weather", desc: "Show current weather" },
    { id: "sysmon", file: "Sysmon.qml", label: "System", icon: "monitoring", card: "system", desc: "Watch system resources" },
    { id: "keyboard", file: "Keyboard.qml", label: "Keyboard", icon: "keyboard", card: "", desc: "Switch keyboard layouts" },
    { id: "activity", file: "Activity.qml", label: "Activity", icon: "sensors", card: "", desc: "Show recording and microphone use" },
    { id: "ai", file: "Ai.qml", label: "AI usage", icon: "auto_awesome", card: "", desc: "Watch coding-agent usage limits" },
    { id: "phone", file: "Phone.qml", label: "Phone", icon: "smartphone", card: "", desc: "Open phone tools in Kit" },
    { id: "vpn", file: "Vpn.qml", label: "VPN", icon: "vpn_lock", card: "vpn", desc: "Watch and switch VPN tunnels" },
    { id: "updates", file: "Updates.qml", label: "Updates", icon: "system_update_alt", card: "", desc: "Show pending updates" },
    { id: "tray", file: "Tray.qml", label: "Tray", icon: "apps", card: "tray", desc: "Open background applications" },
    { id: "notifications", file: "Notifications.qml", label: "Notifications", icon: "notifications", card: "notifications", desc: "Read notifications" },
    { id: "controls", file: "Controls.qml", label: "Controls", icon: "tune", card: "controls", desc: "Open quick controls" },
    { id: "battery", file: "Battery.qml", label: "Battery", icon: "battery_full", card: "power", desc: "Show battery and power" },
    { id: "clock", file: "Clock.qml", label: "Clock", icon: "schedule", card: "today", desc: "Show time and calendar" },
    { id: "actions", file: "Actions.qml", label: "Shortcuts", icon: "bolt", card: "", desc: "Open chosen shortcuts", home: "left" },
    { id: "power", file: "Power.qml", label: "Power", icon: "power_settings_new", card: "session", desc: "Open session actions" }
]

var defaultLayout = {
    left: ["logo", "workspaces"],
    center: ["voice"],
    right: ["tray", "controls", "battery"]
}

function find(id) {
    for (var i = 0; i < entries.length; ++i) {
        if (entries[i].id === id)
            return entries[i]
    }
    return null
}
