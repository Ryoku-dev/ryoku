pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    required property var panel
    required property var catalog
    required property var card
    property real s: 1
    readonly property real lithicScale: s
    readonly property string result: String(panel.resultText || "")
    readonly property bool speaking: String(panel.speakingId || "") === String(card.id || "") && String(card.id || "").length > 0
    readonly property bool canReplace: catalog.feature("inplace") && card.source === "highlight" && currentAction !== null && currentAction.replaces === true
    readonly property bool hasActions: canReplace || catalog.feature("readaloud") || catalog.feature("save")
    readonly property bool ready: result.trim().length > 0 && !panel.busy
    readonly property var currentAction: catalog.action(panel.action)
    implicitWidth: actions.implicitWidth
    implicitHeight: actions.childrenRect.height
    visible: result.trim().length > 0 && hasActions

    Flow {
        id: actions
        width: root.width
        spacing: 6 * root.s
        TheiaChip {
            s: root.s
            small: true
            visible: root.canReplace
            text: I18n.tr("Replace")
            icon: "find_replace"
            enabled: root.ready
            onClicked: root.panel.replaceWith(root.result)
        }
        TheiaChip {
            s: root.s
            small: true
            visible: root.catalog.feature("readaloud")
            text: root.speaking ? I18n.tr("Stop reading") : I18n.tr("Read aloud")
            icon: root.speaking ? "stop" : "volume_up"
            selected: root.speaking
            enabled: root.speaking || root.ready
            onClicked: {
                if (root.speaking) root.panel.stopSpeaking();
                else root.panel.speak(root.result);
            }
        }
        TheiaChip {
            s: root.s
            small: true
            visible: root.catalog.feature("save")
            text: I18n.tr("Save")
            icon: "bookmark_add"
            enabled: root.ready
            onClicked: root.panel.save(root.result, root.currentAction ? root.currentAction.label : I18n.tr("Theia"))
        }
    }
}
