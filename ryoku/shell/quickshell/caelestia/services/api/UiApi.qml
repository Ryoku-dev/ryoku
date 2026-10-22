import "../" as Services
import QtQuick
import Caelestia as Backend

QtObject {
    readonly property var toast: Backend.Toast
    readonly property var clipboard: Backend.ClipboardManager
    readonly property var emojis: Backend.EmojiDb
    readonly property var keyboard: Services.KbLayout

    readonly property ListModel topBarLeft: ListModel {}
    readonly property ListModel topBarMiddle: ListModel {}
    readonly property ListModel topBarRight: ListModel {}
    readonly property ListModel launcherWidgets: ListModel {}
}
