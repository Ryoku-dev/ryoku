import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/graphics"
import "../schema/graphics.js" as Schema

// which card draws the desktop, what the screen is wired to, this session's tuning and passthrough
Item {
    id: root

    required property var hub

    readonly property var store: Changes.stores["graphics"] || null
    readonly property var caps: root.store ? root.store.caps : ({})
    readonly property bool twoCards: !!root.store && root.store.twoCards
    readonly property string dgpu: root.caps.passthrough ? root.caps.passthrough.model : I18n.tr("the discrete graphics card")
    readonly property var modeBind: ({ store: "graphics", key: "mode" })
    readonly property var muxBind: ({ store: "graphics", key: "mux" })
    // a window manager that picks its own card never takes the mode, so nothing waits on a log out
    readonly property bool modePending: Attention.has("graphics-mode") && !!root.store && root.store.modeApplies
    readonly property bool muxPending: !!root.store && root.store.mux.capable === true && root.store.mux.reboot_pending === true
    // the wiring the computer started with; when it cannot be seen, a restart may already have been done
    readonly property string muxBooted: root.store && ["hybrid", "discrete"].indexOf(root.store.mux.booted_mode) >= 0 ? root.store.mux.booted_mode : ""
    readonly property string muxRestartLine: root.muxBooted !== "" ? I18n.tr("Restart to move the screen to the other card.")
        : I18n.tr("Restart if the screen has not moved to the other card yet.")

    // Performance pins the desktop to the discrete card only while one of its ports has a
    // monitor at log in, since the built-in screen is wired to the integrated graphics
    readonly property bool externalRoute: !!root.store && root.store.performanceAvailable && root.store.performanceRoute === "external"

    readonly property var modeLabels: ({ hybrid: I18n.tr("Hybrid"), performance: I18n.tr("Performance"), passthrough: I18n.tr("Passthrough") })

    // rows come from what the cards report, so a deep link waits until they have answered
    property string pendingRow: ""
    readonly property bool dataReady: Changes.revision >= 0 && !!root.store && root.store.capsRead && root.store.tuneRead

    function focusRow(rowId) {
        root.pendingRow = rowId || "";
        settle.restart();
    }

    // a template row is drawn by the page itself, a section id by its section; any
    // other waits until it is in the layout
    function landPending(force) {
        if (root.pendingRow === "" || page.layout.length === 0)
            return;
        var id = root.pendingRow;
        var template = Schema.rows.some(r => r.id === id && r.template === true);
        var known = template || !!page.sectionOfRow(id) || page.layout.some(s => s.id === id);
        if (force !== true && (!root.dataReady || !known)) {
            if (!giveUp.running)
                giveUp.restart();
            return;
        }
        giveUp.stop();
        root.pendingRow = "";
        page.focusRow(id);
    }
    onDataReadyChanged: settle.restart()
    // rows redraw while the cards' answers arrive; land once they rest
    Timer {
        id: settle
        interval: 400
        onTriggered: root.landPending()
    }
    Timer {
        id: giveUp
        interval: 5000
        onTriggered: root.landPending(true)
    }

    Component.onCompleted: {
        Changes.ensureStore("graphics", Qt.resolvedUrl("../components/graphics/GraphicsStore.qml"));
        // a store kept from an earlier visit reads the hardware again
        if (root.store && root.store.capsRead)
            root.store.refresh();
    }

    function cardName(slot) {
        if (root.caps.host && root.caps.host.slot === slot)
            return root.caps.host.model;
        if (root.caps.passthrough && root.caps.passthrough.slot === slot)
            return root.caps.passthrough.model;
        return I18n.tr("Graphics card");
    }

    // a reported knob as a row; the first of each kind takes the template's id so search lands on it
    readonly property var knobIds: ({
        power_limit: "tune-power-limit", power_cap: "tune-power-limit", persistence: "tune-persistence",
        perf_level: "tune-perf-level", clock_lock: "tune-clock", sclk_od: "tune-clock", gt_freq: "tune-clock", fan_pct: "tune-fan"
    })
    readonly property var knobLabels: ({
        power_limit: "Power limit", power_cap: "Power limit", persistence: "Keep the driver loaded",
        perf_level: "Performance level", clock_lock: "Highest graphics clock", sclk_od: "Highest graphics clock",
        gt_freq: "Highest graphics clock", fan_pct: "Fan speed"
    })
    readonly property var levelLabels: ({ auto: "Automatic", low: "Low", high: "High" })

    function knobRows() {
        if (!root.store)
            return [];
        var seen = {}, out = [];
        root.store.tune.forEach(function (t) {
            var base = root.knobIds[t.id] || ("tune-" + t.id);
            seen[base] = (seen[base] || 0) + 1;
            var template = Schema.rows.find(r => r.id === base);
            var label = root.knobLabels[t.id] || t.label;
            var advanced = t.risk === "advanced";
            var parts = [root.twoCards ? root.cardName(t.gpu) : "", template ? I18n.tr(template.detail) : "",
                         advanced ? I18n.tr("Can freeze the screen until you restart") : ""];
            var row = {
                id: seen[base] > 1 ? base + "-" + seen[base] : base,
                section: "tuning", more: advanced, label: label,
                detail: parts.filter(p => p !== "").join(". "),
                changeLabel: root.twoCards ? I18n.tr("%1 on %2").arg(I18n.tr(label)).arg(root.cardName(t.gpu)) : label,
                keywords: template ? template.keywords : [],
                bind: { store: "graphics", key: root.store.key(t.gpu, t.id) }
            };
            if (t.kind === "toggle")
                row.control = "toggle";
            else if (t.kind === "segment") {
                row.control = "segmented";
                row.options = (t.options || []).map(o => ({ value: o, label: root.levelLabels[o] || o }));
            } else {
                row.control = root.store.autoKnobs.indexOf(t.id) >= 0 ? "custom" : "slider";
                row.min = t.min || 0;
                row.max = t.max;
                row.step = 1;
                row.unit = t.unit || "";
            }
            out.push(row);
        });
        return out;
    }

    function modeDetail() {
        var m = root.store ? root.store.mode : "";
        var text = m === "performance"
            ? (root.externalRoute
                ? I18n.tr("%1 draws the desktop while a monitor is plugged into one of its ports when you log in, and the built-in screen can then stay dark after you log in or after sleep. Without that monitor, the integrated graphics draw the desktop.").arg(root.dgpu)
                : root.caps.chassis === "laptop" && root.store.mux.capable && root.store.mux.mode !== "discrete"
                ? I18n.tr("%1 draws the desktop: fastest, and it uses more power. For the full effect, wire the screen to it too.").arg(root.dgpu)
                : I18n.tr("%1 draws the desktop: fastest, and it uses more power.").arg(root.dgpu))
            : m === "passthrough"
                ? I18n.tr("The integrated graphics draw the desktop, so %1 is free for a virtual machine.").arg(root.dgpu)
                : root.caps.passthrough && root.caps.passthrough.drivesDisplay
                    ? I18n.tr("No card is pinned. The screen is wired to %1, so that card draws the desktop.").arg(root.dgpu)
                    : I18n.tr("The integrated graphics draw the desktop to save battery; apps can still use %1 when they ask for it.").arg(root.dgpu);
        if (root.store && !root.store.performanceAvailable)
            text += " " + I18n.tr("Performance needs the screen wired to %1 first.").arg(root.dgpu);
        else if (root.externalRoute && m !== "performance")
            text += " " + I18n.tr("Performance makes %1 draw the desktop only while a monitor is plugged into one of its ports when you log in, and the built-in screen can then stay dark after you log in or after sleep.").arg(root.dgpu);
        if (root.modePending)
            text += " " + I18n.tr("Takes effect after you log out.");
        return text;
    }

    function muxDetail() {
        if (root.muxPending)
            return root.muxRestartLine;
        return root.store && root.store.mux.mode === "discrete"
            ? I18n.tr("The built-in screen runs on %1, so the card stays awake while the screen is on.").arg(root.dgpu)
            : I18n.tr("The built-in screen runs on the integrated graphics, so %1 can sleep.").arg(root.dgpu);
    }

    readonly property var schema: {
        Changes.revision;
        var rows = [];
        if (root.store && root.store.capsRead) {
            Schema.rows.forEach(function (r) {
                if (r.template === true && r.section !== "mode" && r.section !== "presets")
                    return;
                if (r.section === "mode" && !root.twoCards)
                    return;
                if (r.id === "graphics-mode")
                    rows.push(Object.assign({}, r, { detail: root.modeDetail() },
                        root.store.modeError !== "" ? { unavailable: I18n.tr("Could not read the graphics mode") }
                        : !root.store.modeApplies ? { unavailable: I18n.tr("The window manager in use picks its graphics card itself, so the graphics mode has no effect while you use it") }
                        : {}));
                else if (r.id === "display-wired") {
                    if (root.store.mux.capable === true)
                        rows.push(Object.assign({}, r, { detail: root.muxDetail() }));
                } else if (r.section === "presets") {
                    if (root.store.tune.length > 0)
                        rows.push(r);
                } else
                    rows.push(r);
            });
            rows = rows.concat(root.knobRows());
        }
        return { title: Schema.title, lead: Schema.lead, sections: Schema.sections, related: Schema.related, rows: rows };
    }

    readonly property string stateLine: {
        if (!root.store || !root.store.capsRead)
            return "";
        var parts = [];
        var render = root.store.renderCard;
        if (render) {
            var live = root.store.live;
            if (live.ok)
                parts.push(I18n.tr("%1 draws the desktop, at %2°C and %3% busy.").arg(render.model).arg(live.temp).arg(live.util));
            else
                parts.push(I18n.tr("%1 draws the desktop.").arg(render.model));
            if (live.asleep)
                parts.push(I18n.tr("The discrete card is asleep, drawing no power."));
        }
        if (root.modePending)
            parts.push(I18n.tr("%1 graphics takes effect after you log out.").arg(I18n.tr(root.modeLabels[root.store.mode] || "Hybrid")));
        if (root.muxPending)
            parts.push(root.muxRestartLine);
        return parts.join(" ");
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "graphics"
        schema: root.schema
        stateLine: root.stateLine
        onLayoutChanged: settle.restart()
        controls: Object.assign({ "graphics-mode": modeControl, "display-wired": muxControl, "tune-preset": presetControl, "tune-reset": resetControl },
                                root.liveKnobControls)
        bodies: root.store && root.store.capsRead ? ({ tuning: tuningLead }) : ({})
        footers: root.store && root.store.capsRead
            ? Object.assign({ tuning: lactNote }, !root.twoCards || root.store.capsError !== "" || root.store.modeError !== "" ? { mode: modeNote } : {})
            : ({})
        moreBodies: root.store && root.store.capsRead
            ? Object.assign(root.twoCards ? { mode: passthroughBody } : {}, root.store.tune.length > 0 ? { presets: yourPresets } : {})
            : ({})
    }

    // the clock limit and the fan have an automatic state no number stands for:
    // the value "auto" (the clock slider at its top, the fan until it is set),
    // which Undo and Reset all tuning put back like any other value
    readonly property var liveKnobControls: {
        var out = {};
        root.knobRows().filter(r => r.control === "custom").forEach(r => out[r.id] = liveKnob);
        return out;
    }

    Component {
        id: liveKnob
        Row {
            id: knob
            property var row
            readonly property bool clock: !!knob.row && !knob.row.bind.key.endsWith("|fan_pct")
            readonly property var stored: knob.row ? Changes.value(knob.row.bind) : undefined
            readonly property bool auto: knob.stored === "auto" && !slider.pressed
            readonly property string writeError: knob.row ? Changes.error(knob.row.bind) : ""
            property bool bound: true
            // a failed write puts the slider back on the stored value
            onWriteErrorChanged: {
                knob.bound = false;
                knob.bound = true;
            }
            spacing: LithicTokens.space.s3
            // the row hands keyboard focus to the control; the slider is what takes keys
            onActiveFocusChanged: if (activeFocus) slider.forceActiveFocus(Qt.OtherFocusReason)
            Slider {
                id: slider
                anchors.verticalCenter: parent.verticalCenter
                width: 200
                from: knob.row ? knob.row.min : 0
                to: knob.row ? knob.row.max : 1
                stepSize: 1
                Binding {
                    target: slider
                    property: "value"
                    value: knob.stored === "auto" ? (knob.clock ? slider.to : slider.from) : Number(knob.stored) || 0
                    when: knob.bound && !slider.pressed
                }
                accessibleName: knob.row ? I18n.tr(knob.row.label) : ""
                onMoved: {
                    var r = knob.row, v = Math.round(slider.value);
                    Changes.set(r.bind, knob.clock && v >= r.max ? "auto" : v, r.changeLabel || r.label, "graphics");
                }
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: 72
                horizontalAlignment: Text.AlignRight
                text: knob.auto || (knob.clock && slider.pressed && Math.round(slider.value) >= slider.to)
                    ? (knob.clock ? I18n.tr("No limit") : I18n.tr("Automatic"))
                    : Math.round(slider.value) + (!knob.row || !knob.row.unit ? "" : knob.row.unit === "%" ? "%" : " " + knob.row.unit)
                typeRole: "label"
                inkRole: "fg2"
                numeric: true
            }
        }
    }

    Component {
        id: modeControl
        Segmented {
            id: seg
            property var row
            readonly property var values: ["hybrid", "performance", "passthrough"].filter(m => m !== "performance" || !root.store || root.store.performanceAvailable || root.store.mode === "performance")
            readonly property int current: root.store ? values.indexOf(root.store.mode) : -1
            width: Math.min(implicitWidth, LithicTokens.size.settingsControlMax)
            model: values.map(m => I18n.tr(root.modeLabels[m]))
            // a mode that could not be read selects no segment, rather than one that may not be true
            readonly property int shown: !root.store || root.store.modeError !== "" ? -1 : Math.max(0, seg.current)
            currentIndex: seg.shown
            accessibleName: I18n.tr("Graphics mode")
            onActivated: index => {
                var m = seg.values[index];
                seg.currentIndex = Qt.binding(() => seg.shown);
                if (m === root.store.mode)
                    return;
                var label = I18n.tr(root.modeLabels[m]);
                var what = m === "performance"
                    ? (root.externalRoute
                        ? I18n.tr("%1 will draw the desktop while a monitor is plugged into one of its ports when you log in, and the built-in screen can then stay dark after you log in or after sleep.").arg(root.dgpu)
                        : I18n.tr("%1 will draw the desktop.").arg(root.dgpu))
                    : m === "passthrough" ? I18n.tr("The integrated graphics will draw the desktop and %1 will be free for a virtual machine.").arg(root.dgpu)
                    : I18n.tr("The integrated graphics will draw the desktop, and apps can still use %1.").arg(root.dgpu);
                root.hub.confirm({
                    title: I18n.tr("Switch to %1 graphics?").arg(label),
                    text: what + " " + I18n.tr("It takes effect after you log out."),
                    actionText: I18n.tr("Switch"),
                    destructive: false
                }, function () {
                    Changes.set(root.modeBind, m, "Graphics mode", "graphics", I18n.tr("Graphics mode set to %1. Log out to finish.").arg(label));
                });
            }
        }
    }

    // the wiring write can wait on a password prompt, so it has no time limit: past 1 s
    // it says it is working, past 10 s it shows progress and Cancel
    Component {
        id: muxControl
        Column {
            id: wired
            property var row
            readonly property bool writing: !!root.store && root.store.busy["mux"] === true
            readonly property bool slow: wired.writing && root.store.muxSlow
            readonly property bool verySlow: wired.writing && root.store.muxVerySlow
            readonly property bool stopping: wired.writing && root.store.muxStopping
            width: seg.width
            spacing: LithicTokens.space.s2
            // Cancel appears under keyboard focus, which comes back to the segments once the write ends
            property bool keys: false
            onVerySlowChanged: {
                if (wired.verySlow && seg.visualFocus) {
                    wired.keys = true;
                    Qt.callLater(() => cancelWiring.forceActiveFocus(Qt.TabFocusReason));
                }
            }
            onWritingChanged: {
                if (!wired.writing && wired.keys)
                    seg.forceActiveFocus(Qt.TabFocusReason);
                wired.keys = false;
            }

            Segmented {
                id: seg
                readonly property var values: ["hybrid", "discrete"]
                readonly property var labels: ({ hybrid: I18n.tr("Integrated"), discrete: I18n.tr("Discrete") })
                readonly property int current: root.store ? values.indexOf(root.store.mux.mode) : -1
                width: Math.min(implicitWidth, LithicTokens.size.settingsControlMax)
                model: values.map(m => I18n.tr(seg.labels[m]))
                currentIndex: Math.max(0, seg.current)
                accessibleName: I18n.tr("Display wired to")
                onActivated: index => {
                    var m = seg.values[index];
                    seg.currentIndex = Qt.binding(() => Math.max(0, seg.current));
                    if (m === root.store.mux.mode || wired.writing)
                        return;
                    root.hub.confirm({
                        title: m === "discrete" ? I18n.tr("Wire the screen to %1?").arg(root.dgpu) : I18n.tr("Wire the screen to the integrated graphics?"),
                        text: m === root.muxBooted
                            ? (m === "discrete"
                                ? I18n.tr("The built-in screen stays on %1, where it runs now, so no restart is needed. The card stays awake and uses more power while the screen is on.").arg(root.dgpu)
                                : I18n.tr("The built-in screen stays on the integrated graphics, where it runs now, so no restart is needed. %1 can sleep and the battery lasts longer.").arg(root.dgpu))
                            : m === "discrete"
                                ? I18n.tr("After you restart, the built-in screen runs on %1. It is faster, and the card stays awake and uses more power while the screen is on.").arg(root.dgpu)
                                : I18n.tr("After you restart, the built-in screen runs on the integrated graphics, so %1 can sleep and the battery lasts longer.").arg(root.dgpu),
                        actionText: I18n.tr("Change wiring"),
                        destructive: false
                    }, function () {
                        Changes.set(root.muxBind, m, I18n.tr("Display wiring"), "graphics",
                                    m === root.muxBooted ? I18n.tr("The screen stays where it is, so no restart is needed.")
                                    : root.muxBooted !== "" ? I18n.tr("The screen moves to the other card when you restart.")
                                    : I18n.tr("Restart if the screen has not moved to the other card yet."));
                    });
                }
            }
            Row {
                visible: wired.slow
                width: parent.width
                spacing: LithicTokens.space.s2
                Working {
                    id: wiringWork
                    anchors.verticalCenter: parent.verticalCenter
                    active: wired.slow
                }
                Label {
                    width: parent.width - wiringWork.width - parent.spacing
                    anchors.verticalCenter: parent.verticalCenter
                    text: wired.stopping ? I18n.tr("Stopping…")
                        : wired.verySlow ? I18n.tr("Still changing the wiring. Enter your password if it is asked for, or cancel.")
                        : I18n.tr("Changing the wiring. Enter your password if it is asked for.")
                    typeRole: "detail"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }
            BladeProgress {
                width: parent.width
                visible: wired.verySlow
                active: wired.verySlow
                indeterminate: true
            }
            Plaque {
                id: cancelWiring
                visible: wired.verySlow && !wired.stopping
                kind: "quiet"
                small: true
                text: I18n.tr("Cancel")
                accessibleName: I18n.tr("Cancel changing the wiring")
                onClicked: root.store.cancelMux()
            }
        }
    }

    Component {
        id: presetControl
        Plaque {
            id: pick
            property var row
            readonly property var names: root.store ? root.store.presets.map(p => p.name) : []
            readonly property var builtin: ({ Quiet: "Quiet", Balanced: "Balanced", Performance: "Performance" })
            function shown(name) {
                return pick.builtin[name] ? I18n.tr(pick.builtin[name]) : name;
            }
            function activate() {
                presetMenu.open();
            }
            kind: "quiet"
            small: true
            glyph: "expand_more"
            busy: !!root.store && root.store.busy["tune|all"] === true
            text: I18n.tr("Choose")
            accessibleName: I18n.tr("Apply a preset")
            onClicked: activate()

            // the menu widens to the longest name, a saved one too, up to the library's widest menu
            Label {
                id: menuType
                visible: false
                typeRole: "label"
            }
            FontMetrics {
                id: menuMetrics
                font: menuType.font
            }

            Popover {
                id: presetMenu
                parent: root.hub.overlayItem
                anchorItem: pick
                opener: pick

                // the popover focuses the first item, marked only when the keys opened it
                Menu {
                    menuWidth: Math.ceil(Math.max(0, ...pick.names.map(n => menuMetrics.advanceWidth(pick.shown(n)))))
                        + LithicTokens.size.menuInset * 2 + 1
                    items: pick.names.map(n => ({ label: pick.shown(n) }))
                    onTriggered: index => {
                        var name = pick.names[index];
                        presetMenu.close();
                        Changes.set({ store: "graphics", key: "tune|all" }, { preset: name }, I18n.tr("Tuning preset"), "graphics",
                                    I18n.tr("%1 preset applied").arg(pick.shown(name)));
                    }
                }
            }
        }
    }

    Component {
        id: resetControl
        Plaque {
            property var row
            function activate() {
                root.hub.confirm({
                    title: I18n.tr("Reset all tuning?"),
                    text: I18n.tr("Every tuning setting goes back to the driver's own. Undo puts your tuning back."),
                    actionText: I18n.tr("Reset")
                }, function () {
                    Changes.set({ store: "graphics", key: "tune|reset" }, { reset: true }, I18n.tr("All tuning"), "graphics",
                                I18n.tr("Tuning reset"));
                });
            }
            kind: "quiet"
            small: true
            glyph: "restart_alt"
            busy: !!root.store && root.store.busy["tune|all"] === true
            text: I18n.tr("Reset")
            accessibleName: I18n.tr("Reset all tuning")
            onClicked: activate()
        }
    }

    Component {
        id: tuningLead
        Column {
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                text: root.store && root.store.tuneError !== ""
                    ? root.store.tuneError
                    : root.store && root.store.tune.length > 0
                        ? I18n.tr("Changes apply now and last until you restart, so a setting that misbehaves goes away with a restart.")
                        : I18n.tr("Your graphics driver offers nothing to tune on this computer.")
                typeRole: "detail"
                ink: root.store && root.store.tuneError !== "" ? Lithic.jasper : Lithic.fg2
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                visible: !!root.store && root.store.tuneError !== ""
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.store.loadTune()
            }
        }
    }

    Component {
        id: lactNote
        Label {
            visible: !!root.store && root.store.tune.length > 0
            text: I18n.tr("For voltage and fan curves, install LACT, a dedicated graphics control app.")
            typeRole: "detail"
            inkRole: "fg3"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Component {
        id: modeNote
        Column {
            readonly property string error: !root.store ? ""
                : root.store.capsError !== "" ? root.store.capsError
                : root.twoCards && root.store.modeError !== "" ? root.store.modeError
                : ""
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                text: parent.error !== "" ? parent.error : I18n.tr("This computer has one graphics card, so it always draws the desktop.")
                typeRole: "detail"
                ink: parent.error !== "" ? Lithic.jasper : Lithic.fg3
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                visible: parent.error !== ""
                kind: "quiet"
                small: true
                glyph: "refresh"
                text: I18n.tr("Retry")
                onClicked: root.store.refresh()
            }
        }
    }

    Component {
        id: passthroughBody
        Passthrough {
            hub: root.hub
            store: root.store
        }
    }

    Component {
        id: yourPresets
        YourPresets {
            hub: root.hub
            store: root.store
        }
    }
}
