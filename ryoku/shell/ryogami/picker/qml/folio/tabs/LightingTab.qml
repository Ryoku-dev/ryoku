import QtQuick
import Ryoku.Ui.Singletons

FolioTabData {
    tabKey: "lighting"
    title: I18n.tr("Lighting")
    note: I18n.tr("Recolour keyboards, mice, headsets, RAM, motherboards and laptop keyboards from your palette through OpenRGB, asusd for ASUS laptops, and qmk_hid for QMK and VIA boards.")
    sections: [
        {
            key: "overview",
            title: I18n.tr("RGB lighting"),
            subtitle: I18n.tr("Choose when Ryoku may reach lighting hardware, see the live accent, and detect connected devices."),
            controls: [
                { id: "lighting.overview",
                  key: null,
                  kind: "lighting",
                  part: "overview",
                  label: I18n.tr("RGB lighting"),
                  help: I18n.tr("Follow the wallpaper and theme palette on supported lighting hardware."),
                  search: ["lighting", "rgb", "openrgb", "keyboard", "mouse", "headset", "led", "aura", "asusd", "qmk", "via", "palette", "matugen", "detect", "scan", "enable"] }
            ]
        },
        {
            key: "devices",
            title: I18n.tr("Devices"),
            subtitle: I18n.tr("Adopt only the hardware Ryoku should control, then choose its effect, colours and live behaviour."),
            controls: [
                { id: "lighting.devices",
                  key: null,
                  kind: "lighting",
                  part: "devices",
                  label: I18n.tr("Lighting devices"),
                  help: I18n.tr("Connected and saved devices, including devices that are currently unplugged."),
                  search: ["lighting", "rgb", "openrgb", "keyboard", "mouse", "headset", "ram", "motherboard", "led", "aura", "asusd", "qmk", "via", "palette", "device", "effect", "mode", "brightness", "speed", "direction", "colour", "zone", "save", "offline"] }
            ]
        }
    ]
}
