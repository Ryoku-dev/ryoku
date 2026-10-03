import QtQuick
import Ryoku.Ui.Singletons

QtObject {
    function item(id) {
        switch (id) {
        case "app-launcher": return I18n.tr("App Launcher")
        case "audio-input": return I18n.tr("Audio Input")
        case "audio-output": return I18n.tr("Audio Output")
        case "battery": return I18n.tr("Battery")
        case "bluetooth": return I18n.tr("Bluetooth")
        case "clipboard": return I18n.tr("Clipboard")
        case "clock": return I18n.tr("Clock")
        case "color-picker": return I18n.tr("Color Picker")
        case "container": return I18n.tr("Container")
        case "divider": return I18n.tr("Divider")
        case "dock": return I18n.tr("Dock")
        case "launcher": return I18n.tr("Launcher")
        case "layout-switcher": return I18n.tr("Layout Switcher")
        case "lock": return I18n.tr("Lock")
        case "logout": return I18n.tr("Log Out")
        case "media": return I18n.tr("Media")
        case "music": return I18n.tr("Music")
        case "network": return I18n.tr("Network")
        case "notifications": return I18n.tr("Notifications")
        case "power-profile": return I18n.tr("Power Profile")
        case "quick-actions": return I18n.tr("Quick Actions")
        case "quick-settings": return I18n.tr("Quick Settings")
        case "reboot": return I18n.tr("Reboot")
        case "recording": return I18n.tr("Recording")
        case "screenshot": return I18n.tr("Screenshot")
        case "shutdown": return I18n.tr("Shut Down")
        case "spacer": return I18n.tr("Spacer")
        case "sysmon": return I18n.tr("System Monitor")
        case "theme": return I18n.tr("Theme")
        case "tray": return I18n.tr("Tray")
        case "vpn": return I18n.tr("VPN")
        case "wallpaper": return I18n.tr("Wallpaper")
        case "weather": return I18n.tr("Weather")
        case "workspaces": return I18n.tr("Workspaces")
        default: return I18n.tr("Unknown")
        }
    }

    function anchor(id) {
        switch (id) {
        case "bottom": return I18n.tr("Bottom")
        case "bottom-left": return I18n.tr("Bottom left")
        case "bottom-right": return I18n.tr("Bottom right")
        case "left": return I18n.tr("Left")
        case "right": return I18n.tr("Right")
        case "top": return I18n.tr("Top")
        case "top-left": return I18n.tr("Top left")
        case "top-right": return I18n.tr("Top right")
        default: return I18n.tr("Unknown")
        }
    }

    function edge(id) {
        switch (id) {
        case "bottom": return I18n.tr("Bottom")
        case "left": return I18n.tr("Left")
        case "right": return I18n.tr("Right")
        case "top": return I18n.tr("Top")
        default: return I18n.tr("Unknown")
        }
    }

    function zone(id) {
        switch (id) {
        case "bottom": return I18n.tr("Bottom")
        case "center": return I18n.tr("Center")
        case "end": return I18n.tr("End")
        case "start": return I18n.tr("Start")
        case "top": return I18n.tr("Top")
        default: return I18n.tr("Unknown")
        }
    }
}