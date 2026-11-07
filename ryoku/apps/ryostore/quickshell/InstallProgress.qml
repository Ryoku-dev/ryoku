import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: root

    property string stage: ""
    property real done: 0
    property real total: 0
    property string category: ""
    property bool installed: false
    property bool active: false
    property bool reducedMotion: false
    property real sweepPosition: 0.5

    readonly property bool showingProgress: active && stage !== ""
    readonly property bool determinate: showingProgress && total > 0
    readonly property real fraction: determinate ? Math.max(0, Math.min(1, done / total)) : 0
    readonly property string activationHint: {
        if (!installed)
            return "";
        if (category === "rices")
            return I18n.tr("Switch it in Ryoku Hub → Appearance → Rices.");
        if (category === "decors")
            return I18n.tr("Choose it in Ryoku Hub → Profile → Hero.");
        if (category === "barstyles" || category === "subshells")
            return I18n.tr("Press Apply to wear it, or switch it in Ryoku Hub.");
        return "";
    }
    readonly property bool showingHint: !active && activationHint !== ""
    readonly property string stageLabel: {
        if (stage === "download") return I18n.tr("DOWNLOADING");
        if (stage === "extract") return I18n.tr("EXTRACTING");
        if (stage === "verify") return I18n.tr("VERIFYING");
        if (stage === "apply") return I18n.tr("APPLYING");
        return "";
    }
    readonly property string instruction: {
        if (stage === "download")
            return I18n.tr("Downloading — you can keep browsing; the store installs in the background.");
        if (stage === "extract")
            return I18n.tr("Extracting — preparing the downloaded files.");
        if (stage === "verify")
            return I18n.tr("Verifying — checking the install before it is applied.");
        if (stage === "apply" && (category === "barstyles" || category === "subshells"))
            return I18n.tr("Applying — your bar may reload when done.");
        if (stage === "apply")
            return I18n.tr("Applying — finishing the install.");
        return "";
    }
    readonly property string progressText: {
        if (!determinate) {
            if (done <= 0)
                return I18n.tr("TOTAL UNKNOWN");
            var current = stage === "download"
                    ? byteAmount(done)
                    : Math.round(done) + " " + (stage === "verify" ? I18n.tr("checks") : I18n.tr("files"));
            return current + "  ·  " + I18n.tr("TOTAL UNKNOWN");
        }
        var amount = stage === "download"
                ? bytePair(done, total)
                : countPair(done, total, stage);
        return amount + "  ·  " + Math.floor(fraction * 100) + "%";
    }

    function bytePair(current, maximum) {
        var unit = I18n.tr("B");
        var divisor = 1;
        if (maximum >= 1073741824) {
            unit = I18n.tr("GB");
            divisor = 1073741824;
        } else if (maximum >= 1048576) {
            unit = I18n.tr("MB");
            divisor = 1048576;
        } else if (maximum >= 1024) {
            unit = I18n.tr("KB");
            divisor = 1024;
        }
        if (divisor === 1)
            return Math.round(current) + " / " + Math.round(maximum) + " " + unit;
        return (current / divisor).toFixed(1) + " / "
                + (maximum / divisor).toFixed(1) + " " + unit;
    }

    function byteAmount(value) {
        if (value >= 1073741824)
            return (value / 1073741824).toFixed(1) + " " + I18n.tr("GB");
        if (value >= 1048576)
            return (value / 1048576).toFixed(1) + " " + I18n.tr("MB");
        if (value >= 1024)
            return (value / 1024).toFixed(1) + " " + I18n.tr("KB");
        return Math.round(value) + " " + I18n.tr("B");
    }

    function countPair(current, maximum, currentStage) {
        var noun = currentStage === "verify" ? I18n.tr("checks") : I18n.tr("files");
        return Math.round(current) + " / " + Math.round(maximum) + " " + noun;
    }

    visible: showingProgress || showingHint
    implicitHeight: visible ? content.implicitHeight : 0
    height: implicitHeight

    onShowingProgressChanged: if (!showingProgress) sweepPosition = 0.5
    onReducedMotionChanged: if (reducedMotion) sweepPosition = 0.5

    Column {
        id: content
        width: parent.width
        spacing: Tokens.s2

        Item {
            width: parent.width
            height: Math.max(progressStage.implicitHeight, progressAmount.implicitHeight)
            visible: root.showingProgress

            Text {
                id: progressStage
                anchors {
                    left: parent.left
                    right: progressAmount.left
                    rightMargin: Tokens.s2
                    verticalCenter: parent.verticalCenter
                }
                text: root.stageLabel
                color: Tokens.ink
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.weight: Font.Medium
                font.letterSpacing: Tokens.trackLabel
                elide: Text.ElideRight
            }

            Text {
                id: progressAmount
                anchors {
                    right: parent.right
                    verticalCenter: parent.verticalCenter
                }
                width: Math.min(implicitWidth, parent.width * 0.68)
                text: root.progressText
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
                horizontalAlignment: Text.AlignRight
                elide: Text.ElideLeft
            }
        }

        Rectangle {
            id: track
            width: parent.width
            height: Math.max(3, Tokens.border * 3)
            visible: root.showingProgress
            color: Tokens.lineSoft
            clip: true

            Rectangle {
                width: root.determinate
                        ? track.width * root.fraction
                        : Math.max(Tokens.s6, track.width * 0.22)
                height: parent.height
                x: root.determinate
                   ? 0
                   : (root.reducedMotion
                      ? (track.width - width) / 2
                      : root.sweepPosition * (track.width + width) - width)
                color: Tokens.bone
            }
        }

        Text {
            width: parent.width
            visible: root.showingProgress
            text: root.instruction
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.Wrap
            textFormat: Text.PlainText
        }

        Text {
            width: parent.width
            visible: root.showingHint
            text: root.activationHint
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.Wrap
            textFormat: Text.PlainText
        }
    }

    NumberAnimation {
        target: root
        property: "sweepPosition"
        from: 0
        to: 1
        duration: 1100
        easing.type: Easing.InOutSine
        loops: Animation.Infinite
        running: root.showingProgress && !root.determinate && !root.reducedMotion && root.visible
    }
}
