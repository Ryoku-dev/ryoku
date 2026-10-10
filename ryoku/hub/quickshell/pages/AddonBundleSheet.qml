pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons

FocusScope {
    id: sheet

    property bool open: false
    property var bundle: ({})
    property bool loading: false
    property bool busy: false

    signal dismissed()
    signal repairRequested()
    signal removeRequested()

    readonly property var components: ((sheet.bundle.metadata || {}).items || [])
    readonly property var installedComponents: sheet.components.filter(item => item.installed === true)
    readonly property var missingComponents: sheet.components.filter(item => item.installed !== true)
    readonly property int installedCount: Number(sheet.bundle.installedCount || 0)
    readonly property int totalCount: Number(sheet.bundle.totalCount || sheet.components.length)
    readonly property int missingCount: Math.max(0, sheet.totalCount - sheet.installedCount)
    readonly property string summary: sheet.bundle.summary || sheet.bundle.description
        || I18n.tr("Installed add-on bundle.")
    readonly property string icon: String((sheet.bundle.metadata || {}).icon || "inventory_2")
    readonly property bool motionEnabled: !Tokens.reduceMotion

    visible: false
    opacity: 0
    focus: visible

    function close() {
        if (sheet.open)
            sheet.dismissed();
    }

    onOpenChanged: {
        if (open) {
            closeAnimation.stop();
            visible = true;
            opacity = 0;
            panel.scale = 0.97;
            openAnimation.restart();
            Qt.callLater(() => sheet.forceActiveFocus());
        } else if (visible) {
            openAnimation.stop();
            closeAnimation.restart();
        }
    }

    Keys.onPressed: event => {
        if (event.key === Qt.Key_Escape) {
            event.accepted = true;
            sheet.close();
        }
    }

    ParallelAnimation {
        id: openAnimation
        NumberAnimation {
            target: sheet
            property: "opacity"
            from: 0
            to: 1
            duration: sheet.motionEnabled ? Tokens.swap : 0
            easing.type: Tokens.ease
        }
        NumberAnimation {
            target: panel
            property: "scale"
            from: 0.97
            to: 1
            duration: sheet.motionEnabled ? Tokens.swap : 0
            easing.type: Tokens.ease
        }
    }

    ParallelAnimation {
        id: closeAnimation
        NumberAnimation {
            target: sheet
            property: "opacity"
            to: 0
            duration: sheet.motionEnabled ? Tokens.snap : 0
            easing.type: Tokens.ease
        }
        NumberAnimation {
            target: panel
            property: "scale"
            to: 0.98
            duration: sheet.motionEnabled ? Tokens.snap : 0
            easing.type: Tokens.ease
        }
        onFinished: if (!sheet.open) sheet.visible = false
    }

    MouseArea {
        anchors.fill: parent
        onClicked: sheet.close()
    }

    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.78)
    }

    Rectangle {
        id: panel
        anchors.centerIn: parent
        width: Math.min(680, Math.max(320, sheet.width - Tokens.s6 * 2))
        height: Math.min(720, sheet.height - Tokens.s6 * 2,
            Math.max(360, 280 + Math.min(sheet.totalCount, 10) * 40))
        radius: Tokens.radius
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        transformOrigin: Item.Center

        MouseArea {
            anchors.fill: parent
            onClicked: mouse => mouse.accepted = true
        }

        Item {
            id: header
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.s5
            height: Math.max(iconPlate.height, heading.implicitHeight)

            Rectangle {
                id: iconPlate
                anchors.left: parent.left
                anchors.top: parent.top
                width: 48
                height: 48
                radius: Tokens.radius
                color: sheet.missingCount === 0 ? Tokens.bone : "transparent"
                border.width: sheet.missingCount === 0 ? 0 : Tokens.border
                border.color: Tokens.lineStrong

                Text {
                    anchors.centerIn: parent
                    text: sheet.icon
                    color: sheet.missingCount === 0 ? Tokens.inkOnBone : Tokens.inkDim
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: 24
                }
            }

            Column {
                id: heading
                anchors.left: iconPlate.right
                anchors.leftMargin: Tokens.s3
                anchors.right: closeButton.left
                anchors.rightMargin: Tokens.s3
                anchors.top: parent.top
                spacing: Tokens.s1

                Text {
                    width: parent.width
                    text: sheet.bundle.name || sheet.bundle.id || ""
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fValue
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    text: sheet.summary
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    elide: Text.ElideRight
                }
                Text {
                    width: parent.width
                    text: sheet.loading
                        ? I18n.tr("Checking bundle components…")
                        : I18n.tr("%1 of %2 installed").arg(sheet.installedCount).arg(sheet.totalCount)
                    color: sheet.loading ? Tokens.inkMuted : Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                }
            }

            Btn {
                id: closeButton
                anchors.right: parent.right
                anchors.top: parent.top
                compact: true
                text: I18n.tr("CLOSE")
                onAct: sheet.close()
            }
        }

        Rectangle {
            id: headerRule
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: header.bottom
            anchors.topMargin: Tokens.s5
            height: 1
            color: Tokens.lineSoft
        }

        Flickable {
            id: componentList
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: headerRule.bottom
            anchors.bottom: footerRule.top
            anchors.margins: Tokens.s5
            contentWidth: width
            contentHeight: componentColumn.implicitHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            visible: !sheet.loading
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: componentColumn
                width: componentList.width
                spacing: Tokens.s5

                Repeater {
                    model: [
                        {
                            title: I18n.tr("INSTALLED"),
                            components: sheet.installedComponents,
                            installed: true
                        },
                        {
                            title: I18n.tr("MISSING"),
                            components: sheet.missingComponents,
                            installed: false
                        }
                    ].filter(group => group.components.length > 0)

                    delegate: Column {
                        id: group
                        required property var modelData
                        width: componentColumn.width
                        spacing: Tokens.s2

                        Row {
                            spacing: Tokens.s2

                            Text {
                                text: group.modelData.title
                                color: Tokens.inkDim
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fMicro
                                font.weight: Font.Medium
                                font.letterSpacing: Tokens.trackLabel
                            }
                            Text {
                                text: String(group.modelData.components.length)
                                color: Tokens.inkMuted
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fMicro
                            }
                        }

                        Repeater {
                            model: group.modelData.components

                            delegate: Item {
                                id: componentRow
                                required property var modelData
                                width: group.width
                                height: 40

                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 24
                                    height: 24
                                    radius: 12
                                    color: group.modelData.installed ? Tokens.bone : "transparent"
                                    border.width: group.modelData.installed ? 0 : Tokens.border
                                    border.color: Tokens.lineStrong

                                    Text {
                                        anchors.centerIn: parent
                                        text: group.modelData.installed ? "check" : "close"
                                        color: group.modelData.installed ? Tokens.inkOnBone : Tokens.inkMuted
                                        font.family: "Material Symbols Rounded"
                                        font.pixelSize: 15
                                    }
                                }

                                Text {
                                    anchors.left: parent.left
                                    anchors.leftMargin: 24 + Tokens.s3
                                    anchors.right: kindLabel.left
                                    anchors.rightMargin: Tokens.s3
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: componentRow.modelData.name || ""
                                    color: Tokens.ink
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fSmall
                                    elide: Text.ElideMiddle
                                }

                                Text {
                                    id: kindLabel
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: String(componentRow.modelData.type
                                        || componentRow.modelData.kind || "")
                                    color: Tokens.inkMuted
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny
                                }

                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.bottom: parent.bottom
                                    height: 1
                                    color: Tokens.lineSoft
                                }
                            }
                        }
                    }
                }
            }
        }

        Item {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: headerRule.bottom
            anchors.bottom: footerRule.top
            visible: sheet.loading

            Column {
                anchors.centerIn: parent
                spacing: Tokens.s3

                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: "progress_activity"
                    color: Tokens.inkMuted
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: 28
                    RotationAnimator on rotation {
                        running: sheet.loading && sheet.motionEnabled
                        from: 0
                        to: 360
                        duration: 900
                        loops: Animation.Infinite
                    }
                }
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: I18n.tr("Checking installed components")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }
            }
        }

        Rectangle {
            id: footerRule
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: footer.top
            height: 1
            color: Tokens.lineSoft
        }

        Item {
            id: footer
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.margins: Tokens.s4
            height: 32

            Text {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: sheet.missingCount === 0
                    ? I18n.tr("Bundle complete")
                    : (sheet.missingCount === 1
                        ? I18n.tr("%1 component missing")
                        : I18n.tr("%1 components missing")).arg(sheet.missingCount)
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny
            }

            Row {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s2

                Btn {
                    visible: sheet.missingCount > 0
                    text: sheet.busy ? I18n.tr("WORKING") : I18n.tr("REPAIR")
                    armed: !sheet.busy && !sheet.loading
                    onAct: sheet.repairRequested()
                }
                Btn {
                    visible: sheet.installedCount > 0
                    text: sheet.busy ? I18n.tr("WORKING") : I18n.tr("REMOVE")
                    armed: !sheet.busy && !sheet.loading
                    onAct: sheet.removeRequested()
                }
            }
        }
    }
}
