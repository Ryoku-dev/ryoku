import "../" as Services
import QtQuick

QtObject {
    readonly property var wallpaper: Services.Wallpapers
    readonly property var colors: Services.Colours
    readonly property var nightLight: Services.HyprSunset
    readonly property var brightness: Services.Brightness
}
