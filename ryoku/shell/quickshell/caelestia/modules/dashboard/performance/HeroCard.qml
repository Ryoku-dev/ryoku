import QtQuick
import QtQuick.Layouts
import M3Shapes
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services

StyledRect {
    id: root

    required property string icon
    required property string label
    required property string subLabel
    required property color accent
    required property real usage
    required property real temperature

    property var monitor: null
    property real frequency: 0
    property var coreLoads: []
    property int coreCount: 0
    property var history: []
    // the Controls page can hide the temperature reading
    property bool showTemperature: true
    property bool vramAvailable: false
    property real vramUsedGiB: 0
    property real vramTotalGiB: 0
    property int usageBand: 0

    onHistoryChanged: historyCanvas.requestPaint()
    onAccentChanged: historyCanvas.requestPaint()

    function bandFor(usage: real): int {
        const thresholds = [0.4, 0.8];
        let band = 0;
        while (band < thresholds.length && usage >= thresholds[band])
            ++band;
        return band;
    }

    onUsageChanged: {
        if (isNaN(root.usage))
            return;
        if (bandFor(root.usage) === root.usageBand) {
            usageBandTimer.stop();
            return;
        }
        usageBandTimer.restart();
    }

    color: Colours.tPalette.m3surfaceContainer
    radius: Tokens.rounding.extraLarge

    implicitWidth: Tokens.sizes.dashboard.perfHeroCardWidth
    implicitHeight: Math.max(tempProg.implicitHeight + detailsRow.implicitHeight + Tokens.spacing.large, usageShape.implicitHeight + usageLabel.implicitHeight) + Tokens.padding.large * 2

    Timer {
        id: usageBandTimer

        interval: 2500
        onTriggered: root.usageBand = root.bandFor(root.usage)
    }

    CircularProgress {
        id: tempProg

        anchors.left: parent.left
        anchors.top: parent.top
        anchors.margins: Tokens.padding.large

        fgColour: root.accent

        spacing: Tokens.spacing.extraSmall
        strokeWidth: Tokens.padding.extraSmall
        implicitSize: Math.max(icon.implicitWidth, icon.implicitHeight) + Tokens.padding.medium * 2
        value: root.usage

        Behavior on clampedVal {
            Anim {}
        }

        MaterialIcon {
            id: icon

            anchors.centerIn: parent
            text: root.icon
            color: root.accent
            fontStyle: Tokens.font.icon.medium
        }
    }

    ColumnLayout {
        anchors.left: tempProg.right
        anchors.right: usageShape.left
        anchors.verticalCenter: tempProg.verticalCenter
        anchors.margins: Tokens.spacing.large
        spacing: Tokens.spacing.extraSmall

        StyledText {
            text: root.label
            font: Tokens.font.title.medium
            color: root.accent
        }

        StyledText {
            Layout.fillWidth: true
            text: root.subLabel
            font: Tokens.font.body.small
            color: Colours.palette.m3onSurfaceVariant
            elide: Text.ElideRight
        }
    }

    ColumnLayout {
        id: detailsRow

        anchors.left: parent.left
        anchors.right: usageShape.left
        anchors.bottom: parent.bottom
        anchors.margins: Tokens.padding.largeIncreased
        spacing: Tokens.spacing.extraSmall

        RowLayout {
            Layout.fillWidth: true
            Layout.leftMargin: -Tokens.padding.extraSmall
            spacing: Tokens.spacing.extraSmall

            MaterialIcon {
                visible: root.showTemperature
                Layout.topMargin: Math.round(fontInfo.pointSize * 0.08)
                text: root.temperature > 90 ? "thermometer_alert" : "thermometer"
                color: root.temperature > 90 ? Colours.palette.m3error : root.accent
                fontStyle: Tokens.font.icon.medium
                fill: 1
            }

            StyledText {
                visible: root.showTemperature
                text: Units.formatSensorTemp(root.temperature)
                font: Tokens.font.body.builders.medium.build()
            }

            Item {
                visible: root.frequency > 0
                Layout.fillWidth: true
            }

            StyledText {
                visible: root.frequency > 0
                text: `${root.frequency.toFixed(2)} GHz`
                font: Tokens.font.body.small
                color: Colours.palette.m3onSurfaceVariant
            }
        }

        StyledProgressBar {
            visible: root.showTemperature
            Layout.fillWidth: true
            value: root.temperature / 100
            implicitHeight: Tokens.padding.small
            fgColour: root.accent
            indeterminate: isNaN(root.usage) || isNaN(root.temperature)
        }

        Canvas {
            id: historyCanvas

            visible: root.history.length > 0
            Layout.fillWidth: true
            Layout.topMargin: Tokens.spacing.extraSmall
            implicitHeight: 28

            onWidthChanged: requestPaint()
            onHeightChanged: requestPaint()
            onPaint: {
                const ctx = getContext("2d");
                ctx.reset();
                const pts = root.history;
                if (width <= 0 || height <= 0)
                    return;
                if (pts.length < 2) {
                    ctx.strokeStyle = Colours.palette.m3secondaryContainer;
                    ctx.lineWidth = 1.5;
                    ctx.beginPath();
                    ctx.moveTo(0, height - 2);
                    ctx.lineTo(width, height - 2);
                    ctx.stroke();
                    return;
                }
                const cap = 60;
                const offset = Math.max(0, cap - pts.length);
                ctx.beginPath();
                for (let i = 0; i < pts.length; ++i) {
                    const x = ((offset + i) / (cap - 1)) * width;
                    const y = height - Math.max(0, Math.min(1, pts[i])) * (height - 4) - 2;
                    if (i === 0)
                        ctx.moveTo(x, y);
                    else
                        ctx.lineTo(x, y);
                }
                ctx.strokeStyle = root.accent;
                ctx.lineWidth = 1.5;
                ctx.lineJoin = "round";
                ctx.lineCap = "round";
                ctx.stroke();

                ctx.lineTo(width, height);
                ctx.lineTo((offset / (cap - 1)) * width, height);
                ctx.closePath();
                ctx.fillStyle = Qt.alpha(root.accent, 0.2);
                ctx.fill();
            }
        }

        RowLayout {
            visible: root.coreCount > 0 && root.coreLoads.length > 0
            Layout.fillWidth: true
            Layout.topMargin: Tokens.spacing.extraSmall
            implicitHeight: 22
            spacing: 2

            Repeater {
                model: root.coreLoads

                StyledRect {
                    id: coreTrack

                    required property var modelData
                    readonly property real frac: isFinite(modelData) ? Math.max(0, Math.min(1, Number(modelData) / 100)) : 0

                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    radius: Tokens.rounding.extraSmall
                    color: Colours.palette.m3secondaryContainer

                    StyledRect {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.bottom: parent.bottom
                        implicitHeight: Math.max(radius * 2, parent.height * coreTrack.frac)
                        radius: parent.radius
                        color: coreTrack.frac >= 0.85 ? Colours.palette.m3error : root.accent

                        Behavior on implicitHeight {
                            Anim {}
                        }
                    }
                }
            }
        }

        RowLayout {
            visible: root.vramAvailable && root.vramTotalGiB > 0
            Layout.fillWidth: true
            Layout.topMargin: Tokens.spacing.extraSmall
            spacing: Tokens.spacing.extraSmall

            StyledText {
                text: qsTr("VRAM")
                font: Tokens.font.body.small
                color: Colours.palette.m3onSurfaceVariant
            }

            Item {
                Layout.fillWidth: true
            }

            StyledText {
                text: Units.formatKibUsage(root.vramUsedGiB * 1024 * 1024, root.vramTotalGiB * 1024 * 1024)
                font: Tokens.font.body.small
            }
        }

        StyledProgressBar {
            visible: root.vramAvailable && root.vramTotalGiB > 0
            Layout.fillWidth: true
            value: root.vramTotalGiB > 0 ? root.vramUsedGiB / root.vramTotalGiB : 0
            implicitHeight: Tokens.padding.small
            fgColour: root.accent
        }
    }

    MaterialShape {
        id: usageShape

        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: Tokens.padding.medium

        implicitSize: Tokens.sizes.dashboard.perfUsageShapeSize
        color: Colours.palette.m3secondaryContainer
        shape: [MaterialShape.Cookie4Sided, MaterialShape.Sunny, MaterialShape.SoftBurst][root.usageBand]

        Behavior on color {
            CAnim {}
        }

        StyledText {
            id: usageLabel

            anchors.bottom: parent.top
            anchors.horizontalCenter: parent.horizontalCenter

            text: qsTr("Usage")
            color: Colours.palette.m3onSurfaceVariant
            font: Tokens.font.body.small
        }

        StyledText {
            anchors.centerIn: parent
            text: isNaN(root.usage) ? "...%" : Math.round(root.usage * 100) + "%"
            color: root.accent
            font: Tokens.font.headline.builders.small.width(50).build()
        }
    }
}
