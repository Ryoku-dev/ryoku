import QtQuick
import Ryoku.Ui.Singletons
import "../kit" as K

// Stops a window's program. A first press arms it in the alert colour; the
// engine kills only on a second press within three seconds.
Item {
    id: root

    property bool armed: false

    signal clicked()

    implicitWidth: label.implicitWidth + glyph.width + K.Style.s1 + K.Style.s2 * 2
    implicitHeight: 22
    scale: press.pressed && K.Look.microMotion && !K.Style.still ? K.Look.pressScale : 1

    Behavior on scale {
        enabled: K.Look.microMotion && !K.Style.still
        NumberAnimation { duration: K.Style.snap }
    }

    K.Shape {
        anchors.fill: parent
        cutSize: K.Look.cutToken
        fill: root.armed ? K.Style.alert
            : press.hovered ? K.Style.alpha(K.Style.alert, 0.14) : "transparent"
        strokeWidth: root.armed ? 0 : K.Style.border
        strokeColor: K.Style.alpha(K.Style.alert, 0.55)

        Behavior on fill {
            enabled: K.Look.microMotion && !K.Style.still
            ColorAnimation { duration: K.Look.stateMs }
        }
    }

    K.Glyph {
        id: glyph
        anchors.left: parent.left
        anchors.leftMargin: K.Style.s2
        anchors.verticalCenter: parent.verticalCenter
        name: "dangerous"
        size: 14
        color: root.armed ? K.Style.base : K.Style.alert
    }

    K.Label {
        id: label
        anchors.left: glyph.right
        anchors.leftMargin: K.Style.s1
        anchors.verticalCenter: parent.verticalCenter
        text: root.armed ? I18n.tr("Kill?") : I18n.tr("Kill")
        role: "caption"
        color: root.armed ? K.Style.base : K.Style.alert
    }

    K.Press {
        id: press
        tint: false
        onClicked: root.clicked()
    }
}
