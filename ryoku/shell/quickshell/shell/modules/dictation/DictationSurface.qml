import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services

// Voice typing, while it is happening.
//
// This is a shell-owned popup, not a frame popout: it follows the `dictation`
// bus topic, so it looks and behaves the same under every bar style, and the
// daemon never has to know which one is running. It sits at bottom-centre,
// where the hands already are, takes no keyboard focus, and disappears the
// moment the topic goes quiet — an idle desktop pays nothing for it.
//
// Lithic: one cut slab, the state as a flake of mineral, the microphone as an
// engraved level, progress as a blade. Colour is structural only — accent while
// it listens, muted while it thinks — and there is no glow or gradient anywhere.
PanelWindow {
    id: root

    // The topic frame, re-read as it changes.
    readonly property var frame: Dictation.frame
    readonly property var levels: Dictation.levels
    readonly property real drift: Dictation.drift
    readonly property string state: Dictation.state
    readonly property bool live: Dictation.live
    readonly property bool busy: Dictation.busy
    readonly property bool loading: Dictation.loading
    readonly property bool failed: Dictation.failed
    readonly property bool showingOff: Dictation.showingOff
    readonly property string attentionReason: Dictation.attentionReason

    readonly property real s: Tokens.uiScaleFor(root.screen ? root.screen.name : "")
    readonly property bool offNote: showingOff && !busy

    visible: true

    // on the screen the keyboard is on, at the bottom edge
    screen: ShellState.screens.length > 0
            && ShellState.screens.find(scr => scr.name === Wm.focusedOutput)
                ? ShellState.screens.find(scr => scr.name === Wm.focusedOutput)
                : ShellState.primary
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-dictation"
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
    WlrLayershell.exclusionMode: ExclusionMode.Ignore

    // a bottom strip, with the card centred inside it: layer windows anchor by
    // edge, so centring is the card's own business. The mask keeps everything
    // outside the card click-through.
    anchors { bottom: true; left: true; right: true }

    readonly property real cardWidth: Math.round(380 * s)
    readonly property real cardHeight: Math.round(offNote ? 62 * s : 132 * s)

    implicitWidth: root.screen && root.screen.geometry
                   ? root.screen.geometry.width : Math.round(1280 * s)
    implicitHeight: root.cardHeight + Math.round(18 * s)

    mask: Region { item: card }

    Item {
        id: card
        objectName: "dictationCard"
        width: root.cardWidth
        height: root.cardHeight
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Math.round(10 * root.s)

        // it lifts and settles on the one curve; nothing moves at rest
        SequentialAnimation on scale {
            id: reveal
            running: false
            loops: 1
            PropertyAction { property: "scale"; value: 0.965 }
            NumberAnimation { property: "scale"; to: 1.0; duration: Motion.morph; easing.type: Easing.OutCubic }
        }

        Connections {
            target: Dictation
            function onLiveChanged() { if (Dictation.live) reveal.restart(); }
            function onAttentionReasonChanged() { if (Dictation.attentionReason !== "") reveal.restart(); }
        }

        // the slab: one plane, cut corners, hairline
        Canvas {
            id: slab
            anchors.fill: parent
            onPaint: {
                const ctx = getContext("2d");
                ctx.reset();
                const w = width, h = height;
                const cut = Math.round(14 * root.s);
                const bevel = Math.round(6 * root.s);
                ctx.beginPath();
                ctx.moveTo(cut, 0);
                ctx.lineTo(w - bevel, 0);
                ctx.lineTo(w, bevel);
                ctx.lineTo(w, h - cut);
                ctx.lineTo(w - cut, h);
                ctx.lineTo(bevel, h);
                ctx.lineTo(0, h - bevel);
                ctx.lineTo(0, cut);
                ctx.closePath();
                ctx.fillStyle = Theme.surfaceContainerHigh;
                ctx.fill();
                ctx.strokeStyle = Theme.outlineVariant;
                ctx.lineWidth = Math.max(1, 1 * root.s);
                ctx.stroke();
            }
            Connections {
                target: Theme
                function onSurfaceContainerHighChanged() { slab.requestPaint(); }
                function onOutlineVariantChanged() { slab.requestPaint(); }
            }
            Connections {
                target: root
                function onSChanged() { slab.requestPaint(); }
            }
        }

        Column {
            anchors { fill: parent; margins: Math.round(16 * root.s) }
            spacing: Math.round(10 * root.s)

            Row {
                spacing: Math.round(11 * root.s)
                anchors.horizontalCenter: parent.horizontalCenter

                StateFlake {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.round(15 * root.s)
                    height: Math.round(15 * root.s)
                    mineral: root.busy || root.offNote || root.failed
                             ? Theme.ink(Theme.surfaceContainerHighest) : Theme.primary
                    fraction: root.offNote || root.failed ? 0
                            : root.loading ? 0.4
                            : root.busy ? 0.25
                            : root.live ? 1 : 0
                    outline: Theme.outlineVariant
                }

                Column {
                    spacing: Math.round(3 * root.s)
                    anchors.verticalCenter: parent.verticalCenter

                    Text {
                        text: root.offNote
                              ? I18n.tr("Dictation is off")
                              : (root.loading ? I18n.tr("Loading model")
                                 : root.failed ? I18n.tr("Model failed to load")
                                 : root.busy ? I18n.tr("Transcribing")
                                 : I18n.tr("Listening"))
                        color: root.offNote ? Theme.ink(Theme.surfaceContainerHighest)
                               : root.busy ? Theme.ink(Theme.surfaceContainerHighest)
                               : Theme.primary
                        font.family: Theme.fontPrimaryFamily
                        font.pixelSize: Math.round(13 * root.s)
                        font.weight: Font.DemiBold
                        font.letterSpacing: 0.2
                    }

                    Text {
                        visible: !root.offNote
                        text: root.loading ? (Dictation.modelName || "")
                              : root.failed ? I18n.tr("Open Ryoku Settings and pick a model again")
                              : root.busy ? I18n.tr("Hold on a moment")
                              : I18n.tr("Speak — press Super+` again to stop")
                        color: Theme.ink(Theme.surfaceContainerHighest)
                        font.family: Theme.fontPrimaryFamily
                        font.pixelSize: Math.round(11 * root.s)
                        elide: Text.ElideRight
                        width: Math.min(implicitWidth, root.cardWidth - Math.round(70 * root.s))
                    }
                }
            }

            // the microphone, engraved — idle when nothing is being said
            EngravedLevel {
                visible: !root.offNote
                width: parent.width
                height: Math.round(44 * root.s)
                levels: root.levels
                drift: root.drift
                live: root.live
                ink: Theme.ink(Theme.surfaceContainerHighest)
                vein: root.live ? Theme.primary : Theme.outlineVariant
                scale: root.s
            }

            // the recording against the automatic stop
            CutBlade {
                visible: !root.offNote && root.state === "recording"
                width: parent.width
                height: Math.round(5 * root.s)
                fraction: root.bladeFraction
                track: Theme.outline
                fill: Theme.ink(Theme.surfaceContainerLow)
            }
        }
    }

    // seconds elapsed against the daemon's own automatic stop
    readonly property real bladeFraction: {
        if (state !== "recording" || frame.recordingAtMs <= 0 || frame.maxDuration <= 0)
            return 0;
        const elapsed = Date.now() - frame.recordingAtMs;
        return Math.max(0, Math.min(1, elapsed / (frame.maxDuration * 1000)));
    }
}
