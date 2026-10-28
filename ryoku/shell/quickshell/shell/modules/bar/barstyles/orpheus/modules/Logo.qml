import QtQuick
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: true

    readonly property real markSize: 18 * K.Style.scale
    readonly property real sealSize: 28 * K.Style.scale

    // Left-click opens what logoClick names (Orpheus settings or the Ryoku
    // Hub); middle-click opens the other, so neither is ever out of reach.
    function activate(target) {
        if (target === "hub")
            root.scene.openHub("");
        else
            root.scene.openSettings("");
    }

    implicitWidth: sealSize + 2 * K.Style.hitPad
    implicitHeight: K.Style.barH
    height: K.Style.barH

    Item {
        id: seal
        anchors.centerIn: parent
        width: root.sealSize
        height: width
        scale: press.pressScale

        K.Press {
            id: press
            radius: K.Style.radius
            onClicked: root.activate(S.Prefs.logoClick)
            onRightClicked: root.scene.toggleLauncher()
            onMiddleClicked: root.activate(S.Prefs.logoClick === "hub" ? "settings" : "hub")
        }

        K.LogoMark {
            anchors.centerIn: parent
            mark: S.Prefs.logoMark === "custom" ? S.Prefs.logoIcon : "system"
            size: root.markSize
            color: K.Style.accent
            weight: Font.Black
        }
    }


    K.Hint {
        target: root
        text: S.Prefs.logoClick === "hub" ? "Ryoku Hub" : "Orpheus settings"
        shown: press.hovered
    }
}
