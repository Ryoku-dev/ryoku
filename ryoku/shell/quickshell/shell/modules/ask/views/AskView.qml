pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Dialogs
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import ".."
import "../bar"
import "../look"
import "../lib/format.js" as Format
import "../lib/root.js" as RootLogic

// Quick Ask: one question, one read-only answer from Rashin, streamed in place.
// With nothing asked it lists the recent asks. args: {question} asks at once,
// {session} opens a past answer, {context: {title, url}} asks about a page.
Item {
    id: root

    required property real s
    property bool active: false
    property string query: ""
    property var args: ({})
    property real maximumHeight: 480 * s
    property RashinClient rashin: null
    property RashinSession session: null

    readonly property string title: I18n.tr("Ask")
    readonly property string placeholder: root.context ? I18n.tr("Ask about this page")
        : root.showingAnswer ? I18n.tr("Ask something else") : I18n.tr("Ask Rashin about this machine or anything else")
    readonly property real preferredWidth: 0
    readonly property bool yieldsKeyboard: attachDialog.visible
    readonly property string status: {
        if (!root.ready)
            return "";
        if (root.busy)
            return "";
        if (root.showingAnswer && String(root.turn.via || "").length > 0)
            return I18n.tr("Answered by %1").arg(root.turn.via);
        return "";
    }
    readonly property Component statusItem: root.session && root.session.forgetting.length > 0 && !root.showingAnswer ? undoLine : null
    // the field keeps Delete and Ctrl+Z while a question is being typed
    readonly property bool fieldEmpty: root.text.length === 0
    readonly property var pastAsks: root.session ? RootLogic.pickAsks(root.session.sessions, Date.now(), 0, root.session.forgetting) : []

    readonly property string clientState: root.rashin ? root.rashin.state : "off"
    readonly property bool ready: root.clientState === "ready"
    readonly property bool missing: root.clientState === "off" || root.clientState === "not-installed"
    readonly property var turn: root.session ? root.session.state : ({ phase: "idle" })
    readonly property bool busy: root.session ? root.session.busy : false
    readonly property string text: root.query.trim()
    property bool history: true
    // a page waiting to be asked about takes the stage from the last answer
    readonly property bool showingAnswer: root.ready && !root.history && root.context === null && root.turn.phase !== "idle"
    property var attachments: []
    property var context: null
    property string pendingQuestion: ""
    property string handledArgs: ""

    readonly property Component accessory: Component {
        AskOrb {
            s: root.s
            orbSize: Math.round(24 * root.s)
            active: root.active && (root.busy || settle.running)
            phase: String(root.turn.phase || "idle")
            working: answer.workingLabel
            ink: root.busy ? Lithic.fg : Lithic.fg3
        }
    }

    readonly property var primaryAction: {
        if (root.missing)
            return { label: I18n.tr("Open Rashin settings") };
        if (!root.ready)
            return null;
        if (root.text.length > 0 || (root.context && !root.showingAnswer))
            return { label: root.context && root.text.length === 0 ? I18n.tr("Summarise this page") : I18n.tr("Ask Rashin") };
        if (root.showingAnswer) {
            if (root.busy)
                return null;
            if (root.turn.needsAgent === true)
                return { label: I18n.tr("Open in Rashin") };
            return answer.answerText.length > 0 ? { label: I18n.tr("Copy answer") } : null;
        }
        return recent.current ? { label: I18n.tr("Open answer") } : null;
    }
    readonly property var secondaryAction: null
    readonly property var actions: {
        const out = [];
        if (!root.ready)
            return out;
        if (root.showingAnswer) {
            if (root.busy)
                out.push({ id: "cancel", label: I18n.tr("Stop answering"), glyph: "stop", hint: "Ctrl+.", section: "" });
            const handOver = root.turn.needsAgent === true;
            if (answer.answerText.length > 0 && !root.busy) {
                if (handOver)
                    out.push({ id: "copy", label: I18n.tr("Copy answer"), glyph: "content_copy", hint: "Ctrl+Shift+C", section: "" });
                out.push({ id: "pin", label: I18n.tr("Pin bubble"), glyph: "keep", hint: "Ctrl+P", section: "" });
            }
            if (root.session && root.session.id.length > 0 && (!handOver || root.busy))
                out.push({ id: "open", label: I18n.tr("Open in Rashin"), glyph: "open_in_new", hint: "Ctrl+O", section: "" });
            const offers = answer.offers;
            for (var i = 0; i < offers.length; i++)
                out.push({ id: "offer:" + i, label: answer.offerLabel(offers[i]), glyph: answer.offerGlyph(offers[i]), hint: "", section: I18n.tr("From the answer") });
            out.push({ id: "history", label: I18n.tr("Recent asks"), glyph: "history", hint: "Ctrl+H", section: I18n.tr("Ask") });
        } else if (recent.current && recent.visible) {
            out.push({ id: "open-row", label: I18n.tr("Open in Rashin"), glyph: "open_in_new", hint: "Ctrl+O", section: "" });
            out.push({ id: "delete-row", label: I18n.tr("Delete this ask"), glyph: "delete", hint: root.fieldEmpty ? "Delete" : "", section: I18n.tr("Remove"), destructive: true });
        }
        if (root.session && root.session.forgetting.length > 0)
            out.push({ id: "undo-delete", label: I18n.tr("Undo the delete"), glyph: "undo", hint: root.fieldEmpty ? "Ctrl+Z" : "", section: "" });
        out.push({ id: "attach", label: I18n.tr("Attach image"), glyph: "attach_file", hint: "Ctrl+I", section: I18n.tr("Ask") });
        return out;
    }

    signal pushRequested(string view, var args)
    signal closeRequested()
    signal hud(string text)
    signal alert(string text)
    signal queryRequested(string text)
    signal pinRequested(string question, string answer)

    implicitHeight: body.implicitHeight

    function agoText(stamp) {
        const value = Date.parse(String(stamp || ""));
        if (isNaN(value))
            return "";
        const ago = Format.ago(value, Date.now());
        if (!ago || ago.unit === "now")
            return I18n.tr("Just now");
        if (ago.unit === "m")
            return I18n.tr("%1m ago").arg(ago.n);
        if (ago.unit === "h")
            return I18n.tr("%1h ago").arg(ago.n);
        return I18n.tr("%1d ago").arg(ago.n);
    }

    function attachmentPath(url) {
        var value = String(url || "");
        if (value.indexOf("file://") === 0)
            value = value.slice(7);
        try {
            return decodeURIComponent(value);
        } catch (error) {
            return value;
        }
    }

    function ask(question) {
        var text = String(question || "").trim();
        if (root.context) {
            const page = String(root.context.title || "") + " " + String(root.context.url || "");
            text = text.length > 0 ? text + "\n\n" + I18n.tr("Page: %1").arg(page.trim())
                : I18n.tr("Summarise this page: %1").arg(page.trim());
        }
        if (text.length === 0)
            return;
        if (!root.ready) {
            root.pendingQuestion = text;
            return;
        }
        if (root.busy)
            return;
        root.history = false;
        root.session.ask(text, root.attachments);
        root.attachments = [];
        root.context = null;
        root.queryRequested("");
    }

    function takeArgs() {
        const mark = JSON.stringify(root.args || ({}));
        if (mark === root.handledArgs)
            return;
        root.handledArgs = mark;
        const given = root.args || ({});
        root.context = given.context || null;
        if (given.session) {
            root.history = false;
            if (root.ready && root.session.id !== String(given.session))
                root.session.attach(String(given.session));
        } else if (String(given.question || "").trim().length > 0) {
            root.ask(given.question);
        } else if (!given.context && root.session && root.turn.phase !== "idle") {
            // coming back to Ask shows the answer that was left there
            root.history = false;
        }
    }

    function openSettings() {
        Bus.run("hub open rashin");
        root.closeRequested();
    }

    function move(delta) {
        if (root.showingAnswer)
            answer.line(delta);
        else
            recent.move(delta);
    }
    function jumpSection(delta) {
        if (root.showingAnswer)
            answer.page(delta);
        else
            recent.jumpSection(delta);
    }
    function page(delta) {
        if (root.showingAnswer)
            answer.page(delta);
        else
            recent.page(delta);
    }
    function handleEscape() { return false; }

    function activate() {
        if (root.missing) {
            root.openSettings();
        } else if (!root.ready) {
            return;
        } else if (root.text.length > 0 || (root.context && !root.showingAnswer)) {
            root.ask(root.text);
        } else if (root.showingAnswer) {
            if (root.busy)
                return;
            root.act(root.turn.needsAgent === true ? "open" : "copy");
        } else if (recent.current) {
            root.history = false;
            root.session.attach(recent.current.session);
        }
    }

    function activateSecondary() {}

    function act(id) {
        if (id === "cancel") {
            root.session.cancel();
        } else if (id === "copy") {
            if (answer.answerText.length === 0)
                return;
            Quickshell.clipboardText = answer.answerText;
            root.hud(I18n.tr("Copied"));
        } else if (id === "pin") {
            if (answer.answerText.length === 0)
                return;
            root.pinRequested(String(root.turn.question || ""), answer.answerText);
            root.hud(I18n.tr("Pinned to the desktop"));
        } else if (id === "open") {
            Spawn.run(["rashin", "open", "--session", root.session.id]);
            root.closeRequested();
        } else if (id === "open-row") {
            if (!recent.current)
                return;
            Spawn.run(["rashin", "open", "--session", recent.current.session]);
            root.closeRequested();
        } else if (id === "delete-row") {
            if (!recent.current)
                return;
            const leaving = recent.current;
            recent.keepNear(leaving.id);
            root.session.forget(leaving.session);
        } else if (id === "undo-delete") {
            root.undoDelete();
        } else if (id === "history") {
            root.history = true;
            root.session.list(20);
        } else if (id === "attach") {
            attachDialog.open();
        } else if (id.indexOf("offer:") === 0) {
            const offer = answer.offers[Number(id.slice(6))];
            if (offer)
                answer.fire(offer);
        }
    }

    onArgsChanged: root.takeArgs()
    Component.onCompleted: root.takeArgs()
    onActiveChanged: {
        if (!root.active && root.session) {
            root.session.commitForget();
            return;
        }
        if (root.active && root.ready && root.history)
            root.session.list(20);
    }
    onReadyChanged: {
        if (!root.ready)
            return;
        const given = root.args || ({});
        if (given.session && root.session.id !== String(given.session))
            root.session.attach(String(given.session));
        if (root.pendingQuestion.length > 0) {
            const question = root.pendingQuestion;
            root.pendingQuestion = "";
            root.context = null;
            root.ask(question);
        } else if (root.active && root.history) {
            root.session.list(20);
        }
    }

    // the ask comes back where it was, and takes the selection with it
    function undoDelete() {
        if (!root.session || root.session.forgetting.length === 0)
            return;
        const back = root.session.forgetting;
        root.session.undoForget();
        recent.selectId("ask:" + back);
    }
    onBusyChanged: if (!root.busy) settle.restart()

    Timer { id: settle; interval: 1700 }

    Component {
        id: undoLine
        UndoLine {
            s: root.s
            text: I18n.tr("Ask deleted")
            keyed: root.fieldEmpty
            onUndo: root.undoDelete()
        }
    }

    FileDialog {
        id: attachDialog
        title: I18n.tr("Attach images")
        fileMode: FileDialog.OpenFiles
        nameFilters: [I18n.tr("Images") + " (*.png *.jpg *.jpeg *.webp *.gif *.bmp)"]
        onAccepted: {
            const next = [];
            for (var i = 0; i < selectedFiles.length; i++)
                next.push(root.attachmentPath(selectedFiles[i]));
            root.attachments = next;
        }
    }

    Column {
        id: body
        width: parent.width

        // what rides along with the next question
        Flow {
            id: carried
            x: Math.round(14 * root.s)
            width: parent.width - x * 2
            topPadding: Math.round(8 * root.s)
            visible: root.ready && (root.attachments.length > 0 || root.context !== null)
            spacing: Math.round(6 * root.s)

            Chip {
                visible: root.context !== null
                s: root.s
                tier: "line"
                glyph: "link"
                text: root.context ? String(root.context.title || root.context.url || "") .slice(0, 48) : ""
                onClicked: root.context = null
            }
            Repeater {
                model: root.attachments

                delegate: Chip {
                    required property string modelData
                    s: root.s
                    tier: "line"
                    glyph: "image"
                    text: modelData.split("/").pop()
                    onClicked: root.attachments = root.attachments.filter(path => path !== modelData)
                }
            }
        }

        // Rashin is not there to ask
        Item {
            width: parent.width
            visible: !root.ready
            implicitHeight: stateColumn.implicitHeight + Math.round(34 * root.s)
            height: implicitHeight

            Column {
                id: stateColumn
                x: Math.round(18 * root.s)
                y: Math.round(16 * root.s)
                width: parent.width - x * 2
                spacing: Math.round(4 * root.s)

                Text {
                    width: parent.width
                    text: root.clientState === "off" ? I18n.tr("Rashin is off")
                        : root.clientState === "not-installed" ? I18n.tr("Rashin is not installed")
                        : root.clientState === "error" ? I18n.tr("Rashin did not start")
                        : I18n.tr("Connecting to Rashin")
                    color: root.clientState === "error" ? Lithic.jasper : Lithic.fg
                    font.family: Look.sans
                    font.pixelSize: Math.round(15 * root.s)
                    font.weight: Font.Medium
                    renderType: Text.NativeRendering
                }
                Text {
                    width: parent.width
                    visible: text.length > 0
                    text: root.missing ? I18n.tr("Quick Ask needs it. Turn Rashin on in Settings, under Apps. Web search and downloads work without it.")
                        : root.clientState === "error" ? String(root.rashin ? (root.rashin.errorText || I18n.tr("Rashin could not start. See %1").arg(root.rashin.logPath)) : "")
                        : ""
                    color: Lithic.fg2
                    font.family: Look.sans
                    font.pixelSize: Math.round(13 * root.s)
                    wrapMode: Text.WordWrap
                    renderType: Text.NativeRendering
                }
            }
        }

        ResultList {
            id: recent
            width: parent.width
            height: implicitHeight
            visible: root.ready && !root.showingAnswer && root.context === null && count > 0
            s: root.s
            maximumHeight: Math.max(0, root.maximumHeight - (carried.visible ? carried.height : 0))
            sections: root.ready && root.pastAsks.length > 0 ? [{
                id: "recent", title: I18n.tr("Recent asks"),
                rows: root.pastAsks.map(info => ({
                    id: "ask:" + info.id, session: String(info.id || ""), size: "two", glyph: "history",
                    title: String(info.title || info.id || "").replace(/\s+/g, " "),
                    subtitle: String(info.preview || "").replace(/\s+/g, " "),
                    trail: root.agoText(info.updated)
                }))
            }] : []
            onActivated: root.activate()
            onMenuRequested: root.menuRequested()
        }

        Item {
            width: parent.width
            visible: root.ready && !root.showingAnswer && (recent.count === 0 || root.context !== null)
            implicitHeight: Math.round(64 * root.s)
            height: implicitHeight

            Text {
                x: Math.round(18 * root.s)
                width: parent.width - x * 2
                anchors.verticalCenter: parent.verticalCenter
                text: root.context ? I18n.tr("Press Enter for a summary, or type what you want to know about this page.")
                    : I18n.tr("Type a question and press Enter. Rashin reads this machine to answer; it changes nothing.")
                color: Lithic.fg2
                font.family: Look.sans
                font.pixelSize: Math.round(13 * root.s)
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }
        }

        AskAnswer {
            id: answer
            width: parent.width
            height: implicitHeight
            visible: root.showingAnswer
            s: root.s
            session: root.session
            maximumHeight: Math.max(0, root.maximumHeight - (carried.visible ? carried.height : 0))
            onHud: text => root.hud(text)
        }
    }

    signal menuRequested()
}
