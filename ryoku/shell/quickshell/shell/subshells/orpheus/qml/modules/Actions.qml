pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    required property string lane

    readonly property var actionIds: {
        const configured = S.Prefs.actions || [];
        const known = ["launcher", "ask", "overview", "clipboard",
            "capture", "wallpaper", "hub"];
        const result = [];
        for (let i = 0; i < configured.length; ++i) {
            const id = String(configured[i]);
            if (known.indexOf(id) >= 0 && result.indexOf(id) < 0)
                result.push(id);
        }
        return result;
    }

    property bool present: actionIds.length > 0

    implicitWidth: present ? actions.implicitWidth + 2 * K.Style.hitPad : 0
    implicitHeight: K.Style.barH
    height: K.Style.barH

    function iconFor(id) {
        switch (id) {
        case "launcher": return "search";
        case "ask": return "auto_awesome";
        case "overview": return "grid_view";
        case "clipboard": return "content_paste";
        case "capture": return "screenshot_region";
        case "wallpaper": return "wallpaper";
        case "hub": return "settings";
        default: return "bolt";
        }
    }

    function hintFor(id) {
        switch (id) {
        case "launcher": return "App launcher";
        case "ask": return "Ask Ryoku";
        case "overview": return "Window overview";
        case "clipboard": return "Clipboard history";
        case "capture": return "Screen capture";
        case "wallpaper": return "Wallpaper picker";
        case "hub": return "Ryoku Hub";
        default: return "Shortcut";
        }
    }

    function run(id) {
        switch (id) {
        case "launcher":
            root.scene.toggleLauncher();
            break;
        case "ask":
            root.scene.openAsk();
            break;
        case "overview":
            root.scene.toggleOverview();
            break;
        case "clipboard":
            root.scene.toggleClipboard();
            break;
        case "capture":
            Spawn.run(["ryoku-cmd-ryoshot"]);
            break;
        case "wallpaper":
            Quickshell.execDetached(["ryogami", "wallpaper", "ui"]);
            break;
        case "hub":
            root.scene.openHub("");
            break;
        }
    }

    Row {
        id: actions
        anchors.centerIn: parent
        spacing: 0

        Repeater {
            model: root.actionIds

            delegate: Item {
                id: actionItem

                required property var modelData

                width: Math.round(28 * K.Style.scale)
                height: K.Style.barH

                K.Glyph {
                    anchors.centerIn: parent
                    scale: actionPress.pressScale
                    name: root.iconFor(String(actionItem.modelData))
                    size: K.Style.barIcon
                    tone: "dim"
                }

                K.Press {
                    id: actionPress
                    radius: K.Style.radius
                    onClicked: root.run(String(actionItem.modelData))
                    onRightClicked: root.run(String(actionItem.modelData))
                }

                K.Hint {
                    target: actionItem
                    text: root.hintFor(String(actionItem.modelData))
                    shown: actionPress.hovered
                }
            }
        }
    }
}