import QtQuick

// The whole deck in one line, ported from kwepaperpicker's SpectrumRibbon:
// one segment per wallpaper in its scheme colour, favourites above, the
// current wallpaper below, and a bracket round the cards in hand.
Item {
    id: ribbon

    property real pos: 0
    property int span: 7
    property real u: 1
    property color accent: Theme.primary
    property color ink: Theme.surfaceText
    property var colors: []
    property var favourites: []
    property var thumbs: []
    property var names: []
    property int activeIndex: -1

    readonly property int count: colors.length
    readonly property real seg: count > 0 ? width / count : 0
    readonly property bool engaged: mouse.containsMouse || mouse.pressed

    signal scrub(real index)
    signal settle(int index)

    implicitHeight: 26 * u

    Item {
        id: track
        width: parent.width
        height: (ribbon.engaged ? 12 : 8) * ribbon.u
        anchors.verticalCenter: parent.verticalCenter

        Behavior on height {
            NumberAnimation { duration: 160; easing.type: Easing.OutCubic }
        }

        Repeater {
            model: ribbon.count

            Rectangle {
                required property int index
                readonly property string colorText: ribbon.colors[index] || ""

                x: index * ribbon.seg
                width: ribbon.seg + 0.6
                height: track.height
                color: colorText !== "" ? colorText : Theme.withAlpha(ribbon.ink, 0.14)
                topLeftRadius: index === 0 ? height / 2 : 0
                bottomLeftRadius: topLeftRadius
                topRightRadius: index === ribbon.count - 1 ? height / 2 : 0
                bottomRightRadius: topRightRadius
            }
        }
    }

    Repeater {
        model: ribbon.favourites

        Rectangle {
            required property int modelData
            x: (modelData + 0.5) * ribbon.seg - width / 2
            y: track.y - 5 * ribbon.u
            width: Math.max(2, Math.min(ribbon.seg - 1, 3 * ribbon.u))
            height: 3 * ribbon.u
            radius: width / 2
            color: Theme.primary
        }
    }

    Rectangle {
        visible: ribbon.activeIndex >= 0
        x: (ribbon.activeIndex + 0.5) * ribbon.seg - width / 2
        y: track.y + track.height + 3 * ribbon.u
        width: 4 * ribbon.u
        height: width
        radius: width / 2
        color: ribbon.ink
    }

    Rectangle {
        readonly property real from: Math.max(0, ribbon.pos - ribbon.span)
        readonly property real to: Math.min(ribbon.count, ribbon.pos + ribbon.span + 1)
        visible: ribbon.count > 0
        x: from * ribbon.seg - 3 * ribbon.u
        width: Math.max(6 * ribbon.u, (to - from) * ribbon.seg + 6 * ribbon.u)
        y: track.y - 4 * ribbon.u
        height: track.height + 8 * ribbon.u
        radius: Theme.radius
        color: Theme.withAlpha(Theme.background, 0)
        border.width: Math.max(1, 1.3 * ribbon.u)
        border.color: Theme.withAlpha(ribbon.ink, 0.55)
    }

    Rectangle {
        visible: ribbon.count > 0
        x: (ribbon.pos + 0.5) * ribbon.seg - width / 2
        y: track.y - 6 * ribbon.u
        width: Math.max(3 * ribbon.u, Math.min(ribbon.seg, 6 * ribbon.u))
        height: track.height + 12 * ribbon.u
        radius: width / 2
        color: ribbon.accent
        border.width: 1
        border.color: Theme.withAlpha(Theme.background, 0.35)
    }

    MouseArea {
        id: mouse
        readonly property int at: ribbon.count > 0
            ? Math.max(0, Math.min(ribbon.count - 1, Math.floor(mouseX / ribbon.seg)))
            : -1

        anchors.fill: parent
        anchors.topMargin: -6 * ribbon.u
        anchors.bottomMargin: -6 * ribbon.u
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        preventStealing: true

        onPressed: mouse => ribbon.scrub(Math.max(0, Math.min(ribbon.count - 1, mouse.x / ribbon.seg - 0.5)))
        onPositionChanged: mouse => {
            if (pressed)
                ribbon.scrub(Math.max(0, Math.min(ribbon.count - 1, mouse.x / ribbon.seg - 0.5)))
        }
        onReleased: ribbon.settle(at)
    }

    Rectangle {
        id: glimpse
        readonly property int index: mouse.at
        property string shown: ""

        visible: mouse.containsMouse && !mouse.pressed && index >= 0
        x: Math.max(0, Math.min(ribbon.width - width, mouse.mouseX - width / 2))
        y: ribbon.height + 8 * ribbon.u
        width: 176 * ribbon.u
        height: glimpseCol.implicitHeight + 12 * ribbon.u
        radius: Theme.radius * 2
        color: Theme.surfaceContainer
        border.width: 1
        border.color: Theme.withAlpha(ribbon.ink, 0.16)

        onIndexChanged: glimpseTimer.restart()
        onVisibleChanged: if (visible) glimpseTimer.restart()

        Timer {
            id: glimpseTimer
            interval: 70
            onTriggered: glimpse.shown = glimpse.index >= 0 ? (ribbon.thumbs[glimpse.index] || "") : ""
        }

        Column {
            id: glimpseCol
            x: 6 * ribbon.u
            y: 6 * ribbon.u
            width: parent.width - 12 * ribbon.u
            spacing: 5 * ribbon.u

            Rectangle {
                width: parent.width
                height: Math.round(width * 10 / 16)
                radius: Theme.radius
                color: Theme.surfaceVariant
                clip: true

                Image {
                    anchors.fill: parent
                    source: glimpse.shown
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    smooth: true
                }
            }

            Text {
                width: parent.width
                text: ribbon.names[glimpse.index] || ""
                elide: Text.ElideRight
                font.family: Theme.sans
                font.pixelSize: 10 * ribbon.u
                color: ribbon.ink
            }
        }
    }
}
