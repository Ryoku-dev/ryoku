pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/look"
import "devtools.js" as DevTools

Item {
    id: root

    required property real s
    property bool active: false
    property string query: ""
    property var args: ({})
    property real maximumHeight: height
    property var memory: null
    property var source: null
    property string pageId: ""
    readonly property var pages: []
    readonly property string title: I18n.tr("Dev tools")
    readonly property string placeholder: I18n.tr("Filter tools")
    readonly property string status: busy ? I18n.tr("Working offline") : I18n.tr("Runs offline")
    readonly property bool ownsText: inputPane.focused || modifiedPane.focused || outputPane.focused || optionArea.children.some(child => child.ownsText === true)
    readonly property var primaryAction: ({ label: generator ? I18n.tr("Generate") : toolId === "json_formatter" ? I18n.tr("Format") : I18n.tr("Run"), glyph: "play_arrow", hint: "Ctrl+Enter" })
    readonly property var secondaryAction: ({ label: I18n.tr("Copy output"), glyph: "content_copy", hint: "Ctrl+Shift+C" })
    readonly property var actions: [
        { id: "paste", label: I18n.tr("Paste as input"), glyph: "content_paste", hint: "Ctrl+Shift+V", section: "" },
        { id: "clear", label: I18n.tr("Clear"), glyph: "backspace", hint: "Ctrl+L", section: "" },
        { id: "chain", label: I18n.tr("Use output as input"), glyph: "arrow_back", section: "" }
    ]
    readonly property string actionNote: errorText
    readonly property var tools: registry.tools.filter(tool => {
        const haystack = (tool.name + " " + tool.description).toLocaleLowerCase();
        return root.query.trim().toLocaleLowerCase().split(/\s+/).every(word => haystack.includes(word));
    })
    property string toolId: "json_formatter"
    readonly property var tool: registry.byId(toolId)
    readonly property bool generator: tool?.type === "generator"
    readonly property bool diff: toolId === "text_diff"
    property var options: ({})
    property var inputs: ({})
    property var modifiedInputs: ({})
    property string inputText: ""
    property string modifiedText: ""
    property string outputText: ""
    property string errorText: ""
    property bool busy: false
    property bool waitingRandom: false
    property int requestId: 0
    property bool restored: false
    property string requestedTool: ""
    property bool choosing: false
    readonly property string separator: "\n===DIFF_SPLIT===\n"

    signal hud(string text)
    signal alert(string text)
    signal queryRequested(string text)
    signal closeRequested()
    signal menuRequested()

    function restore() {
        if (restored || !memory || !memory.ready)
            return;
        restored = true;
        choose(requestedTool || memory.get("devtools.tool", "json_formatter"));
    }
    function choose(id) {
        const next = registry.byId(id) ?? registry.tools[0];
        cancel();
        toolId = next.id;
        choosing = true;
        if (restored || requestedTool.length)
            pageId = next.id;
        choosing = false;
        options = Object.assign({}, next.defaultOptions, memory?.get("devtools.options." + next.id, {}) ?? {});
        const sample = String(next.sampleInput ?? "").split(separator);
        inputText = inputs[next.id] ?? sample[0];
        modifiedText = modifiedInputs[next.id] ?? sample[1] ?? "";
        outputText = "";
        errorText = "";
        if (restored && memory?.ready)
            memory.set("devtools.tool", next.id);
        execute();
    }
    function setInput(value) {
        if (value.length > DevTools.inputLimit(toolId)) {
            errorText = I18n.tr("Input exceeds the tool's character limit");
            return;
        }
        inputText = value;
        inputs = Object.assign({}, inputs, { [toolId]: value });
    }
    function setModified(value) {
        if (value.length > DevTools.workLimits().input) {
            errorText = I18n.tr("Input exceeds the tool's character limit");
            return;
        }
        modifiedText = value;
        modifiedInputs = Object.assign({}, modifiedInputs, { [toolId]: value });
    }
    function setOption(key, value) {
        options = Object.assign({}, options, { [key]: value });
        if (memory?.ready)
            memory.set("devtools.options." + toolId, options);
    }
    function execute() {
        if (!active || !tool || busy)
            return;
        const input = diff ? inputText + separator + modifiedText : inputText;
        const error = DevTools.inputError(toolId, input, options);
        if (error) {
            finish(requestId, { output: "", error: error });
            return;
        }
        if (generator && !random.ready) {
            waitingRandom = true;
            random.replenishSoon();
            return;
        }
        waitingRandom = false;
        busy = true;
        errorText = "";
        const ticket = ++requestId;
        if (toolId === "regex_tester") {
            const payload = DevTools.regexRequest(input, options);
            if (payload.error)
                finish(ticket, { output: "", error: payload.error });
            else
                regex.run(ticket, payload.request);
            return;
        }
        deferred.ticket = ticket;
        deferred.tool = toolId;
        deferred.input = input;
        deferred.options = generator ? random.options(options) : options;
        deferred.start();
    }
    function cancel() {
        requestId++;
        deferred.stop();
        deferred.input = "";
        deferred.options = ({});
        regex.cancel();
        busy = false;
        waitingRandom = false;
    }
    function finish(ticket, result) {
        if (ticket !== requestId || !active)
            return;
        busy = false;
        outputText = String(result.output ?? "");
        errorText = result.error ? I18n.tr(String(result.error)) : "";
        if (generator)
            random.replenishSoon();
    }
    function activate() { execute(); }
    function activateSecondary() { copyOutput(); }
    function copyOutput() {
        if (!active || !outputText.length)
            return;
        Quickshell.clipboardText = outputText;
        hud(I18n.tr("Output copied"));
    }
    function act(id) {
        if (!active)
            return;
        if (id === "paste" && !generator)
            setInput(String(Quickshell.clipboardText ?? ""));
        else if (id === "clear") {
            cancel();
            setInput("");
            setModified("");
            outputText = "";
            errorText = "";
        } else if (id === "chain" && !generator)
            setInput(outputText);
    }
    function move(delta) {
        if (!tools.length)
            return;
        const index = tools.findIndex(entry => entry.id === toolId);
        choose(tools[Math.max(0, Math.min(tools.length - 1, index + delta))].id);
    }
    function takeFocus() { (generator ? outputPane : inputPane).grabFocus(); }
    function handleKey(event) {
        if (active && !ownsText && event.key === Qt.Key_Tab && !(event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier))) {
            takeFocus();
            return true;
        }
        return false;
    }
    function handleEscape() {
        if (!ownsText)
            return false;
        list.forceActiveFocus();
        return true;
    }
    function cyclePane(backwards) {
        const panes = generator ? [outputPane] : diff ? [inputPane, modifiedPane, outputPane] : [inputPane, outputPane];
        const current = panes.findIndex(pane => pane.focused);
        panes[(current + (backwards ? panes.length - 1 : 1)) % panes.length].grabFocus();
    }

    onPageIdChanged: {
        if (choosing)
            return;
        if (registry.byId(pageId)) {
            requestedTool = pageId;
            choose(pageId);
        } else if (pageId.length && !registry.byId(pageId))
            pageId = toolId;
    }
    onActiveChanged: {
        if (!active)
            cancel();
        else if (!outputText.length)
            execute();
    }
    onQueryChanged: if (tools.length && !tools.some(entry => entry.id === toolId)) choose(tools[0].id)
    Connections { target: root.memory; function onReadyChanged() { root.restore(); } }
    onMemoryChanged: restore()
    Component.onCompleted: { if (!restored) choose(requestedTool || toolId); restore(); }
    Component.onDestruction: cancel()

    DevToolsRegistry { id: registry }
    SecureRandom {
        id: random
        enabled: root.active && root.generator
        onPoolReady: if (root.waitingRandom) root.execute()
        onErrorChanged: if (error.length && root.generator) { root.waitingRandom = false; root.errorText = error; }
    }
    Timer {
        id: deferred
        interval: 1
        property int ticket: 0
        property string tool: ""
        property string input: ""
        property var options: ({})
        onTriggered: {
            if (ticket !== root.requestId || !root.active)
                return;
            let result;
            try {
                result = DevTools.runTool(tool, input, options);
            } catch (error) {
                result = { output: "", error: String(error.message || error) };
            }
            input = "";
            options = ({});
            root.finish(ticket, result);
        }
    }
    RegexJob {
        id: regex
        runnerPath: Quickshell.shellPath("modules/kit/sections/devtools/RegexRunner.qml")
        onFinished: (ticket, result) => root.finish(ticket, result)
    }
    Shortcut { enabled: root.active && root.ownsText; sequences: ["Ctrl+Return", "Ctrl+Enter"]; onActivated: root.execute() }
    Shortcut { enabled: root.active && root.ownsText; sequence: "Ctrl+Shift+C"; onActivated: root.copyOutput() }
    Shortcut { enabled: root.active && root.ownsText; sequence: "Ctrl+Shift+V"; onActivated: root.act("paste") }
    Shortcut { enabled: root.active && root.ownsText; sequence: "Ctrl+L"; onActivated: root.act("clear") }

    Item {
        id: list
        anchors { left: parent.left; top: parent.top; bottom: parent.bottom; margins: 10 * root.s }
        width: Math.min(300 * root.s, root.width * 0.27)
        activeFocusOnTab: true
        Keys.onUpPressed: root.move(-1)
        Keys.onDownPressed: root.move(1)
        Keys.onReturnPressed: root.takeFocus()
        Keys.onEnterPressed: root.takeFocus()
        Keys.onTabPressed: root.takeFocus()
        Shape { anchors.fill: parent; visible: Look.boxed; fill: Look.card; strokeColor: Look.cardStroke; strokeWidth: root.s; cut: Look.cutCard * root.s }
        Flickable {
            anchors.fill: parent
            contentHeight: toolColumn.height
            clip: true
            Column {
                id: toolColumn
                width: parent.width
                Repeater {
                    model: root.tools
                    delegate: Item {
                        id: row
                        required property var modelData
                        width: toolColumn.width
                        height: 38 * root.s
                        readonly property bool selected: modelData.id === root.toolId
                        Shape {
                            anchors.fill: parent
                            fill: row.selected ? Look.plate : hover.hovered ? Look.hover : "transparent"
                            strokeColor: row.selected ? Look.plateStroke : "transparent"
                            strokeWidth: root.s
                            cut: Look.cutRow * root.s
                            Behavior on fill { ColorAnimation { duration: Look.dur(Look.plateMs) } }
                        }
                        Text {
                            anchors { fill: parent; leftMargin: 12 * root.s; rightMargin: 8 * root.s }
                            verticalAlignment: Text.AlignVCenter
                            text: row.modelData.name
                            color: row.selected ? Look.plateInk : Lithic.fg2
                            font { family: Look.sans; pixelSize: 13 * root.s; weight: row.selected ? Font.Medium : Font.Normal }
                            elide: Text.ElideRight
                        }
                        HoverHandler { id: hover }
                        TapHandler { onTapped: { root.choose(row.modelData.id); list.forceActiveFocus(); } }
                    }
                }
                Text {
                    visible: !root.tools.length
                    width: parent.width
                    text: I18n.tr("No matching tools")
                    color: Lithic.fg2
                    font { family: Look.sans; pixelSize: 13 * root.s }
                    wrapMode: Text.Wrap
                }
            }
        }
    }
    Shape {
        id: workspace
        anchors { left: list.right; leftMargin: 10 * root.s; right: parent.right; top: parent.top; bottom: parent.bottom; rightMargin: 10 * root.s; topMargin: 10 * root.s; bottomMargin: 10 * root.s }
        fill: Look.card
        strokeColor: Look.cardStroke
        strokeWidth: root.s
        cut: Look.cutCard * root.s

        Column {
            id: header
            anchors { left: parent.left; right: parent.right; top: parent.top; margins: 22 * root.s }
            spacing: 7 * root.s
            Text {
                width: parent.width
                text: root.tool?.name ?? ""
                color: Lithic.fg
                font { family: Look.sans; pixelSize: 16 * root.s; weight: Font.Medium }
                elide: Text.ElideRight
            }
            Text {
                width: parent.width
                text: root.tool?.description ?? ""
                color: Lithic.fg2
                font { family: Look.sans; pixelSize: 12 * root.s }
                wrapMode: Text.Wrap
            }
        }
        Flow {
            id: optionArea
            anchors { top: header.bottom; topMargin: 16 * root.s; left: header.left; right: header.right }
            spacing: 8 * root.s
            Repeater {
                model: root.tool?.options ?? []
                delegate: ToolOption {
                    required property var modelData
                    s: root.s
                    option: modelData
                    value: root.options[modelData.id] ?? modelData.default
                    onChanged: value => root.setOption(modelData.id, value)
                }
            }
        }
        Item {
            id: editors
            anchors { top: optionArea.bottom; topMargin: 16 * root.s; left: header.left; right: header.right; bottom: parent.bottom; bottomMargin: 22 * root.s }
            Item {
                id: inputColumn
                visible: !root.generator
                width: visible ? (parent.width - 14 * root.s) / 2 : 0
                height: parent.height
                TextPane {
                    id: inputPane
                    s: root.s
                    width: parent.width
                    height: root.diff ? (parent.height - 12 * root.s) / 2 : parent.height
                    label: root.diff ? I18n.tr("Original") : I18n.tr("Input")
                    text: root.inputText
                    onEdited: value => root.setInput(value)
                    maximumLength: DevTools.inputLimit(root.toolId)
                    onLimitExceeded: root.errorText = I18n.tr("Input exceeds the tool's character limit")
                    onCycle: backwards => root.cyclePane(backwards)
                }
                TextPane {
                    id: modifiedPane
                    s: root.s
                    visible: root.diff
                    anchors { top: inputPane.bottom; topMargin: 12 * root.s; bottom: parent.bottom }
                    width: parent.width
                    label: I18n.tr("Modified")
                    text: root.modifiedText
                    onEdited: value => root.setModified(value)
                    maximumLength: DevTools.workLimits().input
                    onLimitExceeded: root.errorText = I18n.tr("Input exceeds the tool's character limit")
                    onCycle: backwards => root.cyclePane(backwards)
                }
            }
            TextPane {
                id: outputPane
                s: root.s
                anchors { left: inputColumn.visible ? inputColumn.right : parent.left; leftMargin: inputColumn.visible ? 14 * root.s : 0; right: parent.right; top: parent.top; bottom: parent.bottom }
                label: I18n.tr("Output")
                readOnly: true
                error: root.errorText.length > 0
                text: root.errorText || root.outputText
                placeholder: root.waitingRandom ? I18n.tr("Preparing secure random source…") : root.busy ? I18n.tr("Working…") : I18n.tr("Ctrl+Enter to run")
                onCycle: backwards => root.cyclePane(backwards)
            }
        }
    }
}
