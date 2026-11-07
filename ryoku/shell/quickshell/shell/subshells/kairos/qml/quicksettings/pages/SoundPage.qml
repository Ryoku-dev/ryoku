pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import shell.barkit as Pill
import Ryoku.Ui.Singletons
import "../kit" as K

Column {
    id: page

    property var host

    spacing: 12

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    readonly property color accent: Theme.primary
    function dim(a) { return Qt.rgba(page.ink.r, page.ink.g, page.ink.b, a) }

    readonly property var sink: Audio.sink
    readonly property var source: Audio.source
    readonly property real outVol: (page.sink && page.sink.audio) ? page.sink.audio.volume : 0
    readonly property bool outMuted: (page.sink && page.sink.audio) ? page.sink.audio.muted : false
    readonly property real inVol: (page.source && page.source.audio) ? page.source.audio.volume : 0
    readonly property bool inMuted: (page.source && page.source.audio) ? page.source.audio.muted : false

    function setOut(v) {
        if (!page.sink || !page.sink.audio) return;
        var a = page.sink.audio;
        if (a.muted && v > 0) a.muted = false;
        a.volume = Math.max(0, Math.min(1, v));
    }
    function setIn(v) {
        if (!page.source || !page.source.audio) return;
        var a = page.source.audio;
        if (a.muted && v > 0) a.muted = false;
        a.volume = Math.max(0, Math.min(1, v));
    }
    function toggleOut() { if (page.sink && page.sink.audio) page.sink.audio.muted = !page.sink.audio.muted; }
    function toggleIn() { if (page.source && page.source.audio) page.source.audio.muted = !page.source.audio.muted; }

    Item {
        width: parent.width
        height: 36

        K.QsBack {
            id: back
            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
            onClicked: page.host.back()
        }
        Text {
            anchors { left: back.right; leftMargin: 12; verticalCenter: parent.verticalCenter }
            text: I18n.tr("Sound")
            color: page.ink
            font.family: Theme.fontPrimary
            font.pixelSize: 18
            font.weight: Font.DemiBold
        }
    }

    Text {
        text: I18n.tr("Output")
        color: page.dim(0.55)
        font.family: Theme.fontPrimary
        font.pixelSize: 12
    }

    Rectangle {
        width: parent.width
        height: 72
        radius: 18
        color: page.dim(0.06)

        K.QsFader {
            anchors {
                left: parent.left; leftMargin: 14
                right: muteBtn.left; rightMargin: 10
                verticalCenter: parent.verticalCenter
            }
            icon: page.outMuted ? "volume_off" : "volume_up"
            value: page.outVol
            onMoved: (v) => page.setOut(v)
        }
        Item {
            id: muteBtn
            anchors { right: parent.right; rightMargin: 10; verticalCenter: parent.verticalCenter }
            width: 38
            height: 38
            Rectangle {
                anchors.fill: parent
                radius: 19
                color: page.outMuted
                    ? Qt.rgba(page.accent.r, page.accent.g, page.accent.b, 0.28)
                    : (muteHover.hovered ? page.dim(0.14) : page.dim(0.08))
            }
            Pill.MaterialIcon {
                anchors.centerIn: parent
                text: page.outMuted ? "volume_off" : "volume_up"
                color: page.outMuted ? page.accent : page.ink
                font.pixelSize: 19
            }
            HoverHandler { id: muteHover }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: page.toggleOut()
            }
        }
    }

    Column {
        width: parent.width
        spacing: 8

        Repeater {
            model: Audio.outputs.filter(Audio.alive)
            delegate: Rectangle {
                id: outRow
                required property var modelData
                readonly property bool active: outRow.modelData === Audio.sink
                width: parent.width
                height: 52
                radius: 16
                color: outRow.active
                    ? Qt.rgba(page.accent.r, page.accent.g, page.accent.b, 0.12)
                    : page.dim(0.05)

                Rectangle {
                    id: outDisc
                    anchors { left: parent.left; leftMargin: 9; verticalCenter: parent.verticalCenter }
                    width: 34
                    height: 34
                    radius: 17
                    color: page.dim(0.10)
                    Pill.GlyphIcon {
                        anchors.centerIn: parent
                        width: 18
                        height: 18
                        name: Audio.nodeIcon(outRow.modelData)
                        color: page.dim(0.85)
                    }
                }
                Text {
                    anchors {
                        left: outDisc.right; leftMargin: 10
                        right: outCheck.left; rightMargin: 8
                        verticalCenter: parent.verticalCenter
                    }
                    text: Audio.nodeLabel(outRow.modelData)
                    color: page.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 13
                    elide: Text.ElideRight
                }
                Pill.MaterialIcon {
                    id: outCheck
                    anchors { right: parent.right; rightMargin: 14; verticalCenter: parent.verticalCenter }
                    visible: outRow.active
                    text: "check"
                    color: page.accent
                    font.pixelSize: 18
                }
                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: Audio.setOutput(outRow.modelData)
                }
            }
        }
    }

    Text {
        text: I18n.tr("Input")
        color: page.dim(0.55)
        font.family: Theme.fontPrimary
        font.pixelSize: 12
    }

    Rectangle {
        width: parent.width
        height: 72
        radius: 18
        color: page.dim(0.06)

        K.QsFader {
            anchors {
                left: parent.left; leftMargin: 14
                right: micBtn.left; rightMargin: 10
                verticalCenter: parent.verticalCenter
            }
            icon: page.inMuted ? "mic_off" : "mic"
            value: page.inVol
            onMoved: (v) => page.setIn(v)
        }
        Item {
            id: micBtn
            anchors { right: parent.right; rightMargin: 10; verticalCenter: parent.verticalCenter }
            width: 38
            height: 38
            Rectangle {
                anchors.fill: parent
                radius: 19
                color: page.inMuted
                    ? Qt.rgba(page.accent.r, page.accent.g, page.accent.b, 0.28)
                    : (micHover.hovered ? page.dim(0.14) : page.dim(0.08))
            }
            Pill.MaterialIcon {
                anchors.centerIn: parent
                text: page.inMuted ? "mic_off" : "mic"
                color: page.inMuted ? page.accent : page.ink
                font.pixelSize: 19
            }
            HoverHandler { id: micHover }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: page.toggleIn()
            }
        }
    }

    Column {
        width: parent.width
        spacing: 8

        Repeater {
            model: Audio.inputs.filter(Audio.alive)
            delegate: Rectangle {
                id: inRow
                required property var modelData
                readonly property bool active: inRow.modelData === Audio.source
                width: parent.width
                height: 52
                radius: 16
                color: inRow.active
                    ? Qt.rgba(page.accent.r, page.accent.g, page.accent.b, 0.12)
                    : page.dim(0.05)

                Rectangle {
                    id: inDisc
                    anchors { left: parent.left; leftMargin: 9; verticalCenter: parent.verticalCenter }
                    width: 34
                    height: 34
                    radius: 17
                    color: page.dim(0.10)
                    Pill.GlyphIcon {
                        anchors.centerIn: parent
                        width: 18
                        height: 18
                        name: Audio.nodeIcon(inRow.modelData)
                        color: page.dim(0.85)
                    }
                }
                Text {
                    anchors {
                        left: inDisc.right; leftMargin: 10
                        right: inCheck.left; rightMargin: 8
                        verticalCenter: parent.verticalCenter
                    }
                    text: Audio.nodeLabel(inRow.modelData)
                    color: page.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 13
                    elide: Text.ElideRight
                }
                Pill.MaterialIcon {
                    id: inCheck
                    anchors { right: parent.right; rightMargin: 14; verticalCenter: parent.verticalCenter }
                    visible: inRow.active
                    text: "check"
                    color: page.accent
                    font.pixelSize: 18
                }
                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: Audio.setInput(inRow.modelData)
                }
            }
        }
    }
}
