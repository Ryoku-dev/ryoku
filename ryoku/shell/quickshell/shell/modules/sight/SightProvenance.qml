pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    required property var panel
    property real s: 1
    property string model: ""
    property bool localModel: false
    property bool localOnly: false
    property bool copied: false
    readonly property bool popupOpen: details.visible
    readonly property string shortModel: model.split("/").pop().replace(/[_-]/g, " ").replace(/\b(\d+) (\d+)\b/g, "$1.$2").replace(/\b[a-z]/g, function(letter) { return letter.toUpperCase(); })
    implicitHeight: Math.max(provenance.implicitHeight, LithicTokens.space.s4 * s)
    function close() { details.close(); }
    T.AbstractButton {
        id: source
        width: Math.min(parent.width, provenance.implicitWidth); height: root.height
        hoverEnabled: true; focusPolicy: Qt.StrongFocus
        Accessible.role: Accessible.Button
        Accessible.name: root.copied ? I18n.tr("Copied") : I18n.tr("Answered by %1").arg(root.shortModel)
        enabled: root.model.length > 0
        onClicked: if (details.visible) details.close(); else { root.panel.preparePopup("provenance"); details.open(); }
        scale: down ? 0.98 : 1
        Behavior on scale { NumberAnimation { duration: Lithic.duration(90) } }
        background: Item {
            CutRect {
                anchors.fill: parent; cut: 3; fill: Lithic.alpha(Lithic.fg, 0.06)
                opacity: source.hovered ? 1 : 0
                Behavior on opacity { NumberAnimation { duration: Lithic.duration(source.hovered ? 160 : 100) } }
            }
            FocusRing { anchors.fill: parent; cut: 3; keyboardFocusVisible: source.visualFocus }
        }
        contentItem: Item {
            id: provenance
            implicitWidth: modelLabel.implicitWidth + 21 * root.s
            implicitHeight: Math.max(modelLabel.implicitHeight, modelIcon.implicitHeight)
            Glyph { id: modelIcon; anchors.verticalCenter: parent.verticalCenter; name: root.copied ? "check" : root.localModel ? "computer" : "cloud"; size: 15; ink: Lithic.fg3 }
            Label { id: modelLabel; x: 21 * root.s; width: Math.max(0, parent.width - x); anchors.verticalCenter: parent.verticalCenter; text: root.copied ? I18n.tr("Copied") : root.shortModel; typeRole: "caption"; inkRole: "fg3" }
        }
    }
    Popover {
        id: details
        parent: root; anchorItem: source; opener: source; menuFocus: false
        closePolicy: T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutsideParent
        implicitWidth: Math.min(300 * root.s, Math.max(1, root.panel.usable.width
                                   - LithicTokens.size.popoverAnchorGap * 2 * root.s))
        implicitHeight: info.implicitHeight + 26 * root.s
        function calculatePlacement() {
            return root.panel.popupPlacement(source, implicitWidth, implicitHeight, root);
        }
        background: CutRect { cut: 8; fill: Lithic.stone4; strokeColor: Lithic.vein; strokeWidth: 1; transform: Translate { y: details.slideOffset } }
        padding: 13 * root.s
        contentItem: Column {
            id: info
            transform: Translate { y: details.slideOffset }
            clip: true
            width: details.availableWidth
            readonly property real lithicScale: root.s
            spacing: 8 * root.s
            Label { width: parent.width; typeRole: "detail"; inkRole: "fg2"; text: I18n.tr("Model: %1").arg(root.model); wrapMode: Text.Wrap; elide: Text.ElideNone }
            Label { width: parent.width; typeRole: "detail"; inkRole: "fg2"; text: root.localOnly ? I18n.tr("Local models only: On") : I18n.tr("Local models only: Off") }
            SightActionButton { s: root.s; text: I18n.tr("Change in Settings"); onClicked: { details.close(); root.panel.settings(false); } }
        }
    }
}
