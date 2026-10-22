pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.services

ColumnLayout {
    id: root

    required property PopoutState popouts

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false

    readonly property bool hasUpdate: UpdateChecker.hasUpdate

    readonly property bool checking: UpdateChecker.checkingUpdates

    readonly property string statusText: {
        if (root.checking)
            return qsTr("Checking for updates…");
        if (root.hasUpdate)
            return UpdateChecker.versionSummaryMode
                ? qsTr("New version available on %1").arg(UpdateChecker.currentBranch)
                : qsTr("%1 new commits on %2 branch").arg(UpdateChecker.pendingCount).arg(UpdateChecker.currentBranch);
        return qsTr("System is up to date");
    }

    readonly property string statusIcon: root.checking ? "sync" : root.hasUpdate ? "update" : "check_circle"

    property double nowMs: Date.now()

    function formatDuration(ms: double): string {
        const total = Math.max(0, Math.floor(ms / 1000));
        const minutes = Math.floor(total / 60);
        const seconds = total % 60;
        return minutes > 0 ? qsTr("%1m %2s").arg(minutes).arg(seconds) : qsTr("%1s").arg(seconds);
    }

    function hideIndicator(): void {
        const entries = GlobalConfig.bar.entries ? [...GlobalConfig.bar.entries] : [];
        const idx = entries.findIndex(e => e.id === "updateIndicator");
        if (idx >= 0)
            entries[idx] = { id: "updateIndicator", enabled: false, zone: entries[idx].zone || "right" };
        else
            entries.push({ id: "updateIndicator", enabled: false, zone: "right" });
        GlobalConfig.bar.entries = entries;
        root.popouts.hasCurrent = false;
    }

    // Joins the sidebar like the other panel-sized popouts while it is open.
    width: Math.max(300 * scaleOffset, _isSidebarOpen ? (Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) * scaleOffset) - Tokens.padding.extraLargeIncreased : 0)
    implicitWidth: width
    spacing: Tokens.spacing.small * scaleOffset

    // Status card: state icon + summary + last/next check timings.
    StyledRect {
        Layout.fillWidth: true
        implicitHeight: statusLayout.implicitHeight + Tokens.padding.medium * 2 * root.scaleOffset
        radius: Tokens.rounding.medium * root.scaleOffset
        color: Colours.tPalette.m3surfaceContainer
        clip: true

        ColumnLayout {
            id: statusLayout

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.padding.medium * root.scaleOffset
            spacing: Tokens.spacing.small * root.scaleOffset

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.spacing.small * root.scaleOffset

                MaterialIcon {
                    text: root.statusIcon
                    color: root.hasUpdate ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                    fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                }

                StyledText {
                    Layout.fillWidth: true
                    text: root.statusText
                    color: Colours.palette.m3onSurface
                    wrapMode: Text.Wrap
                    font.weight: 600
                    font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.spacing.small * root.scaleOffset

                MaterialIcon {
                    text: "history"
                    color: Colours.palette.m3onSurfaceVariant
                    fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                }

                StyledText {
                    Layout.fillWidth: true
                    text: {
                        if (root.checking)
                            return qsTr("Checking…");
                        if (UpdateChecker.lastCheckMs <= 0)
                            return qsTr("Last check: not yet");
                        return qsTr("Last check %1 ago").arg(root.formatDuration(root.nowMs - UpdateChecker.lastCheckMs));
                    }
                    color: Colours.palette.m3onSurfaceVariant
                    font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                }
            }

            RowLayout {
                Layout.fillWidth: true
                visible: UpdateChecker.lastCheckMs > 0
                spacing: Tokens.spacing.small * root.scaleOffset

                MaterialIcon {
                    text: "schedule"
                    color: Colours.palette.m3onSurfaceVariant
                    fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                }

                StyledText {
                    Layout.fillWidth: true
                    text: qsTr("Next check in %1").arg(root.formatDuration(UpdateChecker.lastCheckMs + UpdateChecker.checkIntervalMs - root.nowMs))
                    color: Colours.palette.m3onSurfaceVariant
                    font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                }
            }
        }
    }

    StyledRect {
        Layout.fillWidth: true
        implicitHeight: actionsLayout.implicitHeight + Tokens.padding.medium * 2 * root.scaleOffset
        radius: Tokens.rounding.medium * root.scaleOffset
        color: Colours.tPalette.m3surfaceContainer
        clip: true

        ColumnLayout {
            id: actionsLayout

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: Tokens.padding.medium * root.scaleOffset
            spacing: Tokens.spacing.small * root.scaleOffset

            StyledRect {
                Layout.fillWidth: true
                implicitHeight: checkRow.implicitHeight
                radius: Tokens.rounding.full
                color: "transparent"

                StateLayer {
                    anchors.margins: -Tokens.padding.medium / 2 * root.scaleOffset
                    anchors.leftMargin: -Tokens.padding.medium * root.scaleOffset
                    anchors.rightMargin: -Tokens.padding.medium * root.scaleOffset

                    radius: parent.radius
                    enabled: !root.checking
                    onClicked: UpdateChecker.checkUpdates()
                }

                RowLayout {
                    id: checkRow

                    anchors.left: parent.left
                    anchors.right: parent.right
                    spacing: Tokens.spacing.small * root.scaleOffset

                    MaterialIcon {
                        text: root.checking ? "progress_activity" : "refresh"
                        color: Colours.palette.m3onSurfaceVariant
                        fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                    }

                    StyledText {
                        Layout.fillWidth: true
                        text: root.checking ? qsTr("Checking…") : qsTr("Check for updates")
                        color: Colours.palette.m3onSurface
                        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                    }
                }
            }

            StyledRect {
                Layout.fillWidth: true
                implicitHeight: openRow.implicitHeight
                radius: Tokens.rounding.full
                color: "transparent"

                StateLayer {
                    anchors.margins: -Tokens.padding.medium / 2 * root.scaleOffset
                    anchors.leftMargin: -Tokens.padding.medium * root.scaleOffset
                    anchors.rightMargin: -Tokens.padding.medium * root.scaleOffset

                    radius: parent.radius
                    onClicked: {
                        root.popouts.hasCurrent = false;
                        Hub.open("updates");
                    }
                }

                RowLayout {
                    id: openRow

                    anchors.left: parent.left
                    anchors.right: parent.right
                    spacing: Tokens.spacing.small * root.scaleOffset

                    MaterialIcon {
                        text: "update"
                        color: Colours.palette.m3onSurfaceVariant
                        fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                    }

                    StyledText {
                        Layout.fillWidth: true
                        text: qsTr("Open Updates")
                        color: Colours.palette.m3onSurface
                        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                    }
                }
            }

            StyledRect {
                Layout.fillWidth: true
                implicitHeight: hideRow.implicitHeight
                radius: Tokens.rounding.full
                color: "transparent"

                StateLayer {
                    anchors.margins: -Tokens.padding.medium / 2 * root.scaleOffset
                    anchors.leftMargin: -Tokens.padding.medium * root.scaleOffset
                    anchors.rightMargin: -Tokens.padding.medium * root.scaleOffset

                    radius: parent.radius
                    onClicked: root.hideIndicator()
                }

                RowLayout {
                    id: hideRow

                    anchors.left: parent.left
                    anchors.right: parent.right
                    spacing: Tokens.spacing.small * root.scaleOffset

                    MaterialIcon {
                        text: "visibility_off"
                        color: Colours.palette.m3onSurfaceVariant
                        fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                    }

                    StyledText {
                        Layout.fillWidth: true
                        text: qsTr("Hide from bar")
                        color: Colours.palette.m3onSurface
                        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                    }
                }
            }
        }
    }

    Timer {
        interval: 1000
        repeat: true
        running: true
        onTriggered: root.nowMs = Date.now()
    }
}
