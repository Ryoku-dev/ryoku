import QtQuick
import QtQuick.Effects

// One wallpaper as a paint-chip card in the deck, ported from
// kwepaperpicker's SwatchCard.qml. The card is the wallpaper's surface, the
// text its on-surface, and the five chips are the roles its scheme would give
// the desktop.
Item {
    id: card

    property var entry: null
    property string picture: entry ? (entry.fullImage || entry.thumb || entry.path || "") : ""
    property var scheme: null
    property real u: 1
    property real front: 0
    property real dim: 0
    property bool current: false
    property bool favourite: false
    property bool saved: false
    property bool showSwatches: true
    property bool showShadow: true
    property string title: ""
    property string meta: ""

    readonly property alias hovered: mouse.containsMouse
    signal clicked
    signal doubleClicked
    signal wheeled(real angleY, real pixelY)
    signal dragged(real dx, real dy, bool released)

    readonly property real pad: 9 * u
    readonly property real radius: 16 * u
    readonly property real thumbRadius: 10 * u
    readonly property real chipHeight: 22 * u

    function mix(a, b, amount) {
        const ca = Qt.color(a)
        const cb = Qt.color(b)
        const t = Math.max(0, Math.min(1, amount))
        return Qt.rgba(ca.r * (1 - t) + cb.r * t,
                      ca.g * (1 - t) + cb.g * t,
                      ca.b * (1 - t) + cb.b * t,
                      ca.a * (1 - t) + cb.a * t)
    }

    function withAlpha(color, alpha) {
        const q = Qt.color(color)
        return Qt.rgba(q.r, q.g, q.b, Math.max(0, Math.min(1, alpha)) * q.a)
    }

    function inkOn(color) {
        const q = Qt.color(color)
        const l = 0.2126 * q.r + 0.7152 * q.g + 0.0722 * q.b
        return l > 0.5 ? Qt.rgba(0, 0, 0, 0.78) : Qt.rgba(1, 1, 1, 0.9)
    }

    function pick(s, a, b, fallback) {
        return s[a] || s[b] || fallback
    }

    readonly property var dress: scheme ? DeckPalettes.dressOf(scheme) : DeckPalettes.dressFrom([])

    function placeholder(index) {
        return {
            label: "",
            fill: mix(dress.raised, "#ffffff", 0.02 + 0.025 * (4 - index))
        }
    }

    readonly property var chips: {
        if (!scheme || !card.showSwatches)
            return [0, 1, 2, 3, 4].map(i => card.placeholder(i))
        const f = card.dress
        return [
            { label: "primary", fill: pick(scheme, "primary", "primary", f.accent) },
            { label: "secondary", fill: pick(scheme, "secondary", "secondary", f.accent) },
            { label: "tertiary", fill: pick(scheme, "tertiary", "tertiary", f.accent) },
            { label: "container", fill: pick(scheme, "primaryContainer", "primary_container", f.raised) },
            { label: "surface", fill: pick(scheme, "surfaceContainerHighest", "surface_container_highest", f.raised) }
        ]
    }

    implicitWidth: 212 * u
    implicitHeight: pad + thumbBase.height + 8 * u + chipHeight * 5 + 9 * u
        + titleText.height + 2 * u + metaText.height + pad + 2 * u

    RectangularShadow {
        anchors.fill: body
        visible: card.showShadow
        offset.y: (8 + 10 * card.front) * card.u
        blur: (22 + 18 * card.front) * card.u
        radius: body.radius
        color: Qt.rgba(0, 0, 0, 0.42 + 0.18 * card.front)
    }

    Rectangle {
        id: body
        anchors.fill: parent
        radius: card.radius
        antialiasing: true
        color: card.dress.surface
        border.width: Math.max(1, card.u * (1 + card.front * 0.6))
        border.color: Qt.tint(card.withAlpha(card.dress.line, 0.85), card.withAlpha(card.dress.accent, card.front * 0.85))

        Rectangle {
            id: thumbBase
            x: card.pad
            y: card.pad
            width: parent.width - card.pad * 2
            height: Math.round(width * 10 / 16)
            color: card.dress.raised
            antialiasing: true
            radius: card.thumbRadius
        }

        Text {
            anchors.centerIn: thumbBase
            visible: thumb.status !== Image.Ready
            text: "\uf03e"
            font.family: Theme.icon
            font.pixelSize: 22 * card.u
            color: Theme.withAlpha(card.dress.muted, 0.5)
        }

        Image {
            id: thumb
            anchors.fill: thumbBase
            source: card.picture
            fillMode: Image.PreserveAspectCrop
            sourceSize: Qt.size(Math.ceil(thumbBase.width * 1.3), Math.ceil(thumbBase.height * 1.3))
            asynchronous: true
            smooth: true
            antialiasing: true
            mipmap: true
            opacity: status === Image.Ready ? 1 : 0

            Behavior on opacity {
                NumberAnimation { duration: 220 }
            }
        }

        Rectangle {
            visible: card.thumbRadius > 0
            x: thumbBase.x - body.border.width
            y: thumbBase.y - body.border.width
            width: thumbBase.width + body.border.width * 2
            height: thumbBase.height + body.border.width * 2
            radius: card.thumbRadius + body.border.width
            antialiasing: true
            color: "transparent"
            border.width: 5 * card.u
            border.color: card.dress.surface
        }

        Rectangle {
            id: currentTag
            x: thumbBase.x + 7 * card.u
            y: thumbBase.y + thumbBase.height - height - 7 * card.u
            visible: card.current
            width: currentRow.implicitWidth + 12 * card.u
            height: 19 * card.u
            radius: 9.5 * card.u
            antialiasing: true
            color: card.dress.accent
            transformOrigin: Item.Left

            Row {
                id: currentRow
                anchors.centerIn: parent
                spacing: 3 * card.u

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "\uf00c"
                    font.family: Theme.icon
                    font.pixelSize: 12 * card.u
                    color: card.dress.accentInk
                }

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "current"
                    font.family: Theme.sans
                    font.pixelSize: 10 * card.u
                    font.weight: Font.Bold
                    font.letterSpacing: 0.4 * card.u
                    color: card.dress.accentInk
                }
            }

            SequentialAnimation {
                id: pop
                NumberAnimation {
                    target: currentTag
                    property: "scale"
                    from: 0.4
                    to: 1.12
                    duration: 200
                    easing.type: Easing.OutCubic
                }
                NumberAnimation {
                    target: currentTag
                    property: "scale"
                    to: 1
                    duration: 160
                    easing.type: Easing.InOutQuad
                }
            }
        }

        Rectangle {
            visible: card.favourite
            x: thumbBase.x + thumbBase.width - width - 6 * card.u
            y: thumbBase.y + 6 * card.u
            width: 24 * card.u
            height: width
            radius: width / 2
            antialiasing: true
            color: Qt.rgba(0, 0, 0, 0.42)

            Text {
                anchors.centerIn: parent
                anchors.verticalCenterOffset: 0.5 * card.u
                text: "\u{f02d1}"
                font.family: Theme.icon
                font.pixelSize: 14 * card.u
                color: "#ff5c7a"
            }
        }

        Rectangle {
            visible: card.saved
            x: thumbBase.x + thumbBase.width - width - 6 * card.u
            y: thumbBase.y + thumbBase.height - height - 6 * card.u
            width: 24 * card.u
            height: width
            radius: width / 2
            antialiasing: true
            color: Qt.rgba(0, 0, 0, 0.5)

            Text {
                anchors.centerIn: parent
                text: "\uf0193"
                font.family: Theme.icon
                font.pixelSize: 14 * card.u
                color: "white"
            }
        }

        Column {
            id: chipColumn
            x: card.pad
            y: thumbBase.y + thumbBase.height + 8 * card.u
            width: thumbBase.width

            Repeater {
                model: 5

                Rectangle {
                    id: chip
                    required property int index
                    readonly property var spec: card.chips[index] || null
                    readonly property real r: card.thumbRadius * 0.7

                    width: chipColumn.width
                    height: card.chipHeight
                    antialiasing: true
                    topLeftRadius: index === 0 ? r : 0
                    topRightRadius: index === 0 ? r : 0
                    bottomLeftRadius: index === 4 ? r : 0
                    bottomRightRadius: index === 4 ? r : 0
                    color: spec ? spec.fill : card.dress.raised

                    Text {
                        anchors.left: parent.left
                        anchors.leftMargin: 8 * card.u
                        anchors.verticalCenter: parent.verticalCenter
                        visible: !!chip.spec && chip.spec.label !== ""
                        text: chip.spec ? chip.spec.label.toUpperCase() : ""
                        font.family: Theme.sans
                        font.pixelSize: 8.5 * card.u
                        font.weight: Font.Bold
                        font.letterSpacing: 1.1 * card.u
                        color: card.inkOn(chip.color)
                        opacity: 0.85
                    }

                    Text {
                        anchors.right: parent.right
                        anchors.rightMargin: 8 * card.u
                        anchors.verticalCenter: parent.verticalCenter
                        visible: !!chip.spec && !!chip.spec.fill
                        text: chip.spec ? String(chip.spec.fill).toUpperCase() : ""
                        font.family: "JetBrainsMono Nerd Font"
                        font.pixelSize: 9.5 * card.u
                        font.weight: Font.Medium
                        color: card.inkOn(chip.color)
                    }
                }
            }
        }

        Text {
            id: titleText
            x: card.pad + 2 * card.u
            y: chipColumn.y + chipColumn.height + 9 * card.u
            width: thumbBase.width - 4 * card.u
            text: card.title
            elide: Text.ElideRight
            font.family: Theme.sans
            font.pixelSize: 13 * card.u
            font.weight: Font.DemiBold
            color: card.dress.ink
        }

        Text {
            id: metaText
            x: titleText.x
            y: titleText.y + titleText.height + 2 * card.u
            width: titleText.width
            text: card.meta
            elide: Text.ElideRight
            font.family: Theme.sans
            font.pixelSize: 10 * card.u
            color: card.dress.muted
        }

        Rectangle {
            anchors.fill: parent
            visible: opacity > 0
            opacity: card.hovered ? 0 : card.dim
            radius: parent.radius
            antialiasing: true
            color: "black"
        }
    }

    MouseArea {
        id: mouse
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        preventStealing: true
        property point origin: Qt.point(0, 0)

        onClicked: card.clicked()
        onDoubleClicked: card.doubleClicked()
        onPressed: origin = Qt.point(mouseX, mouseY)
        onPositionChanged: {
            if (pressed)
                card.dragged(mouseX - origin.x, mouseY - origin.y, false)
        }
        onReleased: {
            if (pressed)
                card.dragged(mouseX - origin.x, mouseY - origin.y, true)
        }
        onWheel: event => card.wheeled(event.angleY, event.pixelY)
    }

    onCurrentChanged: if (current) pop.restart()
}
