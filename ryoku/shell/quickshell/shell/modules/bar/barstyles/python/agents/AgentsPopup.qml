import QtQuick
import QtQuick.Layouts
import Quickshell
import "../"
import "../reusables"
import shell.services as Ryoku

Item {
    id: window
    visible: false
    focus: true

    readonly property var agents: Ryoku.AiUsage.present
    property real nowSec: Date.now() / 1000
    property real introMain: 0
    property var exitCallback: null

    function startExit(onFinished) {
        introAnim.stop();
        exitCallback = onFinished;
        exitAnim.restart();
    }

    onVisibleChanged: {
        if (visible) {
            nowSec = Date.now() / 1000;
            forceActiveFocus();
            exitAnim.stop();
            introMain = 0;
            introAnim.restart();
        } else {
            introAnim.stop();
            exitAnim.stop();
            introMain = 0;
        }
    }

    NumberAnimation {
        id: introAnim
        target: window; property: "introMain"; from: 0; to: 1.0
        duration: 600; easing.type: Easing.OutExpo
    }

    SequentialAnimation {
        id: exitAnim
        NumberAnimation { target: window; property: "introMain"; to: 0.0; duration: 150; easing.type: Easing.InQuart }
        ScriptAction {
            script: {
                if (window.exitCallback) {
                    let cb = window.exitCallback;
                    window.exitCallback = null;
                    cb();
                }
            }
        }
    }

    Timer {
        interval: 30000
        running: window.visible
        repeat: true
        onTriggered: window.nowSec = Date.now() / 1000
    }

    Item {
        anchors.fill: parent
        scale: 0.95 + (0.05 * window.introMain)
        opacity: window.introMain
        transform: Translate { y: Scaler.s(20) * (1 - window.introMain) }

        // The window is sized for several agents; the panel hugs its content.
        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            height: Math.min(parent.height, content.implicitHeight + content.anchors.margins * 2)
            radius: ThemeBackend.borderRadius
            color: Qt.alpha(ThemeBackend.base, ThemeBackend.elementOpacity)
            clip: true

            WallpaperBlur {
                z: -1
                anchors.fill: parent
                topLeftRadius: parent.topLeftRadius
                topRightRadius: parent.topRightRadius
                bottomLeftRadius: parent.bottomLeftRadius
                bottomRightRadius: parent.bottomRightRadius
            }

            ColumnLayout {
                id: content
                anchors.fill: parent
                anchors.margins: Scaler.s(18)
                spacing: Scaler.s(14)

                RowLayout {
                    Layout.fillWidth: true
                    spacing: Scaler.s(10)

                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: Scaler.s(2)

                        Text {
                            text: AgentsFormat.tr("title", "Agent usage")
                            font.family: ThemeBackend.fontFamily
                            font.pixelSize: Scaler.s(20)
                            font.weight: Font.Black
                            color: ThemeBackend.text
                        }

                        Text {
                            text: Ryoku.AiUsage.refreshing
                                ? AgentsFormat.tr("refreshing", "Refreshing…")
                                : AgentsFormat.tr("subtitle", "Updates every few minutes")
                            font.family: ThemeBackend.fontFamily
                            font.pixelSize: Scaler.s(12)
                            color: ThemeBackend.subtext0
                        }
                    }

                    Rectangle {
                        Layout.preferredWidth: Scaler.s(36)
                        Layout.preferredHeight: Scaler.s(36)
                        radius: ThemeBackend.borderRadius
                        color: refreshMouse.containsMouse ? ThemeBackend.surface1 : ThemeBackend.surface0

                        Text {
                            anchors.centerIn: parent
                            text: "󰑐"
                            font.family: ThemeBackend.iconFont
                            font.pixelSize: Scaler.s(18)
                            color: ThemeBackend.text
                            RotationAnimation on rotation {
                                running: Ryoku.AiUsage.refreshing
                                loops: Animation.Infinite
                                from: 0; to: 360; duration: 900
                            }
                        }

                        MouseArea {
                            id: refreshMouse
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: Ryoku.AiUsage.refresh()
                        }
                    }
                }

                Text {
                    Layout.fillWidth: true
                    visible: window.agents.length === 0
                    text: AgentsFormat.tr("empty", "No agent usage yet. Ryoku reads the caches that the Claude, Codex and OpenCode collectors write.")
                    wrapMode: Text.WordWrap
                    font.family: ThemeBackend.fontFamily
                    font.pixelSize: Scaler.s(13)
                    color: ThemeBackend.subtext0
                }

                Flickable {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    Layout.minimumHeight: 0
                    Layout.preferredHeight: cards.implicitHeight
                    contentHeight: cards.implicitHeight
                    clip: true
                    boundsBehavior: Flickable.StopAtBounds

                    ColumnLayout {
                        id: cards
                        width: parent.width
                        spacing: Scaler.s(12)

                        Repeater {
                            model: window.agents

                            AgentCard {
                                required property string modelData
                                Layout.fillWidth: true
                                agentId: modelData
                                nowSec: window.nowSec
                            }
                        }
                    }
                }
            }
        }
    }
}
