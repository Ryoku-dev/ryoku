pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import shell.services

// the board heads this block so it draws no frame or title of its own
Item {
    id: root

    required property real s
    required property bool active

    readonly property var labels: ({
        "power-saver": I18n.tr("Power saver"),
        "balanced": I18n.tr("Balanced"),
        "performance": I18n.tr("Performance")
    })
    readonly property var glyphs: ({
        "power-saver": "eco",
        "balanced": "balance",
        "performance": "speed"
    })
    readonly property var proofs: ({
        "power-saver": I18n.tr("Saves battery, slower when busy"),
        "balanced": I18n.tr("Steady speed and battery life"),
        "performance": I18n.tr("Full speed, uses more power")
    })
    readonly property real proofGap: Math.round(8 * root.s)

    implicitHeight: PowerProfiles.available
        ? segments.implicitHeight + root.proofGap + proof.implicitHeight
        : unavailable.implicitHeight
    Accessible.role: Accessible.Grouping
    Accessible.name: I18n.tr("Power mode")

    onActiveChanged: PowerProfiles.setActive(root, root.active)
    Component.onCompleted: PowerProfiles.setActive(root, root.active)
    Component.onDestruction: PowerProfiles.setActive(root, false)

    SidebarSegments {
        id: segments
        visible: PowerProfiles.available
        anchors.left: parent.left
        anchors.right: parent.right
        s: root.s
        options: PowerProfiles.profiles
        labels: root.labels
        glyphs: root.glyphs
        current: PowerProfiles.profile
        accessibleName: I18n.tr("Power mode")
        onChose: key => PowerProfiles.setProfile(key)
    }

    Text {
        id: proof
        visible: PowerProfiles.available
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: segments.bottom
        anchors.topMargin: root.proofGap
        text: root.proofs[PowerProfiles.profile] !== undefined ? root.proofs[PowerProfiles.profile] : ""
        color: Lithic.fg3
        font.family: QsLook.sans
        font.pixelSize: Math.round(12 * root.s)
        renderType: Text.NativeRendering
        elide: Text.ElideRight
        onTextChanged: if (root.active) settle.restart()

        NumberAnimation {
            id: settle
            target: proof
            property: "opacity"
            from: 0.25
            to: 1
            duration: QsLook.dur(QsLook.stateMs * 2)
            easing.type: Easing.OutCubic
        }
    }

    Text {
        id: unavailable
        visible: !PowerProfiles.available
        anchors.left: parent.left
        anchors.right: parent.right
        text: I18n.tr("Power profiles are not available on this computer")
        color: Lithic.fg3
        font.family: QsLook.sans
        font.pixelSize: Math.round(12 * root.s)
        renderType: Text.NativeRendering
        elide: Text.ElideRight
    }
}
