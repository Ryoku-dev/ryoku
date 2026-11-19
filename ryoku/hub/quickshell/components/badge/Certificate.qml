import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../about/design"
import ".."

// A popup, not a plain item: View certificate is pressed inside the card studio, itself a
// dialog, and dialogs are drawn in the window's popup layer above every item, so an item would
// open behind the studio. It covers its parent, Settings' overlay layer, and closes on Escape,
// a click on the shade or its close button.
T.Popup {
    id: root
    property var badge: null
    property var spec: null
    readonly property var entry: root.badge && root.badge.badge ? root.badge.badge : root.badge
    readonly property var emblemSpec: root.spec && root.spec.badge ? root.spec.badge : root.spec
    readonly property string role: root.entry ? String(root.entry.role || "tester") : "tester"
    readonly property string program: root.entry && root.entry.program ? String(root.entry.program) : ""
    readonly property bool verified: !!root.entry && root.entry.verified !== false && root.entry.badgeCheck !== "offline"
    readonly property bool moving: root.opened && !Lithic.reduceMotion
    property real elapsed: 0

    x: 0
    y: 0
    width: parent ? parent.width : 0
    height: parent ? parent.height : 0
    padding: 0
    modal: true
    dim: false
    focus: true
    closePolicy: T.Popup.CloseOnEscape

    enter: Transition {
        NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Lithic.reduceMotion ? 0 : Tone.move; easing.type: Easing.OutCubic }
    }
    exit: Transition {
        NumberAnimation { property: "opacity"; from: 1; to: 0; duration: Lithic.reduceMotion ? 0 : Tone.move; easing.type: Easing.OutCubic }
    }

    contentItem: Item {
        NumberAnimation { target: root; property: "elapsed"; from: 0; to: 3600; duration: 3600000; running: root.moving; loops: Animation.Infinite }

        Rectangle {
            anchors.fill: parent
            color: Lithic.alpha(Lithic.ground, 0.97)
            MouseArea { anchors.fill: parent; onClicked: root.close() }
        }
        Item {
            id: page
            readonly property real factor: Math.max(0.1, Math.min((root.width - 64) / 1080, (root.height - 64) / 690))
            width: 1080; height: 690
            anchors.centerIn: parent
            scale: page.factor
            CutRect { anchors.fill: parent; cut: 32; fill: Lithic.stone1; strokeWidth: 1; strokeColor: Lithic.edge }
            DesignEffects {
                anchors.fill: parent
                effects: [{id: "hexgrid", strength: 0.5, speed: 0.3}]
                accent: seal.accent
                time: root.moving ? root.elapsed : 0
                opacity: 0.35
            }
            MouseArea { anchors.fill: parent }
            Rectangle { x: 66; y: 96; width: 948; height: 1; color: Lithic.edge }
            Row {
                x: 66; y: 51; spacing: 16
                RyokuMark { width: 27; height: 26; ink: Lithic.fg; accent: seal.accent }
                Label { text: "RYOKU"; typeRole: "mono"; font.pixelSize: 20; font.letterSpacing: 6 }
            }
            Label {
                x: 682; y: 58; width: 330
                horizontalAlignment: Text.AlignRight
                text: I18n.tr("Certificate of membership")
                typeRole: "monoSm"; font.pixelSize: 12; inkRole: "fg2"
            }
            Emblem {
                id: seal
                x: 44; y: 155; size: 400
                role: root.role
                spec: root.emblemSpec
                live: root.moving
            }
            Column {
                x: 478; y: 152; width: 530; spacing: 20
                Label {
                    width: parent.width
                    text: "Ryoku " + root.role.charAt(0).toUpperCase() + root.role.slice(1)
                    typeRole: "name"; font.pixelSize: 45; font.weight: Font.Black
                    wrapMode: Text.WordWrap
                }
                Label {
                    width: parent.width
                    text: root.entry ? "@" + root.entry.login : ""
                    typeRole: "title"; font.pixelSize: 28; ink: seal.accent
                    elide: Text.ElideRight
                }
                Label {
                    width: parent.width - 32
                    text: ({
                        founder: I18n.tr("Started Ryoku and still leads it."),
                        core: I18n.tr("Builds and maintains Ryoku with the core team."),
                        nixos: I18n.tr("Keeps Ryoku running on NixOS."),
                        contributor: I18n.tr("Has work in Ryoku's code."),
                        tester: I18n.tr("Tests Ryoku before anyone else and reports what breaks.")
                    })[root.entry ? root.entry.role : ""] || I18n.tr("Part of the people who build Ryoku.")
                    typeRole: "body"; font.pixelSize: 17; inkRole: "fg2"
                    wrapMode: Text.WordWrap
                }
            }
            Row {
                x: 478; y: 390; spacing: root.program !== "" ? 44 : 60
                Column {
                    spacing: 8
                    Label { text: I18n.tr("Issued by"); typeRole: "monoSm"; font.pixelSize: 12; inkRole: "fg3" }
                    Label { text: "neur0map"; typeRole: "label"; font.pixelSize: 20 }
                }
                Column {
                    spacing: 8
                    Label { text: I18n.tr("Issued on"); typeRole: "monoSm"; font.pixelSize: 12; inkRole: "fg3" }
                    Label { text: root.entry ? String(root.entry.issued || "") : ""; typeRole: "mono"; font.pixelSize: 18 }
                }
                Column {
                    visible: root.program !== ""
                    width: Math.max(0, 530 - x)
                    spacing: 8
                    Label { text: I18n.tr("Program"); typeRole: "monoSm"; font.pixelSize: 12; inkRole: "fg3" }
                    // a long program takes a second, smaller line before the verification strip
                    Label {
                        width: parent.width
                        text: root.program
                        typeRole: "label"
                        font.pixelSize: programFit.advanceWidth <= width ? 20 : 16
                        lineHeight: font.pixelSize + 4
                        wrapMode: Text.Wrap
                        maximumLineCount: 2
                        TextMetrics { id: programFit; font.family: Lithic.type.label.family; font.weight: Lithic.type.label.weight; font.pixelSize: 20; text: root.program }
                    }
                }
            }
            CutRect {
                x: 478; y: 493; width: 530; height: 48; cut: 10
                fill: Lithic.alpha(root.verified ? Lithic.jade : Lithic.gold, 0.12)
                Row {
                    anchors.verticalCenter: parent.verticalCenter; x: 16; spacing: 12
                    Glyph { name: root.verified ? "verified" : "pending"; size: 21; ink: root.verified ? Lithic.jade : Lithic.gold }
                    Label { text: root.verified ? I18n.tr("Signature verified") : I18n.tr("Verification unavailable"); typeRole: "label"; font.pixelSize: 15 }
                }
            }
            Rectangle { x: 66; y: 590; width: 948; height: 1; color: Lithic.edge }
            Column {
                x: 66; y: 612; spacing: 7
                Label { text: I18n.tr("Certificate number"); typeRole: "monoSm"; font.pixelSize: 10; inkRole: "fg3" }
                Label { text: root.entry ? String(root.entry.id || "").toUpperCase() : ""; typeRole: "mono"; font.pixelSize: 14; font.letterSpacing: 2 }
            }
            Label {
                x: 680; y: 631; width: 330; horizontalAlignment: Text.AlignRight
                // every badge is issued by neur0map; Mika signs tester and contributor badges for him
                text: root.entry && root.entry.issuer === "mika" ? "ED25519  /  SIGNED BY MIKA" : "ED25519  /  SIGNED BY NEUR0MAP"
                typeRole: "monoSm"; font.pixelSize: 11; inkRole: "fg3"; font.letterSpacing: 1
            }
        }
        IconButton {
            anchors.right: parent.right; anchors.top: parent.top; anchors.margins: 16
            glyph: "close"; accessibleName: I18n.tr("Close certificate")
            onClicked: root.close()
        }
    }
}
