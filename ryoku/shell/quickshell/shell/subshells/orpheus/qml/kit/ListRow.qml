import QtQuick
import Quickshell.Widgets

Item {
    id: root

    property string icon: ""
    property url image: ""
    property string title: ""
    property string subtitle: ""
    property bool selected: false
    property bool divider: false
    default property alias trailing: trailingRow.data

    signal clicked()
    signal rightClicked()

    readonly property bool hasImage: String(image).length > 0
    readonly property bool hasLeading: hasImage || icon.length > 0
    readonly property real leadingSpan: hasLeading ? 32 + Style.s3 : 0
    readonly property real trailingSpan: trailingRow.implicitWidth > 0
        ? trailingRow.implicitWidth + Style.s3 : 0

    implicitWidth: Style.s3 + leadingSpan
        + Math.max(titleLabel.implicitWidth, subtitleLabel.implicitWidth)
        + trailingSpan + Style.s3
    implicitHeight: Math.max(Style.rowH, subtitle.length > 0 ? 52 : Style.rowH)
    opacity: enabled ? 1 : 0.38
    scale: rowPress.pressScale

    Rectangle {
        visible: root.divider
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: Style.border
        color: Style.lineSoft
    }

    Rectangle {
        anchors.fill: parent
        anchors.topMargin: root.divider ? Style.border : 0
        radius: Style.plateRadius
        color: root.selected ? Style.bone
            : rowPress.pressed ? Style.tint16
            : rowPress.hovered ? Style.tint5 : "transparent"

        Behavior on color {
            enabled: !Style.still
            ColorAnimation { duration: Style.snap }
        }
    }

    ClippingRectangle {
        id: leading
        visible: root.hasLeading
        anchors.left: parent.left
        anchors.leftMargin: Style.s3
        anchors.verticalCenter: parent.verticalCenter
        width: 32
        height: 32
        radius: root.hasImage ? Style.radius : 0
        color: "transparent"

        Image {
            id: leadingImage
            anchors.fill: parent
            source: root.image
            sourceSize.width: Math.max(1, Math.round(width * Screen.devicePixelRatio * 2))
            sourceSize.height: Math.max(1, Math.round(height * Screen.devicePixelRatio * 2))
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            smooth: true
            mipmap: true
            visible: root.hasImage && status === Image.Ready
        }

        Glyph {
            anchors.centerIn: parent
            visible: !leadingImage.visible
            name: root.icon
            size: Style.icon
            tone: root.selected ? "onBone" : "dim"
        }
    }

    Row {
        id: trailingRow
        anchors.right: parent.right
        anchors.rightMargin: Style.s3
        anchors.verticalCenter: parent.verticalCenter
        spacing: Style.s2
    }

    Column {
        anchors.left: root.hasLeading ? leading.right : parent.left
        anchors.leftMargin: Style.s3
        anchors.right: trailingRow.left
        anchors.rightMargin: trailingRow.implicitWidth > 0 ? Style.s3 : 0
        anchors.verticalCenter: parent.verticalCenter
        spacing: 2

        Label {
            id: titleLabel
            width: parent.width
            text: root.title
            role: "bodyStrong"
            tone: root.selected ? "onBone" : "ink"
        }

        Label {
            id: subtitleLabel
            visible: root.subtitle.length > 0
            width: parent.width
            text: root.subtitle
            role: "caption"
            tone: root.selected ? "onBone" : "muted"
            opacity: root.selected ? 0.78 : 1
        }
    }

    Press {
        id: rowPress
        enabled: root.enabled
        tint: false
        radius: Style.plateRadius
        onClicked: root.clicked()
        onRightClicked: root.rightClicked()
    }
}
