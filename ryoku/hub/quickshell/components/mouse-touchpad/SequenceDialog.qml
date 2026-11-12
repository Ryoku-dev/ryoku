import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "buttons.js" as Buttons

// edits the sequence of keys one mouse button runs; nothing is set until Save
Dialog {
    id: dialog

    required property KeyCapture capture
    property var store: null
    property string subject: ""
    property var steps: []
    property int repeat: 1
    property bool cancelOnRelease: false
    property string problem: ""
    property string note: ""
    // the steps before a recording started, put back if it is cancelled
    property var beforeRecording: null
    property Item stepEditor: null
    // the step editor whose keys were just chosen, so Return there means Add
    property Item refocus: null

    signal saved(var target)

    function openWith(subject, current) {
        var t = current && current.kind === "sequence" ? current : { sequence: [], repeat: 1, cancelOnRelease: false };
        dialog.subject = subject;
        dialog.steps = (t.sequence || []).map(s => Buttons.cleanStep(s));
        dialog.repeat = t.repeat || 1;
        dialog.cancelOnRelease = t.cancelOnRelease === true;
        dialog.problem = "";
        dialog.note = "";
        dialog.open();
    }

    function target() {
        return { kind: "sequence", sequence: dialog.steps.map(s => Buttons.cleanStep(s)), repeat: dialog.repeat, cancelOnRelease: dialog.cancelOnRelease };
    }

    function setSteps(list) {
        dialog.steps = list.slice(0, 64);
        dialog.note = list.length > 64 ? I18n.tr("A sequence holds up to 64 steps; the rest were left out.") : "";
    }

    function record(opener) {
        dialog.beforeRecording = dialog.steps;
        dialog.steps = [];
        dialog.capture.begin("record", dialog.subject, opener);
    }

    function chooseKeys(editorItem, opener) {
        dialog.stepEditor = editorItem;
        dialog.capture.begin("step", I18n.tr("this step"), opener);
    }

    function tryIt() {
        dialog.problem = "";
        dialog.note = I18n.tr("Sending the sequence now.");
        if (!dialog.store)
            return;
        dialog.store.test(dialog.target(), function (ok, error) {
            if (!ok)
                console.warn("mouse.test failed:", error);
            dialog.note = ok ? I18n.tr("Sent. It went to the window that has focus.") : "";
            dialog.problem = ok ? "" : I18n.tr("The sequence did not run. Check the steps, then try again; if it still fails, run ryoku doctor in a terminal.");
        });
    }

    Connections {
        target: dialog.capture
        enabled: dialog.opened
        function onKeysPressed(keys, delayMs) {
            if (dialog.capture.mode === "step") {
                if (dialog.stepEditor) {
                    dialog.stepEditor.keys = keys;
                    dialog.refocus = dialog.stepEditor;
                }
                return;
            }
            if (dialog.capture.mode !== "record")
                return;
            var next = dialog.steps.slice();
            if (next.length > 0 && delayMs > 0)
                next.push({ kind: "delay", delayMs: delayMs });
            next.push({ kind: "tap", keys: keys });
            dialog.setSteps(next);
        }
        function onFinished(keep) {
            if (dialog.capture.mode === "record" && !keep && dialog.beforeRecording !== null)
                dialog.steps = dialog.beforeRecording;
            dialog.beforeRecording = null;
            dialog.stepEditor = null;
        }
        function onClosed() {
            if (dialog.refocus)
                dialog.refocus.focusKind();
            dialog.refocus = null;
        }
    }

    title: I18n.tr("Sequence for %1").arg(dialog.subject)
    dialogWidth: 600
    actions: [
        { text: I18n.tr("Cancel"), kind: "quiet" },
        { text: I18n.tr("Try it"), kind: "line", glyph: "play_arrow", enabled: dialog.steps.length > 0, closeOnTrigger: false },
        { text: I18n.tr("Save"), kind: "gold", enabled: dialog.steps.length > 0 }
    ]
    onActionTriggered: index => {
        if (index === 1)
            dialog.tryIt();
        else if (index === 2)
            dialog.saved(dialog.target());
    }

    Column {
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            width: parent.width
            text: I18n.tr("The button runs these steps in order without holding up the pointer. Record them as you type, or add them one by one.")
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Plaque {
            id: recordPlaque
            kind: "line"
            small: true
            glyph: "fiber_manual_record"
            text: I18n.tr("Record steps")
            onClicked: dialog.record(recordPlaque)
        }

        Flickable {
            id: stepsView
            width: parent.width
            height: Math.min(stepList.implicitHeight, Math.max(280, (dialog.parent ? dialog.parent.height : 800) - 470))
            contentWidth: width
            contentHeight: stepList.implicitHeight
            boundsBehavior: Flickable.StopAtBounds
            clip: true

            ListEditor {
                id: stepList
                width: stepsView.width - LithicTokens.space.s3
                model: dialog.steps
                plane: Lithic.stone4
                reorderable: true
                accessibleName: I18n.tr("Steps")
                titleOf: step => Buttons.stepText(step, I18n.tr)
                detailOf: step => ""
                addText: I18n.tr("Add a step")
                doneText: I18n.tr("Add")
                emptyText: I18n.tr("No steps yet.")
                editor: stepEditorComponent
                onAdded: item => dialog.setSteps(dialog.steps.concat([item]))
                onEdited: (index, item) => {
                    var next = dialog.steps.slice();
                    next[index] = item;
                    dialog.setSteps(next);
                }
                onRemoved: index => {
                    var next = dialog.steps.slice();
                    next.splice(index, 1);
                    dialog.setSteps(next);
                }
                onMoved: (from, to) => {
                    var next = dialog.steps.slice();
                    var step = next.splice(from, 1)[0];
                    next.splice(to, 0, step);
                    dialog.setSteps(next);
                }
            }
        }

        SettingRow {
            width: parent.width
            first: true
            plane: Lithic.stone4
            title: I18n.tr("Run it")
            detail: I18n.tr("How many times the whole sequence runs for one press")
            Stepper {
                from: 1
                to: 100
                stepSize: 1
                value: dialog.repeat
                unit: I18n.tr("times")
                editable: true
                accessibleName: I18n.tr("Times the sequence runs")
                onValueModified: dialog.repeat = value
            }
        }

        SettingRow {
            width: parent.width
            plane: Lithic.stone4
            title: I18n.tr("When you let go of the button")
            detail: I18n.tr("Stopping at once also lets go of any keys the sequence holds down")
            Segmented {
                model: [I18n.tr("Finish"), I18n.tr("Stop at once")]
                currentIndex: dialog.cancelOnRelease ? 1 : 0
                accessibleName: I18n.tr("When you let go of the button")
                onActivated: index => dialog.cancelOnRelease = index === 1
            }
        }

        Label {
            visible: dialog.note.length > 0
            width: parent.width
            text: dialog.note
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Label {
            visible: dialog.problem.length > 0
            width: parent.width
            text: dialog.problem
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Component {
        id: stepEditorComponent
        Column {
            id: editorRoot
            property var item
            property string kind: editorRoot.item && editorRoot.item.kind ? editorRoot.item.kind : "tap"
            property var keys: editorRoot.item && editorRoot.item.keys ? editorRoot.item.keys : []
            property int delayMs: editorRoot.item && editorRoot.item.delayMs ? editorRoot.item.delayMs : 100
            readonly property bool valid: editorRoot.kind === "delay" ? editorRoot.delayMs >= 10 : editorRoot.keys.length > 0
            readonly property var kinds: ["tap", "down", "up", "delay"]
            function value() {
                return Buttons.cleanStep({ kind: editorRoot.kind, keys: editorRoot.keys, delayMs: editorRoot.delayMs });
            }
            function focusKind() {
                kindPicker.forceActiveFocus(Qt.TabFocusReason);
            }
            spacing: LithicTokens.space.s3

            Segmented {
                id: kindPicker
                model: [I18n.tr("Press"), I18n.tr("Hold down"), I18n.tr("Let go"), I18n.tr("Wait")]
                currentIndex: Math.max(0, editorRoot.kinds.indexOf(editorRoot.kind))
                accessibleName: I18n.tr("Kind of step")
                onActivated: index => editorRoot.kind = editorRoot.kinds[index]
            }

            Row {
                visible: editorRoot.kind !== "delay"
                spacing: LithicTokens.space.s3
                Plaque {
                    id: keysPlaque
                    kind: "tonal"
                    small: true
                    glyph: "keyboard"
                    text: editorRoot.keys.length > 0 ? Buttons.keysText(editorRoot.keys) : I18n.tr("Choose keys")
                    accessibleName: I18n.tr("Choose the keys for this step")
                    onClicked: dialog.chooseKeys(editorRoot, keysPlaque)
                }
            }

            Stepper {
                visible: editorRoot.kind === "delay"
                from: 10
                to: 60000
                stepSize: 10
                value: editorRoot.delayMs
                unit: "ms"
                editable: true
                accessibleName: I18n.tr("Pause length")
                onValueModified: editorRoot.delayMs = value
            }
        }
    }
}
