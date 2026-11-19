pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../ask"
import "../ask/bar"
import "../ask/look"
import "../ask/lib/keys.js" as Chords
import "lib/sections.js" as Sections
import "lib/keymap.js" as Keymap
import "lib/memory.js" as Memory
import "sections/downloads" as Downloads
import "sections/keys" as KeysSection
import "sections/commands" as Commands
import "sections/phone" as Phone
import "sections/devtools" as Devtools
import "sections/typing" as Typing

// Kit's body, the tabbed manual: the rail with its six tabs, and the page they
// weld into (field, the section's page strip, the section, the foot, the Action
// Panel). It owns the keyboard (lib/keymap.js), the memory and the one section
// that is alive; a section only answers the contract (kit-contract section 2):
// sections/<id>/Section.qml is what the page shows, sections/<id>/Source.qml is
// what the section knows while Kit is open (its tab's figure, its search rows).
FocusScope {
    id: root

    required property real s
    property bool active: false
    // a route's request; the serial lets the same one land twice
    property string requestSection: ""
    property string requestPage: ""
    property var requestArgs: ({})
    property int requestSerial: 0

    readonly property real pageX: rail.railWidth
    property bool built: false
    property string section: ""
    property string fromSection: ""
    // the weld and the body change on one progress value
    property real k: 1
    // all of Kit (Ctrl+F) over the section
    property bool all: false
    property real folded: root.all ? 1 : 0
    property int direction: 1
    // each section keeps its own words in the field while Kit is open
    property var texts: ({})
    property string sectionText: ""
    property string allText: ""
    property var sectionArgs: ({})
    property string pendingPage: ""
    // {id: Section}, {id: Source}, {id: {text, tone}}
    property var views: ({})
    property var sources: ({})
    property var lit: ({})

    readonly property var labels: ({
        downloads: I18n.tr("Downloads"), keys: I18n.tr("Keys"), commands: I18n.tr("Commands"),
        phone: I18n.tr("Phone"), devtools: I18n.tr("Dev tools"), typing: I18n.tr("Typing test")
    })
    readonly property var sectionView: root.views[root.section] || null
    readonly property var view: root.all ? allSlot.view : root.sectionView
    readonly property bool quiet: root.view !== null && root.view.ownsText === true
    readonly property var stripPages: !root.all && root.sectionView && root.sectionView.pages ? root.sectionView.pages : []
    readonly property var primaryAction: root.view && root.view.primaryAction ? root.view.primaryAction : null
    readonly property var secondaryAction: root.view && root.view.secondaryAction ? root.view.secondaryAction : null
    readonly property var viewActions: root.view && root.view.actions ? root.view.actions : []
    // the panel lists what Enter and the second chord do first, then the section's actions
    readonly property var panelActions: {
        const out = [];
        if (root.primaryAction)
            out.push({ id: "\u0000primary", label: root.primaryAction.label, glyph: root.primaryAction.glyph || "keyboard_return", hint: root.primaryAction.hint || "Enter", section: "" });
        if (root.secondaryAction)
            out.push({ id: "\u0000secondary", label: root.secondaryAction.label, glyph: root.secondaryAction.glyph || "subdirectory_arrow_right", hint: root.secondaryAction.hint || "Ctrl+Enter", section: "" });
        return out.concat(root.viewActions);
    }
    readonly property var figures: {
        const out = {};
        for (var id in root.sources)
            out[id] = root.sources[id].figure || null;
        return out;
    }

    signal requestClose()

    function flush() { memory.flush(); }

    focus: true

    Behavior on folded {
        enabled: root.active
        NumberAnimation { duration: Look.dur(Look.viewMs); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
    }
    NumberAnimation {
        id: weld
        target: root
        property: "k"
        from: 0
        to: 1
        duration: Look.dur(Look.viewMs)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Look.arrive
    }

    function noteView(id, view) {
        const next = Object.assign({}, root.views);
        if (view)
            next[id] = view;
        else
            delete next[id];
        root.views = next;
        // after the section has been given its memory, which may restore a page
        if (view && id === root.section)
            Qt.callLater(root.settle);
    }

    function noteSource(id, source) {
        const next = Object.assign({}, root.sources);
        next[id] = source;
        root.sources = next;
    }

    function noteLit(id, news) {
        if (!news || String(news.text || "").length === 0)
            return;
        if (id === root.section && !root.all) {
            Qt.callLater(root.seen, id);
            return;
        }
        const next = Object.assign({}, root.lit);
        next[id] = { text: String(news.text), tone: String(news.tone || "") };
        root.lit = next;
    }

    function seen(id) {
        if (root.lit[id] !== undefined) {
            const next = Object.assign({}, root.lit);
            delete next[id];
            root.lit = next;
        }
        const source = root.sources[id];
        if (source && typeof source.seen === "function")
            source.seen();
    }

    // settle: a section that just arrived takes its pending page and the keyboard
    function settle() {
        const view = root.sectionView;
        if (!view)
            return;
        if (root.pendingPage.length > 0) {
            view.pageId = root.pendingPage;
            root.pendingPage = "";
        }
        root.focusKeyboard();
    }

    function setPage(id) {
        const view = root.sectionView;
        if (view && String(id).length > 0)
            view.pageId = String(id);
    }

    // openSection goes to a section (and a page of it). remember: the reader
    // chose it, so Kit reopens there; a route does not move that memory.
    function openSection(id, page, remember, text, args) {
        if (!Sections.has(id))
            return;
        panel.open = false;
        root.pendingPage = "";
        const changed = id !== root.section;
        if (changed) {
            if (!root.all && root.section.length > 0)
                root.keepText();
            if (root.section.length > 0) {
                root.fromSection = root.section;
                if (root.active)
                    weld.restart();
            }
            root.direction = 1;
            root.sectionArgs = args || ({});
            root.section = id;
        } else if (args) {
            root.sectionArgs = args;
        }
        const words = text !== undefined && text !== null ? String(text) : String(root.texts[id] || "");
        if (root.all) {
            root.direction = changed ? 1 : -1;
            root.all = false;
        }
        field.text = words;
        root.sectionText = words;
        const leaf = String(page || memory.get("frame.page." + id, ""));
        if (leaf.length > 0) {
            if (root.sectionView && !changed)
                root.sectionView.pageId = leaf;
            else
                root.pendingPage = leaf;
        }
        if (remember === true)
            memory.set(Memory.SECTION_KEY, id);
        root.seen(id);
        Qt.callLater(root.focusKeyboard);
    }

    function keepText() {
        const next = Object.assign({}, root.texts);
        next[root.section] = field.text;
        root.texts = next;
    }

    function enterAll() {
        if (root.all)
            return;
        panel.open = false;
        root.keepText();
        root.direction = 1;
        root.all = true;
        field.text = "";
        root.allText = "";
        Qt.callLater(root.focusKeyboard);
    }

    function leaveAll() {
        if (!root.all)
            return;
        panel.open = false;
        root.direction = -1;
        root.all = false;
        field.text = String(root.texts[root.section] || "");
        root.sectionText = field.text;
        Qt.callLater(root.focusKeyboard);
    }

    function applyRequest() {
        if (!memory.ready)
            return;
        if (Sections.has(root.requestSection))
            root.openSection(root.requestSection, root.requestPage, false, undefined, root.requestArgs);
        else
            root.openSection(Memory.startSection(memory.values, Sections.ORDER, Sections.FIRST), "", false);
    }

    // the keyboard rests in the field unless the section takes all typing
    function focusKeyboard() {
        if (!root.active || panel.open)
            return;
        const view = root.view;
        if (view && view.ownsText === true && typeof view.takeFocus === "function") {
            view.takeFocus();
            return;
        }
        field.focusField();
    }

    function openPanel() {
        if (root.panelActions.length > 0)
            panel.open = true;
    }

    function closePanel() {
        panel.open = false;
        root.focusKeyboard();
    }

    function runPanel(id) {
        root.closePanel();
        if (!root.view)
            return;
        if (id === "\u0000primary")
            root.view.activate();
        else if (id === "\u0000secondary")
            root.view.activateSecondary();
        else
            root.view.act(id);
    }

    function escapeLadder() {
        const steps = Keymap.escape({ panel: panel.open, all: root.all, fieldText: field.text });
        for (var i = 0; i < steps.length; i++) {
            const step = steps[i];
            if (step === "panel") {
                root.closePanel();
                return;
            }
            if (step === "all") {
                root.leaveAll();
                return;
            }
            if (step === "section") {
                if (root.view && typeof root.view.handleEscape === "function" && root.view.handleEscape()) {
                    Qt.callLater(root.focusKeyboard);
                    return;
                }
                continue;
            }
            if (step === "clear") {
                field.clear();
                return;
            }
            root.requestClose();
            return;
        }
    }

    // the field is a plain item, so its own activeFocus never says its input has the keyboard
    function inside(item, ancestor) {
        for (var at = item; at; at = at.parent) {
            if (at === ancestor)
                return true;
        }
        return false;
    }

    function keyContext() {
        const win = root.Window.window;
        const item = win ? win.activeFocusItem : null;
        const typing = item instanceof TextInput || item instanceof TextEdit;
        const inField = root.inside(item, field);
        return {
            panel: panel.open, all: root.all, field: inField,
            textEntry: typing && !inField && !root.inside(item, panel),
            ownsText: root.quiet,
            primaryHint: root.primaryAction ? String(root.primaryAction.hint || "") : "",
            secondaryHint: root.secondaryAction ? String(root.secondaryAction.hint || "") : "",
            actions: root.viewActions
        };
    }

    function call(name, argument) {
        const view = root.view;
        if (view && typeof view[name] === "function")
            view[name](argument);
    }

    function run(op, arg) {
        switch (op) {
        case "escape":
            root.escapeLadder();
            break;
        case "section":
            root.openSection(Sections.at(arg), "", true);
            break;
        case "sectionStep":
            root.openSection(Sections.step(root.section, arg), "", true);
            break;
        case "pageStep":
            if (root.stripPages.length > 0)
                root.setPage(Sections.stepPage(root.stripPages, root.sectionView.pageId, arg));
            else
                root.call("jumpSection", arg);
            break;
        case "all":
            if (root.all)
                root.leaveAll();
            else
                root.enterAll();
            break;
        case "panel":
            if (panel.open)
                root.closePanel();
            else
                root.openPanel();
            break;
        case "activate":
            root.call("activate");
            break;
        case "secondary":
            root.call("activateSecondary");
            break;
        case "move":
            root.call("move", arg);
            break;
        case "jump":
            root.call("jumpSection", arg);
            break;
        case "rows":
            root.call("page", arg);
            break;
        case "act":
            root.call("act", String(arg));
            break;
        case "type":
            field.text = field.text + String(arg);
            field.focusField();
            break;
        }
    }

    function handleKey(event) {
        const chord = Chords.chord(event.key, event.modifiers);
        const command = Keymap.resolve(chord, event.text, root.keyContext());
        if (chord !== "Esc" && !Keymap.global(chord) && !root.all
                && !panel.open && root.sectionView && typeof root.sectionView.handleKey === "function"
                && root.sectionView.handleKey(event)) {
            event.accepted = true;
            return;
        }
        if (!command)
            return;
        event.accepted = true;
        if (command.op === "escape" && event.isAutoRepeat)
            return;
        root.run(command.op, command.arg);
    }

    Keys.onPressed: event => root.handleKey(event)

    onRequestSerialChanged: if (root.built) root.applyRequest()
    onActiveChanged: {
        if (root.active) {
            root.texts = ({});
            root.sectionArgs = ({});
            root.all = false;
            field.text = "";
            root.applyRequest();
            Qt.callLater(root.focusKeyboard);
        } else {
            panel.open = false;
            weld.stop();
            root.k = 1;
            root.all = false;
            foot.reset();
            memory.flush();
        }
    }
    Component.onCompleted: {
        root.built = true;
        root.applyRequest();
    }

    KitMemory {
        id: memory
        active: root.active
        // with no route, where Kit opens is the memory's to say
        onReadyChanged: if (ready && root.built) root.applyRequest()
    }
    Loader {
        id: downloadsLoader
        active: root.active
        sourceComponent: DownloadsClient {}
    }
    readonly property var downloadsClient: downloadsLoader.item

    // the frame's chords reach it from anywhere, a text pane included
    Repeater {
        model: Keymap.GLOBAL

        delegate: Item {
            id: chord

            required property var modelData

            Shortcut {
                sequences: chord.modelData.sequences
                enabled: root.active
                onActivated: root.run(chord.modelData.op, chord.modelData.arg)
            }
        }
    }

    // what each section knows while Kit is open
    Repeater {
        model: Sections.ORDER

        delegate: Loader {
            id: sourceLoader

            required property string modelData
            readonly property var news: sourceLoader.item && sourceLoader.item.lit ? sourceLoader.item.lit : null

            Component.onCompleted: sourceLoader.setSource(Qt.resolvedUrl("sections/" + sourceLoader.modelData + "/Source.qml"), { memory: memory, downloads: downloadsClient })
            onLoaded: root.noteSource(sourceLoader.modelData, sourceLoader.item)
            onNewsChanged: root.noteLit(sourceLoader.modelData, sourceLoader.news)

            Binding { target: sourceLoader.item; property: "current"; value: root.active && !root.all && root.section === sourceLoader.modelData; when: sourceLoader.item !== null }
            Binding { target: sourceLoader.item; property: "searching"; value: root.active && root.all; when: sourceLoader.item !== null }
            Binding { target: sourceLoader.item; property: "active"; value: root.active; when: sourceLoader.item !== null && "active" in sourceLoader.item }
            Binding { target: sourceLoader.item; property: "downloads"; value: root.downloadsClient; when: sourceLoader.item !== null }
        }
    }

    KitRail {
        id: rail
        anchors.fill: parent
        s: root.s
        active: root.active
        to: root.section
        from: root.fromSection
        k: root.k
        folded: root.folded
        labels: root.labels
        figures: root.figures
        lit: root.lit
        counting: root.all && allSlot.view !== null && allSlot.view.asked === true
        counts: allSlot.view && allSlot.view.counts ? allSlot.view.counts : ({})
        chosen: allSlot.view && allSlot.view.chosen ? allSlot.view.chosen : ""
        onPicked: id => root.openSection(id, "", true)
    }

    Item {
        id: page
        x: root.pageX
        width: parent.width - root.pageX
        height: parent.height

        // a press on the page is the page's; it never falls through to the scrim
        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.AllButtons
            onPressed: mouse => mouse.accepted = true
        }

        SearchField {
            id: field
            x: 1
            y: 1
            width: parent.width - 2
            height: Math.round(56 * root.s) - 1
            enabled: !root.quiet
            s: root.s
            token: root.all ? I18n.tr("All of Kit") : ""
            placeholder: root.view ? String(root.view.placeholder || "") : ""
            accessory: root.all || root.quiet ? null : allDoor
            onKeyPressed: event => root.handleKey(event)
            onBackRequested: root.leaveAll()
            onTextChanged: {
                if (root.all)
                    root.allText = field.text;
                else
                    root.sectionText = field.text;
            }
        }
        Component {
            id: allDoor

            Row {
                spacing: Math.round(8 * root.s)

                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("All of Kit")
                    color: allHover.hovered ? Lithic.fg : Lithic.fg2
                    font.family: Look.sans
                    font.pixelSize: Math.round(12.5 * root.s)
                    renderType: Text.NativeRendering
                }
                KeyHint {
                    anchors.verticalCenter: parent.verticalCenter
                    s: root.s
                    hint: "Ctrl+F"
                }
                HoverHandler { id: allHover; cursorShape: Qt.PointingHandCursor }
                TapHandler { onTapped: root.enterAll() }
            }
        }
        Rectangle {
            x: 1
            y: field.y + field.height
            width: parent.width - 2
            height: 1
            color: Look.rule
            visible: !Look.fluent
            z: 3
        }

        KitPages {
            id: strip
            x: 0
            y: Math.round(56 * root.s)
            width: parent.width
            height: root.stripPages.length > 0 ? implicitHeight : 0
            visible: height > 0
            s: root.s
            active: root.active
            pages: root.stripPages
            current: root.sectionView && root.sectionView.pageId !== undefined ? String(root.sectionView.pageId) : ""
            onPicked: id => {
                root.setPage(id);
                root.focusKeyboard();
            }
        }

        Item {
            id: stage
            x: 1
            y: strip.y + strip.height
            width: parent.width - 2
            height: Math.max(0, foot.y - y - 1)
            clip: true

            Repeater {
                model: Sections.ORDER

                // one place per section; only the one that is open holds a section
                delegate: ViewSlot {
                    id: slot

                    required property string modelData

                    s: root.s
                    name: slot.modelData
                    live: root.active && root.section === slot.modelData
                    current: root.active && root.section === slot.modelData && !root.all
                    direction: root.direction
                    barActive: root.active
                    animate: root.active
                    fieldText: root.sectionText
                    heldText: String(root.texts[slot.modelData] || "")
                    args: root.section === slot.modelData ? root.sectionArgs : ({})
                    maximumHeight: stage.height
                    source: Qt.resolvedUrl("sections/" + slot.modelData + "/Section.qml")
                    gifts: ({ memory: memory, downloads: downloadsClient, source: root.sources[slot.modelData] || null })
                    onViewChanged: root.noteView(slot.modelData, slot.view)

                    Binding { target: slot.view; property: "height"; value: stage.height; when: slot.view !== null }
                }
            }

            ViewSlot {
                id: allSlot
                s: root.s
                name: "all"
                live: root.active && root.all
                current: root.active && root.all
                direction: root.direction
                barActive: root.active
                animate: root.active
                fieldText: root.allText
                maximumHeight: stage.height
                sourceComponent: KitSearch { s: root.s }
                gifts: ({ sources: root.sources, labels: root.labels })

                Binding { target: allSlot.view; property: "height"; value: stage.height; when: allSlot.view !== null }
            }
        }

        Connections {
            target: root.sectionView
            ignoreUnknownSignals: true
            function onPageIdChanged() {
                if (root.active && root.sectionView)
                    memory.set("frame.page." + root.section, String(root.sectionView.pageId || ""));
            }
        }

        Connections {
            target: root.view
            ignoreUnknownSignals: true
            function onCloseRequested() { root.requestClose(); }
            function onHud(text) { foot.flash(text, false); }
            function onAlert(text) { foot.flash(text, true); }
            function onMenuRequested() { root.openPanel(); }
            function onQueryRequested(text) { field.text = String(text || ""); }
            function onOpenRequested(section, page, query, args) { root.openSection(section, page, true, query, args); }
        }

        Rectangle {
            x: 1
            width: parent.width - 2
            anchors.bottom: foot.top
            height: 1
            color: Look.rule
            z: 3
        }

        KitFoot {
            id: foot
            x: 0
            y: parent.height - height
            width: parent.width
            height: implicitHeight
            s: root.s
            active: root.active
            context: root.view && root.view.status !== undefined ? String(root.view.status || "") : ""
            contextItem: root.view && root.view.statusItem ? root.view.statusItem : null
            primary: root.primaryAction
            secondary: root.secondaryAction
            hasActions: root.panelActions.length > 0
            panelOpen: panel.open
            onPrimaryClicked: root.call("activate")
            onSecondaryClicked: root.call("activateSecondary")
            onActionsClicked: panel.open ? root.closePanel() : root.openPanel()
        }

        // a press anywhere else on the page puts the panel away
        MouseArea {
            anchors.fill: parent
            z: 4
            visible: panel.open
            acceptedButtons: Qt.AllButtons
            onPressed: root.closePanel()
        }

        ActionPanel {
            id: panel
            z: 5
            anchors.right: parent.right
            anchors.rightMargin: Math.round(10 * root.s)
            anchors.bottom: foot.top
            anchors.bottomMargin: Math.round(6 * root.s)
            s: root.s
            animate: root.active
            actions: root.panelActions
            note: root.view && root.view.actionNote ? String(root.view.actionNote) : ""
            maximumHeight: Math.max(120 * root.s, stage.height - 12 * root.s)
            onPicked: id => root.runPanel(id)
            onDismissed: root.closePanel()
        }
    }
}
