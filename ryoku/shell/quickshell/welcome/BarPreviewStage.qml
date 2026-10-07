import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

Rectangle {
    id: root

    property string styleId: "qsbar"
    property string styleName: ""
    property string renderedStyle: styleId

    height: 192
    radius: Tokens.radius
    color: Tokens.paperLift
    border.width: Tokens.border
    border.color: Tokens.line

    function syncStyle() {
        if (Motion.reduce) {
            renderedStyle = styleId
            preview.opacity = 1
        } else {
            swap.restart()
        }
    }

    onStyleIdChanged: syncStyle()
    Component.onCompleted: renderedStyle = styleId

    Text {
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.margins: Tokens.s3
        text: I18n.tr("LIVE BAR")
        color: Tokens.inkFaint
        font.family: Tokens.mono
        font.pixelSize: Tokens.fTiny
        font.letterSpacing: Tokens.trackLabel
    }

    Text {
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: Tokens.s3
        text: root.styleName
        color: Tokens.ink
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
        font.weight: Font.DemiBold
    }

    Rectangle {
        id: preview
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.leftMargin: Tokens.s5
        anchors.rightMargin: Tokens.s5
        anchors.topMargin: Tokens.s6
        anchors.bottomMargin: Tokens.s4
        radius: Tokens.radius
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        clip: true

        Rectangle {
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.leftMargin: parent.width * 0.19
            anchors.topMargin: parent.height * 0.34
            width: parent.width * 0.27
            height: parent.height * 0.42
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
        }

        Rectangle {
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.rightMargin: parent.width * 0.16
            anchors.topMargin: parent.height * 0.26
            width: parent.width * 0.23
            height: parent.height * 0.5
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.lineSoft
        }

        BarSilhouette {
            anchors.fill: parent
            anchors.margins: Tokens.s3
            styleId: root.renderedStyle
        }

        Ticks { anchors.margins: Tokens.s2 }
    }

    SequentialAnimation {
        id: swap
        NumberAnimation {
            target: preview
            property: "opacity"
            to: 0.15
            duration: Motion.snap
            easing.type: Tokens.easeSnap
        }
        ScriptAction { script: root.renderedStyle = root.styleId }
        NumberAnimation {
            target: preview
            property: "opacity"
            to: 1
            duration: Motion.swap
            easing.type: Tokens.ease
        }
    }
}
