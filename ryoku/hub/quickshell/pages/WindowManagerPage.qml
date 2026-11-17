pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import Quickshell.Io
import ".."
import "../Singletons"

// Window Manager (COMPOSITOR). Every window-manager setting on one page: the
// neutral window rows the Hub ships (shape, gaps, borders, motion) and the
// exclusives the active provider ships (ProviderSchema), so a new compositor's
// knobs arrive with its package and never a Hub edit. It shows only the running
// provider: Save applies its settings now and keeps them for the next session;
// the other compositor's store is unreachable from here, so there is nothing to
// edit for it. The head names the compositor; the switch to another lives here
// too, beside the honest list of what this compositor cannot do (the losses
// `apply --preview` reports, the same reasons that keep a hidden row from reading
// as a bug). Every setting rides the shared store (hub.hyprVal/hyprEdit) and the
// shared schema renderer, so a capability the compositor lacks drops its rows
// rather than showing dead knobs.
Item {
    id: pg
    property var hub

    function cap(s) { s = String(s || ""); return s.length ? s.charAt(0).toUpperCase() + s.slice(1) : s; }
    readonly property string providerName: Settings.provider
    readonly property string pTitle: pg.providerName !== "" ? pg.cap(pg.providerName) : I18n.tr("Window Manager")
    readonly property string pEyebrow: I18n.tr("COMPOSITOR")
    readonly property string pBlurb: I18n.tr("How windows look and behave here. Save applies changes now.")

    function hv(path) { return pg.hub ? pg.hub.hyprVal(path) : undefined }
    function cv(path) { return pg.hub ? pg.hub.hyprCommittedVal(path) : undefined }

    // Every window-manager setting on one page: the neutral window rows the Hub
    // ships followed by the active provider's exclusives. effectiveRows is the
    // one source the rail gate reads too, so the page and the gate never diverge.
    // ProviderSchema.revision is read so the list rebuilds when the fetch lands.
    readonly property var rows: {
        ProviderSchema.revision;
        return pg.hub ? pg.hub.effectiveRows("windowmanager") : ProviderSchema.rowsFor("windowmanager");
    }

    // per-row visibility for the neutral rows: a dependent stays hidden until its
    // parent toggle is on. Provider rows carry their own `when` gate through the
    // sheet, so their keys are not named here and pass straight through.
    function gateOk(key, d) {
        switch (key) {
        case "desktop.appearance.dimStrength": return d["desktop.appearance.dimInactive"] === true;
        case "desktop.appearance.wobblyWindows": case "desktop.appearance.windowStyle": return d["desktop.appearance.animations"] === true;
        case "desktop.appearance.glowRange": case "desktop.appearance.glowColor": return d["desktop.appearance.glowEnabled"] === true;
        case "desktop.appearance.borderAngleSpeed": return d["desktop.appearance.animatedBorder"] === true;
        case "desktop.appearance.blurContrast": case "desktop.appearance.blurBrightness": case "desktop.appearance.blurSpecial":
        case "desktop.appearance.blurPopups": case "desktop.appearance.blurIgnoreOpacity": case "desktop.appearance.blurNewOptimizations":
        case "desktop.appearance.blurVibrancyDarkness":
            return d["desktop.appearance.blurEnabled"] === true;
        case "desktop.appearance.shadowSharp": case "desktop.appearance.shadowScale": case "desktop.appearance.shadowColor":
        case "desktop.appearance.shadowSpread": case "desktop.appearance.shadowOffsetX": case "desktop.appearance.shadowOffsetY":
            return d["desktop.appearance.shadowEnabled"] === true;
        }
        return true;
    }

    // draft/committed are flat maps off the store (dotted keys), the shape the
    // settings sheet reads. draft depends on hyprVal, so an edit rebuilds it.
    readonly property var draft: {
        var d = {};
        if (pg.hub)
            for (var i = 0; i < pg.rows.length; i++) { var k = pg.rows[i].key; if (k) d[k] = pg.hv(k); }
        return d;
    }
    readonly property var committed: {
        var d = {};
        if (pg.hub)
            for (var i = 0; i < pg.rows.length; i++) { var k = pg.rows[i].key; if (k) d[k] = pg.cv(k); }
        return d;
    }

    // a non-main output is selected with the link off: it shows the layout it has
    // live (the shared value when it has none) and only its tiling row; the
    // compositor tab (picker, cannot-do list) stays on the main screen
    readonly property var viewDraft: {
        if (!pg.scopedView) return pg.draft;
        var d = {};
        for (var k in pg.draft) d[k] = pg.draft[k];
        d["desktop.appearance.layout"] = pg.liveLayout(pg.activeOutput) || pg.draft["desktop.appearance.layout"];
        return d;
    }
    // the neutral gate applied; the shared renderer then drops caps and dead-key
    // rows, so the page and its gate read the same set the rail does.
    readonly property var settingsSchema: {
        var d = pg.viewDraft, out = [];
        for (var i = 0; i < pg.rows.length; i++) {
            if (pg.scopedView && pg.rows[i].key !== "desktop.appearance.layout") continue;
            if (pg.gateOk(pg.rows[i].key, d)) out.push(pg.rows[i]);
        }
        return out;
    }

    // the cannot-do lines: the provider's unhonored reasons, deduped (many
    // keybinds collapse to one "no scratchpad" reason) and shown verbatim.
    readonly property var cannotDo: {
        ProviderSchema.revision;
        var seen = {}, out = [];
        var u = ProviderSchema.unhonored || [];
        for (var i = 0; i < u.length; i++) {
            var r = u[i] ? u[i].reason : "";
            if (r && !seen[r]) { seen[r] = true; out.push(r); }
        }
        return out;
    }

    // the rowless tab that carries the compositor itself
    readonly property string switchTab: I18n.tr("Compositor")

    // Display link: linked (default) keeps every setting on all screens;
    // unlinked lets each output own its tiling layout. Only a provider with a
    // per-workspace layout (Wm.caps.perWorkspaceLayout) offers it; elsewhere the
    // link row is absent and every output shares the one layout.
    // the Hub's own caps frame, the same gate every Hub page uses; reading the Wm
    // singleton at load time crashes the page where no compositor is connected
    readonly property bool perOutput: Settings.supports("perWorkspaceLayout")
    property bool linked: true
    property string selectedOutput: ""
    // the saved state is the source of truth: startup and every failed write re-read it
    Process {
        id: tilingGet
        command: ["ryoku-output-tiling", "get"]
        running: pg.perOutput
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var st = JSON.parse(this.text);
                    pg.linked = st.linked !== false;
                } catch (e) { pg.linked = true; }
            }
        }
    }
    // each write goes through the helper, which applies it through the compositor
    // seam and saves it; a non-zero exit is shown and the real state is re-read
    Process {
        id: tilingSet
        stderr: StdioCollector { id: tilingSetErr }
        onExited: (code) => {
            pg.tilingError = code === 0 ? "" : (tilingSetErr.text.trim() || I18n.tr("Couldn't set the layout."));
            tilingGet.running = true;
        }
    }
    Process {
        id: tilingLink
        stderr: StdioCollector { id: tilingLinkErr }
        onExited: (code) => {
            pg.tilingError = code === 0 ? "" : (tilingLinkErr.text.trim() || I18n.tr("Couldn't change the display link."));
            tilingGet.running = true;
        }
    }
    // connected outputs in the compositor's order; the first is the main screen
    readonly property var outputNames: {
        var outs = Wm.outputs || [], names = [];
        for (var i = 0; i < outs.length; i++)
            if (outs[i] && outs[i].name && !outs[i].disabled) names.push(outs[i].name);
        return names;
    }
    readonly property string activeOutput: pg.outputNames.indexOf(pg.selectedOutput) >= 0 ? pg.selectedOutput : (pg.outputNames[0] || "")
    // the layout each non-main output has live, read from the compositor's own
    // workspace list; empty when the output has no workspace of its own
    function liveLayout(output) {
        if (!Wm.ready) return "";
        var ws = Wm.workspaces || [];
        for (var i = 0; i < ws.length; i++)
            if (ws[i].output === output && ws[i].layout) return ws[i].layout;
        return "";
    }
    readonly property bool scopedView: pg.perOutput && !pg.linked && pg.activeOutput !== "" && pg.activeOutput !== pg.outputNames[0]
    // applies a layout to every workspace of one output through the seam; a
    // failure is reported and the page re-reads the real state
    property string tilingError: ""
    function setOutputLayout(output, layout) {
        tilingSet.command = ["ryoku-output-tiling", "set", output, layout];
        tilingSet.running = true;
    }
    function setLink(on) {
        pg.linked = on;
        tilingLink.command = ["ryoku-output-tiling", "link", on ? "on" : "off"];
        tilingLink.running = true;
    }
    Component {
        id: linkRowC
        Column {
            width: parent ? parent.width : implicitWidth
            spacing: Tokens.s2
            OutputTabs {
                outputs: pg.outputNames
                linked: pg.linked
                selected: pg.activeOutput
                onLinkToggled: (v) => {
                    if (!v && pg.selectedOutput === "") pg.selectedOutput = pg.outputNames[1] || "";
                    pg.setLink(v);
                }
                onChose: (o) => pg.selectedOutput = o
            }
            Text {
                visible: pg.tilingError !== ""
                width: parent.width
                wrapMode: Text.WordWrap
                text: pg.tilingError
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
        }
    }

    function focusKey(k) { sp.focusKey(k) }
    SchemaPage {
        id: sp
        anchors.fill: parent
        schema: pg.settingsSchema
        draft: pg.viewDraft
        defaults: pg.committed
        advanced: pg.hub ? pg.hub.advanced : false
        title: pg.pTitle
        eyebrow: pg.pEyebrow
        blurb: pg.pBlurb
        query: pg.hub ? pg.hub.query : ""
        onEdited: (k, v) => {
            // a non-main output's tiling applies live to its own workspaces; it is
            // not part of the Save draft, so Save and the linked view stay as they are
            if (pg.scopedView && k === "desktop.appearance.layout") {
                pg.setOutputLayout(pg.activeOutput, v);
                return;
            }
            if (pg.hub) pg.hub.hyprEdit(k, v);
        }
        onPickRequested: (r) => { if (pg.hub) pg.hub.openPick(r); }
        // Which compositor is running, how to change it, and what it cannot do
        // are about the compositor rather than about any group of settings, so
        // they get a tab of their own instead of repeating above every tab.
        tailTabs: pg.scopedView ? [] : [pg.switchTab]
        headerExtra: pg.perOutput ? linkRowC : null

        Column {
            width: parent.width
            spacing: Tokens.s5
            visible: sp.tab === pg.switchTab

            CompositorControl {
                id: wmPicker
                width: parent.width
                onChose: (target, current) => wmSheet.open(target, current)
            }

            // A rarely-touched list: folded, and its header says what it holds, so the
            // tab reads as a page rather than a wall of near-identical lines on
            // first sight. It opens in place when asked.
            SettingCard {
                id: cannotSec
                width: parent.width
                visible: pg.cannotDo.length > 0
                collapsible: true
                expanded: false
                title: I18n.tr("WHAT %1 CANNOT DO").arg(pg.pTitle.toUpperCase())
                summary: pg.cannotDo.length + " " + I18n.tr("SETTINGS")

                Column {
                    width: cannotSec.width
                    spacing: Tokens.s2

                    Text {
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4; topPadding: Tokens.s3
                        wrapMode: Text.WordWrap
                        text: I18n.tr("These settings have no equivalent here. They stay in your store and return if you switch back.")
                        color: Tokens.inkFaint
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        lineHeight: 1.3
                    }
                    Repeater {
                        model: pg.cannotDo
                        Row {
                            required property var modelData
                            width: cannotSec.width
                            spacing: Tokens.s2
                            bottomPadding: Tokens.s1
                            Text {
                                text: "\u00b7"
                                leftPadding: Tokens.s4
                                color: Tokens.inkFaint
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fSmall
                            }
                            Text {
                                width: cannotSec.width - Tokens.s6
                                wrapMode: Text.WordWrap
                                text: parent.modelData
                                color: Tokens.inkMuted
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                                lineHeight: 1.3
                            }
                        }
                    }
                    Item { width: 1; height: Tokens.s2 }
                }
            }
        }
    }

    // the destructive switch confirmation, a full-page overlay; reload the picker
    // when it closes so the active row is fresh.
    CompositorSwitchSheet {
        id: wmSheet
        anchors.fill: parent
        onClosed: wmPicker.reload()
    }
}
