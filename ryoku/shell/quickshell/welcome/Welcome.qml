pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

Rectangle {
    id: root

    color: Tokens.paper
    focus: true
    implicitWidth: 1180
    implicitHeight: 760

    property int step: 0
    property string barStyle: "qsbar"
    property real arrival: 0
    readonly property int lastStep: steps.length - 1
    readonly property string socketPath:
        (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
    readonly property var steps: [
        { label: I18n.tr("Hello"), next: I18n.tr("Show me around") },
        { label: I18n.tr("Your bar"), next: I18n.tr("Make it mine") },
        { label: I18n.tr("Make it yours"), next: I18n.tr("Show the essentials") },
        { label: I18n.tr("Essentials"), next: I18n.tr("Almost done") },
        { label: I18n.tr("Done"), next: I18n.tr("Finish") }
    ]

    function advance() {
        if (step < lastStep)
            step++
        else
            finish()
    }

    function back() {
        if (step > 0)
            step--
    }

    function finish() {
        Qt.quit()
    }

    function runCommand(argv) {
        if (argv && argv.length > 0)
            Quickshell.execDetached(argv)
    }

    function chooseBar(styleId) {
        barStyle = styleId
        control.queued += "call settings.patch "
            + JSON.stringify({ path: "barStyle", value: styleId }) + "\n"
        if (control.connected)
            control.flushQueued()
        else
            control.connected = true
    }

    function applySettings(line) {
        try {
            const frame = JSON.parse(line)
            const liveStyle = frame && typeof frame.barStyle === "string"
                ? frame.barStyle : ""
            if (["sumi", "qsbar", "kairos", "iris", "python"].indexOf(liveStyle) >= 0)
                barStyle = liveStyle
        } catch (e) {
        }
    }

    Keys.onEscapePressed: finish()
    Keys.onLeftPressed: back()
    Keys.onRightPressed: advance()
    Keys.onReturnPressed: advance()
    Keys.onEnterPressed: advance()

    Socket {
        id: settings
        path: root.socketPath
        parser: SplitParser { onRead: line => root.applySettings(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe settings\n")
                flush()
            } else {
                reconnect.restart()
            }
        }
    }

    Timer {
        id: reconnect
        interval: 2000
        onTriggered: if (!settings.connected) settings.connected = true
    }

    Socket {
        id: control
        path: root.socketPath
        property string queued: ""

        function flushQueued() {
            if (queued.length === 0)
                return
            write(queued)
            flush()
            queued = ""
        }

        onConnectionStateChanged: if (connected) flushQueued()
    }

    Item {
        id: topBar
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 64
        opacity: root.arrival

        Row {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Image {
                width: 32
                height: 32
                source: Qt.resolvedUrl("art/logo-mark.svg")
                fillMode: Image.PreserveAspectFit
                sourceSize: Qt.size(64, 64)
            }

            Column {
                anchors.verticalCenter: parent.verticalCenter
                spacing: 1

                Text {
                    text: I18n.tr("RYOKU")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    font.weight: Font.DemiBold
                    font.letterSpacing: Tokens.trackMark
                }
                Text {
                    text: I18n.tr("FIRST RUN")
                    color: Tokens.inkFaint
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackLabel
                }
            }
        }

        Row {
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Repeater {
                model: root.steps

                Item {
                    id: progressItem
                    required property int index
                    required property var modelData
                    width: progressLabel.implicitWidth
                    height: 34

                    readonly property bool active: root.step === index
                    readonly property bool visited: index < root.step

                    Text {
                        id: progressLabel
                        anchors.top: parent.top
                        text: ("0" + (progressItem.index + 1)).slice(-2)
                            + "  " + progressItem.modelData.label
                        color: progressItem.active ? Tokens.ink
                            : (progressItem.visited ? Tokens.inkMuted : Tokens.inkFaint)
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fTiny
                        font.letterSpacing: 0.4
                    }

                    Rectangle {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        height: progressItem.active ? 3 : 1
                        color: progressItem.active ? Tokens.ink : Tokens.line
                        Behavior on height {
                            NumberAnimation { duration: Motion.snap; easing.type: Tokens.ease }
                        }
                        Behavior on color { ColorAnimation { duration: Motion.snap } }
                    }

                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: root.step = progressItem.index }
                }
            }
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 1
            color: Tokens.line
        }
    }

    Item {
        id: pageFrame
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: topBar.bottom
        anchors.bottom: navigation.top
        anchors.margins: Tokens.s6
        anchors.topMargin: Tokens.s4
        anchors.bottomMargin: Tokens.s4
        clip: true
        opacity: root.arrival
        transform: Translate { y: (1 - root.arrival) * 12 }

        Loader {
            id: pageLoader
            anchors.fill: parent
            sourceComponent: root.pageFor(root.step)
            opacity: 1
            transform: Translate { id: pageShift; y: 0 }
        }
    }

    Item {
        id: navigation
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 64
        opacity: root.arrival

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: 1
            color: Tokens.line
        }

        Btn {
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            text: root.step === 0 ? I18n.tr("Skip") : I18n.tr("Back")
            onAct: root.step === 0 ? root.finish() : root.back()
        }

        Row {
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: ("0" + (root.step + 1)).slice(-2) + " / 0" + root.steps.length
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
                font.letterSpacing: Tokens.trackLabel
            }

            Btn {
                visible: root.step < root.lastStep
                text: root.steps[root.step].next
                primary: true
                onAct: root.advance()
            }
        }
    }

    function pageFor(index) {
        switch (index) {
        case 0: return helloPage
        case 1: return barsPage
        case 2: return makePage
        case 3: return essentialsPage
        default: return donePage
        }
    }

    Component { id: helloPage; PageHello {} }
    Component {
        id: barsPage
        PageBars {
            selectedStyle: root.barStyle
            onChose: styleId => root.chooseBar(styleId)
        }
    }
    Component {
        id: makePage
        PageMake { onRunCommand: argv => root.runCommand(argv) }
    }
    Component {
        id: essentialsPage
        PageEssentials { onRunCommand: argv => root.runCommand(argv) }
    }
    Component {
        id: donePage
        PageDone {
            onRunCommand: argv => root.runCommand(argv)
            onFinish: root.finish()
        }
    }

    SequentialAnimation {
        id: pageReveal
        PropertyAction { target: pageLoader; property: "opacity"; value: 0 }
        PropertyAction { target: pageShift; property: "y"; value: 10 }
        ParallelAnimation {
            NumberAnimation {
                target: pageLoader
                property: "opacity"
                to: 1
                duration: Motion.swap
                easing.type: Tokens.ease
            }
            NumberAnimation {
                target: pageShift
                property: "y"
                to: 0
                duration: Motion.swap
                easing.type: Tokens.ease
            }
        }
    }

    NumberAnimation {
        id: entrance
        target: root
        property: "arrival"
        from: 0
        to: 1
        duration: Motion.reduce ? 0 : Tokens.durLarge
        easing.type: Tokens.ease
    }

    onStepChanged: pageReveal.restart()
    Component.onCompleted: entrance.start()
}
