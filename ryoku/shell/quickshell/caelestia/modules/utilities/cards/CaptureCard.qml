pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Components
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

StyledRect {
    id: root

    required property var props
    required property DrawerVisibilities visibilities
    readonly property bool shotMode: root.props.captureMode === "shot"
    readonly property real nonAnimHeight: btnLayout.implicitHeight + modeRow.implicitHeight + listOrControls.implicitHeight + layout.spacing * 2 + layout.anchors.margins * 2

    Layout.fillWidth: true
    implicitHeight: layout.implicitHeight + layout.anchors.margins * 2

    radius: Tokens.rounding.large
    color: Colours.tPalette.m3surfaceContainer

    ColumnLayout {
        id: layout

        anchors.fill: parent
        anchors.margins: Tokens.padding.large
        spacing: Tokens.spacing.medium

        RowLayout {
            id: btnLayout

            spacing: Tokens.spacing.medium

            StyledRect {
                implicitWidth: implicitHeight
                implicitHeight: icon.implicitHeight + Tokens.padding.small * 2

                radius: Tokens.rounding.full
                color: Recorder.running ? Colours.palette.m3secondary : Colours.palette.m3secondaryContainer

                MaterialIcon {
                    id: icon

                    anchors.centerIn: parent
                    anchors.verticalCenterOffset: Centering.pixelAlign(parent.height, height)
                    text: root.shotMode && !Recorder.running ? "screenshot_region" : "screen_record"
                    color: Recorder.running ? Colours.palette.m3onSecondary : Colours.palette.m3onSecondaryContainer
                    fontStyle: Tokens.font.icon.large
                }
            }

            ColumnLayout {
                Layout.fillWidth: true
                spacing: 0

                StyledText {
                    Layout.fillWidth: true
                    text: qsTr("Screen Capture")
                    font: Tokens.font.body.medium
                    elide: Text.ElideRight
                }

                StyledText {
                    Layout.fillWidth: true
                    text: root.shotMode ? qsTr("Capture, OCR and image search") : Recorder.paused ? qsTr("Paused") : Recorder.running ? qsTr("Running...") : qsTr("Ready")
                    color: Colours.palette.m3onSurfaceVariant
                    font: Tokens.font.body.small
                    elide: Text.ElideRight
                    animate: true
                }
            }

            SplitButton {
                visible: !root.shotMode
                disabled: Recorder.running

                active: menuItems.find(m => root.props.recordingMode === m.icon + m.text) ?? menuItems[0]
                menu.onItemSelected: item => root.props.recordingMode = item.icon + item.text

                menuItems: [
                    MenuItem {
                        icon: "fullscreen"
                        text: qsTr("Record")
                        activeText: qsTr("Start")
                        onClicked: {
                            root.visibilities.utilities = false;
                            Recorder.start();
                        }
                    },
                    MenuItem {
                        icon: "volume_up"
                        text: qsTr("Record with Sound")
                        activeText: qsTr("Start")
                        onClicked: {
                            root.visibilities.utilities = false;
                            Recorder.start(["-s"]);
                        }
                    },
                    MenuItem {
                        icon: "mic"
                        text: qsTr("Record with Microphone")
                        activeText: qsTr("Start")
                        onClicked: {
                            root.visibilities.utilities = false;
                            Recorder.start(["-i"]);
                        }
                    },
                    MenuItem {
                        icon: "select_to_speak"
                        text: qsTr("Both Sound & Microphone")
                        activeText: qsTr("Start")
                        onClicked: {
                            root.visibilities.utilities = false;
                            Recorder.start(["-s", "-i"]);
                        }
                    },
                    MenuItem {
                        visible: Config.utilities.showGifRecorder
                        icon: "animated_images"
                        text: qsTr("Record GIF")
                        activeText: qsTr("Start")
                        onClicked: {
                            root.visibilities.utilities = false;
                            Recorder.startGif();
                        }
                    },
                    MenuItem {
                        icon: "screenshot_region"
                        text: qsTr("Use Spectacle")
                        activeText: qsTr("Spectacle")
                        onClicked: {
                            root.visibilities.utilities = false;
                            Recorder.launchSpectacle();
                        }
                    }
                ]
            }

            SplitButton {
                visible: root.shotMode
                disabled: Recorder.running

                active: captureShotItem

                menuItems: [
                    MenuItem {
                        id: captureShotItem

                        icon: "screenshot_region"
                        text: qsTr("Capture region")
                        activeText: qsTr("Capture")
                        onClicked: {
                            root.visibilities.utilities = false;
                            if (!Visibilities.sidebarPinned)
                                root.visibilities.sidebar = false;
                            Quickshell.execDetached(["qs", "-c", "caelestia", "ipc", "call", "region", "screenshot"]);
                        }
                    },
                    MenuItem {
                        icon: "fullscreen"
                        text: qsTr("Capture fullscreen")
                        activeText: qsTr("Fullscreen")
                        onClicked: {
                            root.visibilities.utilities = false;
                            if (!Visibilities.sidebarPinned)
                                root.visibilities.sidebar = false;
                            const pad = n => String(n).padStart(2, "0");
                            const now = new Date();
                            const file = `${GlobalConfig.paths.screenshotsDir}/screenshot-${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}_${pad(now.getHours())}.${pad(now.getMinutes())}.${pad(now.getSeconds())}.png`;
                            Quickshell.execDetached(["spectacle", "-b", "-f", "-o", file]);
                        }
                    },
                    MenuItem {
                        icon: "select_window"
                        text: qsTr("Capture active window")
                        activeText: qsTr("Window")
                        onClicked: {
                            root.visibilities.utilities = false;
                            if (!Visibilities.sidebarPinned)
                                root.visibilities.sidebar = false;
                            const pad2 = n => String(n).padStart(2, "0");
                            const now2 = new Date();
                            const file2 = `${GlobalConfig.paths.screenshotsDir}/screenshot-${now2.getFullYear()}-${pad2(now2.getMonth() + 1)}-${pad2(now2.getDate())}_${pad2(now2.getHours())}.${pad2(now2.getMinutes())}.${pad2(now2.getSeconds())}.png`;
                            Quickshell.execDetached(["spectacle", "-b", "-a", "-o", file2]);
                        }
                    },
                    MenuItem {
                        icon: "text_fields"
                        text: qsTr("Recognize text")
                        activeText: qsTr("Recognize")
                        onClicked: {
                            root.visibilities.utilities = false;
                            if (!Visibilities.sidebarPinned)
                                root.visibilities.sidebar = false;
                            Quickshell.execDetached(["qs", "-c", "caelestia", "ipc", "call", "region", "ocr"]);
                        }
                    },
                    MenuItem {
                        icon: "image_search"
                        text: qsTr("Search image")
                        activeText: qsTr("Search")
                        onClicked: {
                            root.visibilities.utilities = false;
                            if (!Visibilities.sidebarPinned)
                                root.visibilities.sidebar = false;
                            Quickshell.execDetached(["qs", "-c", "caelestia", "ipc", "call", "region", "search"]);
                        }
                    },
                    MenuItem {
                        icon: "screenshot_monitor"
                        text: qsTr("Use Spectacle")
                        activeText: qsTr("Spectacle")
                        onClicked: {
                            root.visibilities.utilities = false;
                            if (!Visibilities.sidebarPinned)
                                root.visibilities.sidebar = false;
                            Quickshell.execDetached(["spectacle"]);
                        }
                    }
                ]
            }
        }

        ButtonRow {
            id: modeRow

            Layout.fillWidth: true
            spacing: Tokens.spacing.small

            // Scroll over the switcher to flip Record/Capture, like the Quick Toggles card
            WheelHandler {
                acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                onWheel: event => {
                    const d = event.angleDelta.y !== 0 ? event.angleDelta.y : event.angleDelta.x;
                    if (d < 0)
                        root.props.captureMode = "shot";
                    else if (d > 0)
                        root.props.captureMode = "record";
                }
            }

            // No isToggle: the checked binding alone drives the colour, so clicking the
            // already selected mode can't leave it looking deselected
            TextButton {
                Layout.fillWidth: true
                text: qsTr("Record")
                type: TextButton.Tonal
                checked: !root.shotMode
                onClicked: root.props.captureMode = "record"
            }

            TextButton {
                Layout.fillWidth: true
                text: qsTr("Capture")
                type: TextButton.Tonal
                checked: root.shotMode
                onClicked: root.props.captureMode = "shot"
            }
        }

        Loader {
            id: listOrControls

            property bool running: Recorder.running

            asynchronous: true
            Layout.fillWidth: true
            Layout.preferredHeight: implicitHeight
            sourceComponent: running ? recordingControls : root.shotMode ? screenshotList : recordingList
            clip: Layout.preferredHeight < implicitHeight

            Behavior on Layout.preferredHeight {
                id: locHeightAnim

                enabled: false

                Anim {}
            }

            Behavior on running {
                SequentialAnimation {
                    Anim {
                        target: listOrControls
                        property: "opacity"
                        to: 0
                        type: Anim.DefaultEffects
                    }
                    PropertyAction {
                        target: locHeightAnim
                        property: "enabled"
                        value: true
                    }
                    PropertyAction {}
                    ParallelAnimation {
                        SequentialAnimation {
                            PauseAnimation {
                                duration: 100
                            }
                            PropertyAction {
                                target: locHeightAnim
                                property: "enabled"
                                value: false
                            }
                        }
                        Anim {
                            target: listOrControls
                            property: "opacity"
                            to: 1
                            type: Anim.SlowEffects
                        }
                    }
                }
            }
        }
    }

    Component {
        id: recordingList

        RecordingList {
            props: root.props
            visibilities: root.visibilities
        }
    }

    Component {
        id: screenshotList

        ScreenshotList {
            props: root.props
            visibilities: root.visibilities
        }
    }

    Component {
        id: recordingControls

        RowLayout {
            spacing: Tokens.spacing.medium

            StyledRect {
                radius: Tokens.rounding.full
                color: Recorder.paused ? Colours.palette.m3tertiary : Colours.palette.m3error

                implicitWidth: recText.implicitWidth + Tokens.padding.medium * 2
                implicitHeight: recText.implicitHeight + Tokens.padding.large

                StyledText {
                    id: recText

                    anchors.centerIn: parent
                    animate: true
                    text: Recorder.paused ? "PAUSED" : "REC"
                    color: Recorder.paused ? Colours.palette.m3onTertiary : Colours.palette.m3onError
                    font: Tokens.font.mono.small
                }

                Behavior on implicitWidth {
                    Anim {}
                }

                SequentialAnimation on opacity {
                    running: !Recorder.paused
                    alwaysRunToEnd: true
                    loops: Animation.Infinite

                    Anim {
                        from: 1
                        to: 0
                        duration: Tokens.anim.durations.large
                        easing: Tokens.anim.emphasizedAccel
                    }
                    Anim {
                        from: 0
                        to: 1
                        duration: Tokens.anim.durations.extraLarge
                        easing: Tokens.anim.emphasizedDecel
                    }
                }
            }

            StyledText {
                Layout.fillWidth: true
                text: qsTr("Recording %1").arg(Units.formatDuration(Recorder.elapsed))
                font: Tokens.font.body.medium
                elide: Text.ElideMiddle
            }

            ButtonRow {
                spacing: Tokens.spacing.extraSmall

                IconButton {
                    shapeMorph: true
                    isRound: true
                    label.animate: true
                    icon: Recorder.paused ? "play_arrow" : "pause"
                    isToggle: true
                    checked: Recorder.paused
                    type: IconButton.Tonal
                    font: Tokens.font.icon.medium
                    onClicked: {
                        Recorder.togglePause();
                        internalChecked = Recorder.paused;
                    }

                    implicitWidth: {
                        const h = label.implicitHeight + Tokens.padding.large * 2;
                        if (h % 2 !== 0)
                            return h + 1;
                        return h;
                    }
                }

                IconButton {
                    shapeMorph: true
                    isRound: true
                    icon: "stop"
                    inactiveColour: Colours.palette.m3error
                    inactiveOnColour: Colours.palette.m3onError
                    font: Tokens.font.icon.medium
                    onClicked: Recorder.stop()

                    implicitWidth: {
                        const h = label.implicitHeight + Tokens.padding.large * 2;
                        if (h % 2 !== 0)
                            return h + 1;
                        return h;
                    }
                }
            }
        }
    }
}
