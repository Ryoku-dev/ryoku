import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "../../Combos.js" as Combos
import "buttons.js" as Buttons

// listens for the next mouse button or key combination. Modes: "button" waits
// for a press on a mouse, "chord" and "step" take one key combination, "record"
// takes key combinations in order with the pauses between them
Dialog {
    id: capture

    property string mode: "chord"
    property string subject: ""
    property string forming: ""
    property string problem: ""
    property int recorded: 0
    property string modifierOnly: ""
    property bool resolved: false

    // keys: ["ctrl", "c"]; delayMs: the pause before them, for "record"
    signal keysPressed(var keys, int delayMs)
    signal finished(bool keep)

    property double lastAt: 0

    // opener takes focus back when the capture closes
    function begin(mode, subject, opener) {
        capture.mode = mode;
        capture.subject = subject;
        capture.forming = "";
        capture.problem = "";
        capture.recorded = 0;
        capture.modifierOnly = "";
        capture.resolved = false;
        capture.lastAt = Date.now();
        capture.opener = opener || null;
        capture.open();
    }

    // a found mouse button goes straight on to its keys, in the same dialog
    function switchTo(mode, subject) {
        capture.mode = mode;
        capture.subject = subject;
        capture.forming = "";
        capture.problem = "";
        idle.restart();
        keyTarget.forceActiveFocus();
    }

    function end(keep) {
        if (capture.resolved)
            return;
        capture.resolved = true;
        idle.stop();
        capture.close();
        capture.finished(keep);
    }

    function accept(chord) {
        capture.modifierOnly = "";
        var keys = Buttons.chordToKeys(chord);
        if (keys === null) {
            capture.problem = I18n.tr("A mouse button cannot send %1. Use letters, digits, F1 to F12, arrows, editing keys and Ctrl, Alt, Shift or Super.").arg(chord);
            return;
        }
        capture.problem = "";
        var now = Date.now();
        var pause = capture.recorded > 0 ? Math.max(1, Math.min(60000, Math.round(now - capture.lastAt))) : 0;
        capture.lastAt = now;
        capture.recorded++;
        capture.keysPressed(keys, pause);
        if (capture.mode === "record")
            idle.restart();
        else
            capture.end(true);
    }

    function modifierToken(key) {
        if (key === Qt.Key_Shift) return "SHIFT";
        if (key === Qt.Key_Control) return "CTRL";
        if (key === Qt.Key_Alt) return "ALT";
        if (key === Qt.Key_Meta || key === Qt.Key_Super_L || key === Qt.Key_Super_R) return "SUPER";
        return "";
    }

    title: capture.mode === "button" ? I18n.tr("Press a button on your mouse")
        : capture.mode === "record" ? I18n.tr("Type the keys in order")
        : I18n.tr("Press the keys for %1").arg(capture.subject)
    dialogWidth: 440
    // escape is consumed here; the popup's own close lets the press fall through to whatever takes focus next
    closePolicy: T.Popup.NoAutoClose
    actions: capture.mode === "record"
        ? [{ text: I18n.tr("Cancel"), kind: "quiet", closeOnTrigger: false }, { text: I18n.tr("Done"), kind: "gold", closeOnTrigger: false }]
        : [{ text: I18n.tr("Cancel"), kind: "quiet", closeOnTrigger: false }]
    onActionTriggered: index => capture.end(capture.mode === "record" && index === 1)
    onOpened: {
        keyTarget.forceActiveFocus();
        idle.restart();
    }
    // a recording keeps what it has when the dialog closes, anything else stops
    onClosed: capture.end(capture.mode === "record")

    Shortcut {
        sequences: [StandardKey.Cancel]
        enabled: capture.opened
        onActivated: capture.end(capture.mode === "record")
    }

    Timer {
        id: idle
        interval: capture.mode === "button" ? 30000 : 12000
        onTriggered: capture.end(capture.mode === "record")
    }

    Column {
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            width: parent.width
            text: capture.mode === "button" ? I18n.tr("Click the side, thumb or extra button you want to set. The usual left, right and wheel clicks are left alone.")
                : capture.mode === "record" ? I18n.tr("Each key combination becomes a step, and the pauses between them are kept. Press Done when you have finished.")
                : I18n.tr("Hold Ctrl, Alt, Shift or Super if you need them, then press the key. Esc stops.")
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Item {
            id: keyTarget
            width: parent.width
            height: Math.max(LithicTokens.size.factsRowHeight, shown.implicitHeight)
            focus: true
            Accessible.role: Accessible.StaticText
            Accessible.name: capture.title

            Label {
                id: shown
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width
                text: capture.mode === "record"
                    ? (capture.forming || (capture.recorded === 1 ? I18n.tr("1 step so far") : I18n.tr("%1 steps so far").arg(capture.recorded)))
                    : capture.mode === "button" ? I18n.tr("Waiting for a click")
                    : (capture.forming || I18n.tr("Waiting for keys"))
                typeRole: "mono"
                ink: capture.forming ? Lithic.fg : Lithic.fg3
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Keys.onPressed: event => {
                event.accepted = true;
                if (event.key === Qt.Key_Escape) {
                    capture.end(capture.mode === "record");
                    return;
                }
                if (event.isAutoRepeat || capture.mode === "button")
                    return;
                idle.restart();
                capture.forming = Combos.formingChord(event);
                var chord = Combos.chordFrom(event);
                if (chord === "") {
                    var m = capture.modifierToken(event.key);
                    // on wayland the press of a lone modifier carries no modifier state yet
                    capture.modifierOnly = (capture.mode === "step" || capture.mode === "record") && m !== "" && (capture.forming === m || capture.forming === "") ? m : "";
                    if (capture.modifierOnly !== "")
                        capture.forming = m.charAt(0) + m.slice(1).toLowerCase();
                    return;
                }
                var chosen = Buttons.chordToKeys(chord);
                capture.forming = chosen ? Buttons.keysText(chosen) : chord.split(" + ").join("+");
                capture.accept(chord);
            }
            Keys.onReleased: event => {
                if (event.isAutoRepeat)
                    return;
                event.accepted = true;
                var m = capture.modifierToken(event.key);
                if (m !== "" && m === capture.modifierOnly) {
                    capture.forming = m.charAt(0) + m.slice(1).toLowerCase();
                    capture.accept(m);
                }
            }
        }

        Label {
            visible: capture.problem.length > 0
            width: parent.width
            text: capture.problem
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }
}
