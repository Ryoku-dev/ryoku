pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../shortcuts/binds.js" as Binds

// the import after a source was chosen: review, conflicts, preview, done
Column {
    id: root

    required property var flow
    property var hub: null
    // a shortcut field records; the page holds the window manager's keys meanwhile
    signal recording(Item field, bool on)
    // the steps closed while the keys were in them: the page puts focus on choosing a source
    signal leftSteps()
    // the way back to choosing a source once an import is done; a search for a source lands here
    readonly property Item another: anotherButton

    // a step or a job that hides the control the focus was on hands it to the next button,
    // marked when the keys had put it there
    property bool inside: false
    property bool wasInside: false
    property bool keyed: false
    function holds(item) {
        for (var it = item; it; it = it.parent)
            if (it === root)
                return true;
        return false;
    }
    function land() {
        if (root.flow.busy !== "")
            return;
        // a hidden control keeps the focus; one the window moved on lands on a container
        var f = root.Window.activeFocusItem;
        var fell = f && root.holds(f) ? !f.visible : root.wasInside && !root.inside && !(f && f.activeFocusOnTab);
        if (!fell)
            return;
        root.wasInside = false;
        if (root.flow.step === "source")
            root.leftSteps();
        else
            (root.flow.step !== "done" ? nextButton : root.flow.undone ? anotherButton : reloadButton)
                .forceActiveFocus(root.keyed ? Qt.TabFocusReason : Qt.OtherFocusReason);
    }
    Connections {
        target: root.Window.window
        function onActiveFocusItemChanged() {
            var f = root.Window.activeFocusItem;
            root.wasInside = root.inside;
            root.inside = root.holds(f);
            if (root.inside)
                root.keyed = f.visualFocus === true;
        }
    }
    Connections {
        target: root.flow
        function onStepChanged() { Qt.callLater(root.land); }
        function onBusyChanged() { Qt.callLater(root.land); }
    }

    readonly property var steps: root.flow.conflicts.length > 0 ? ["review", "resolve", "preview", "done"] : ["review", "preview", "done"]
    readonly property int stepIndex: Math.max(0, root.steps.indexOf(root.flow.step))
    readonly property string stepName: {
        switch (root.flow.step) {
        case "review": return I18n.tr("What comes over");
        case "resolve": return I18n.tr("Shortcuts that clash");
        case "preview": return I18n.tr("Before it applies");
        case "done": return root.flow.undone ? I18n.tr("Import undone") : I18n.tr("Imported");
        }
        return "";
    }

    spacing: LithicTokens.space.s3

    function tierNote(t) {
        if (t === "deep")
            return I18n.tr("Shortcuts, window rules and the settings Ryoku knows become Ryoku settings; the rest stays in the window manager's own include file.");
        if (t === "layer")
            return I18n.tr("Added to this app's own override file, which already wins over Ryoku's.");
        return I18n.tr("Placed in this app's override slot and used as it is.");
    }

    function count(n, one, many) {
        return (n === 1 ? I18n.tr(one) : I18n.tr(many)).replace("%1", n);
    }

    // what was found in an app, counted in the reader's words
    function found(app) {
        var n = { bind: 0, rule: 0, file: 0, setting: 0 };
        (app.items || []).forEach(it => {
            if (it.kind === "bind")
                n.bind++;
            else if (it.kind === "windowrule" || it.kind === "layerrule")
                n.rule++;
            else if (it.kind === "file")
                n.file++;
            else
                n.setting++;
        });
        return [n.bind > 0 ? root.count(n.bind, "%1 shortcut", "%1 shortcuts") : "",
                n.rule > 0 ? root.count(n.rule, "%1 window rule", "%1 window rules") : "",
                n.setting > 0 ? root.count(n.setting, "%1 setting", "%1 settings") : "",
                n.file > 0 ? root.count(n.file, "%1 file", "%1 files") : ""].filter(s => s.length > 0).join(", ");
    }

    function tilde(path) {
        var home = Quickshell.env("HOME") || "";
        return home.length > 0 && path.indexOf(home + "/") === 0 ? "~" + path.slice(home.length) : path;
    }

    Label {
        width: parent.width
        visible: root.flow.step !== "done"
        text: I18n.tr("Step %1 of %2: %3").replace("%1", root.stepIndex + 1).replace("%2", root.steps.length).replace("%3", root.stepName)
        typeRole: "label"
        inkRole: "fg2"
    }

    // review: each app found, on or off
    Slab {
        width: parent.width
        visible: root.flow.step === "review"
        Column {
            width: parent.width
            Label {
                width: parent.width
                visible: root.flow.apps.length === 0
                text: I18n.tr("Nothing that can be imported was found there. Try another folder, one app's folder, or a git address.")
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Repeater {
                model: root.flow.step === "review" ? root.flow.apps : []
                delegate: SettingRow {
                    id: appRow
                    required property var modelData
                    required property int index
                    readonly property int losses: (modelData.losses || []).length
                    width: parent.width
                    first: index === 0
                    activatable: true
                    title: modelData.name || modelData.id
                    detail: [modelData.error ? I18n.tr("Part of it could not be read: %1").replace("%1", modelData.error) : root.found(modelData),
                             root.tierNote(modelData.tier || "drop"),
                             appRow.losses > 0 ? root.count(appRow.losses, "%1 line stays in the app's own config", "%1 lines stay in the app's own config") : ""]
                        .filter(s => s.length > 0).join("\n")
                    onActivated: root.flow.setIncluded(modelData.id, !root.flow.included(modelData.id))
                    Toggle {
                        checked: root.flow.included(appRow.modelData.id)
                        Accessible.name: I18n.tr("Import %1").replace("%1", appRow.title)
                        onToggled: root.flow.setIncluded(appRow.modelData.id, checked)
                    }
                }
            }
        }
    }

    // resolve: every shortcut both sides want
    Label {
        width: parent.width
        visible: root.flow.step === "resolve"
        text: I18n.tr("Your configuration wants these shortcuts, and Ryoku already uses them. Keep Ryoku's, use yours (Ryoku lets go of its own), or move yours to other keys. Untouched ones keep Ryoku's.")
            + " " + I18n.tr("%1 of %2 settled.").replace("%1", root.flow.resolvedCount).replace("%2", root.flow.conflicts.length)
        typeRole: "body"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Slab {
        width: parent.width
        visible: root.flow.step === "resolve"
        Column {
            width: parent.width
            Repeater {
                model: root.flow.step === "resolve" ? root.flow.conflicts : []
                delegate: SettingRow {
                    id: clash
                    required property var modelData
                    required property int index
                    readonly property var decision: root.flow.decisions[modelData.norm]
                    readonly property int choice: clash.decision === "ryoku" ? 0 : clash.decision === "mine" ? 1 : clash.decision && clash.decision.remap !== undefined ? 2 : -1
                    width: parent.width
                    first: index === 0
                    title: Binds.toField(modelData.combo || "").split("+").join(" ")
                    detail: [I18n.tr("Ryoku: %1").replace("%1", modelData.ryoku.desc || I18n.tr("a shortcut it ships")),
                             I18n.tr("Yours: %1").replace("%1", modelData.mine.desc || modelData.mine.raw || ""),
                             modelData.kind === "duplicate" ? I18n.tr("Your configuration also uses it twice") : ""]
                        .filter(s => s.length > 0).join("\n")
                    Column {
                        spacing: LithicTokens.space.s2
                        Segmented {
                            model: [I18n.tr("Keep Ryoku's"), I18n.tr("Use mine"), I18n.tr("Move mine")]
                            currentIndex: clash.choice
                            accessibleName: I18n.tr("Shortcut %1").replace("%1", clash.title)
                            onActivated: index => {
                                if (index === 0)
                                    root.flow.decide(clash.modelData.norm, "ryoku");
                                else if (index === 1)
                                    root.flow.decide(clash.modelData.norm, "mine");
                                else {
                                    root.flow.decide(clash.modelData.norm, { remap: "" });
                                    Qt.callLater(() => moveTo.startRecording());
                                }
                            }
                        }
                        ChordField {
                            id: moveTo
                            visible: clash.choice === 2
                            width: parent.width
                            accessibleName: I18n.tr("New keys for %1").replace("%1", clash.title)
                            chord: clash.decision && clash.decision.remap ? Binds.toField(clash.decision.remap) : ""
                            conflict: root.flow.remapClashes(clash.modelData.norm) ? I18n.tr("These keys clash with another of your shortcuts") : ""
                            onRecordingChanged: root.recording(moveTo, recording)
                            onRecorded: chord => root.flow.decide(clash.modelData.norm, { remap: Binds.toStore(chord) })
                            onCleared: root.flow.decide(clash.modelData.norm, { remap: "" })
                        }
                    }
                }
            }
        }
    }
    Disclosure {
        width: parent.width
        visible: root.flow.step === "resolve" && root.flow.layered.length > 0
        text: root.count(root.flow.layered.length, "%1 other setting is added as it is", "%1 other settings are added as they are")
        Column {
            width: parent.width
            spacing: LithicTokens.space.s1
            Repeater {
                model: root.flow.step === "resolve" ? root.flow.layered : []
                delegate: Label {
                    required property var modelData
                    width: parent.width
                    text: modelData.app + ": " + modelData.raw
                    typeRole: "monoSm"
                    inkRole: "fg2"
                    elide: Text.ElideRight
                }
            }
        }
    }

    // preview: what applying does, and the backup
    Slab {
        width: parent.width
        visible: root.flow.step === "preview"
        Column {
            width: parent.width
            spacing: LithicTokens.space.s3
            Facts {
                width: parent.width
                facts: [
                    { label: I18n.tr("Apps"), value: root.flow.apps.filter(a => root.flow.included(a.id)).map(a => a.name || a.id).join(", ") || I18n.tr("None") },
                    { label: I18n.tr("Shortcuts added"), value: String(root.flow.forecast.binds) },
                    { label: I18n.tr("Window rules added"), value: String(root.flow.forecast.rules) },
                    { label: I18n.tr("Ryoku shortcuts let go"), value: String(root.flow.forecast.unbinds) }
                ]
            }
            Label {
                width: parent.width
                text: I18n.tr("Every file this touches is copied to ~/.config/ryoku/import-backups first, so the whole import can be undone afterwards.")
                typeRole: "detail"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }

    // done: the exact change, reload and undo
    Slab {
        width: parent.width
        visible: root.flow.step === "done"
        Column {
            width: parent.width
            spacing: LithicTokens.space.s3
            Label {
                width: parent.width
                text: root.flow.undone
                    ? I18n.tr("Everything the import touched is back from its backup.")
                    : I18n.tr("Your shortcuts are in Keyboard shortcuts and checked for clashes from now on; the rest is in place. Reload the desktop to see it all.")
                typeRole: "body"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Facts {
                width: parent.width
                visible: !root.flow.undone && root.flow.applied !== null
                facts: root.flow.applied ? [
                    { label: I18n.tr("Shortcuts added"), value: String(root.flow.applied.bindsIngested || 0) },
                    { label: I18n.tr("Window rules added"), value: String(root.flow.applied.rulesIngested || 0) },
                    { label: I18n.tr("Ryoku shortcuts let go"), value: String(root.flow.applied.unbinds || 0) },
                    { label: I18n.tr("Backup"), value: root.tilde(String(root.flow.applied.backupDir || "")), literal: true }
                ] : []
            }
            Disclosure {
                id: filesFold
                width: parent.width
                readonly property var files: root.flow.undone
                    ? (root.flow.undoneResult && root.flow.undoneResult.restored ? root.flow.undoneResult.restored : [])
                    : (root.flow.applied && root.flow.applied.filesWritten ? root.flow.applied.filesWritten : [])
                visible: files.length > 0
                text: root.flow.undone ? root.count(files.length, "%1 file restored", "%1 files restored")
                                       : root.count(files.length, "%1 file written", "%1 files written")
                Column {
                    width: parent.width
                    spacing: LithicTokens.space.s1
                    Repeater {
                        model: filesFold.files
                        delegate: Label {
                            required property var modelData
                            width: parent.width
                            text: String(modelData)
                            typeRole: "monoSm"
                            inkRole: "fg2"
                            elide: Text.ElideMiddle
                        }
                    }
                }
            }
        }
    }

    Row {
        spacing: LithicTokens.space.s2
        visible: root.flow.busy === ""
        Plaque {
            visible: root.flow.step === "review" || root.flow.step === "resolve" || root.flow.step === "preview"
            kind: "quiet"
            glyph: "arrow_back"
            text: root.flow.step === "review" ? I18n.tr("Start again") : I18n.tr("Back")
            onClicked: root.flow.back()
        }
        Plaque {
            id: nextButton
            visible: root.flow.step === "review" || root.flow.step === "resolve" || root.flow.step === "preview"
            enabled: root.flow.step !== "review" || root.flow.includedCount > 0
            kind: "gold"
            text: root.flow.step === "preview" ? I18n.tr("Apply") : root.flow.step === "resolve" ? I18n.tr("Preview") : I18n.tr("Continue")
            onClicked: root.flow.next()
        }
        Plaque {
            id: reloadButton
            visible: root.flow.step === "done" && !root.flow.undone
            kind: "gold"
            glyph: "refresh"
            text: I18n.tr("Reload the desktop")
            onClicked: root.flow.reloadDesktop()
        }
        Plaque {
            visible: root.flow.step === "done" && !root.flow.undone
            kind: "line"
            glyph: "undo"
            text: I18n.tr("Undo the import")
            onClicked: root.flow.undo()
        }
        Plaque {
            id: anotherButton
            visible: root.flow.step === "done"
            kind: "quiet"
            text: I18n.tr("Import another")
            onClicked: root.flow.restart()
        }
    }
}
