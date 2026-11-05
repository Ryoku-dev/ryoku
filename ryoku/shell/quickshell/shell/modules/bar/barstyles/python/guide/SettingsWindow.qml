pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import ".."
import "."

Scope {
    id: root

    property bool open: false

    function show() { root.open = true; }
    function hide() { root.open = false; }
    function toggle() { root.open = !root.open; }

    Loader {
        active: root.open
        sourceComponent: FloatingWindow {
            title: I18n.t("widgets.guide.name", "Settings")
            color: "transparent"
            implicitWidth: Math.round(1200 * Scaler.uiScale)
            implicitHeight: Math.round(750 * Scaler.uiScale)
            minimumSize: Qt.size(Math.round(480 * Scaler.uiScale), Math.round(400 * Scaler.uiScale))
            visible: true
            onVisibleChanged: if (!visible) root.hide()

            GuidePopup {
                anchors.fill: parent
                standalone: true
                visible: true
                onExitRequested: root.hide()
            }
        }
    }
}
