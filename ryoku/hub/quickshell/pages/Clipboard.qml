import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/desktop-dock"
import "../schema/clipboard.js" as Schema

// the clipboard panel's size and corners, and its history: what it holds,
// clearing it now, and clearing it every week
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    // the daemon owns the entries and their files, so its numbers and its clear agree
    property var stats: null
    property string verb: ""
    property bool slow: false
    property string failure: ""
    property string failedVerb: ""
    // a run outliving the page reports to nobody
    readonly property var life: ({ alive: true })

    readonly property var weeklyRow: Schema.row("clear-weekly")
    readonly property var weeklyBind: root.weeklyRow.bind
    readonly property bool weekly: Changes.value(root.weeklyBind) === true

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => r.template !== true)
    })

    function bytes(n) {
        n = Number(n) || 0;
        if (n < 1024)
            return I18n.tr("%1 B").arg(n);
        if (n < 1048576)
            return I18n.tr("%1 KB").arg((n / 1024).toFixed(1));
        if (n < 1073741824)
            return I18n.tr("%1 MB").arg((n / 1048576).toFixed(1));
        return I18n.tr("%1 GB").arg((n / 1073741824).toFixed(2));
    }

    readonly property string sizeText: !root.stats ? ""
        : root.stats.items === 1 ? I18n.tr("1 item, %1").arg(root.bytes(root.stats.bytes))
        : I18n.tr("%1 items, %2").arg(root.stats.items).arg(root.bytes(root.stats.bytes))

    readonly property string sizeDetail: {
        if (!root.stats)
            return root.verb === "stats" ? I18n.tr("Measuring what the history holds") : I18n.tr(Schema.row("history-size").detail);
        var parts = [I18n.tr("Text %1").arg(root.bytes(root.stats.textBytes)),
                     I18n.tr("pictures %1").arg(root.bytes(root.stats.imageBytes))];
        if (Number(root.stats.otherBytes) > 0)
            parts.push(I18n.tr("other %1").arg(root.bytes(root.stats.otherBytes)));
        var line = parts.join(", ");
        var starred = Number(root.stats.starred) || 0;
        return starred === 1 ? I18n.tr("%1. 1 item is starred.").arg(line)
             : starred > 1 ? I18n.tr("%1. %2 items are starred.").arg(line).arg(starred)
             : I18n.tr("%1. Nothing is starred.").arg(line);
    }

    readonly property string weeklyDetail: {
        var lead = I18n.tr(root.weeklyRow.detail);
        if (!root.stats)
            return lead;
        var last = Number(root.stats.lastPrune) || 0;
        if (last <= 0)
            return I18n.tr("%1. It has not run yet.").arg(lead);
        var days = Math.floor((Date.now() / 1000 - last) / 86400);
        return days <= 0 ? I18n.tr("%1. It last ran today.").arg(lead)
             : days === 1 ? I18n.tr("%1. It last ran yesterday.").arg(lead)
             : I18n.tr("%1. It last ran %2 days ago.").arg(lead).arg(days);
    }

    function run(verb) {
        if (root.verb !== "")
            return;
        root.verb = verb;
        root.failure = "";
        root.slow = false;
        slowMark.restart();
        var life = root.life;
        // a missing tool or a daemon that never answers ends in an error, not a spinner
        Changes.run(["ryoku-hub", "clipboard", verb], function (ok, out, err, code) {
            if (life.alive)
                root.finish(ok, out, err, code);
        }, { timeout: 15000 });
    }

    function finish(ok, out, err, code) {
        var verb = root.verb;
        root.verb = "";
        root.slow = false;
        slowMark.stop();
        if (ok) {
            try {
                root.stats = JSON.parse(out.trim());
                return;
            } catch (e) {}
        }
        root.failedVerb = verb;
        var why = err.trim().replace(/^ryoku-hub:\s*/, "").replace(/[.\s]+$/, "");
        // a tool that is missing or did not answer already says what to do next
        if (code === -1 && why !== "")
            root.failure = (verb === "prune" ? I18n.tr("The history was not cleared. %1.") : I18n.tr("Could not read the history. %1.")).arg(why);
        else if (why !== "")
            root.failure = (verb === "prune" ? I18n.tr("The history was not cleared: %1. Check that the desktop is running, then try again.")
                : I18n.tr("Could not read the history: %1. Check that the desktop is running, then try again.")).arg(why);
        else
            root.failure = verb === "prune" ? I18n.tr("The history was not cleared. Check that the desktop is running, then try again.")
                : I18n.tr("Could not read the history. Check that the desktop is running, then try again.");
    }

    function askClear() {
        var unstarred = root.stats ? Math.max(0, root.stats.items - (Number(root.stats.starred) || 0)) : -1;
        var text = unstarred < 0 ? I18n.tr("Everything you have not starred is removed. Starred items stay.")
            : unstarred === 1 ? I18n.tr("1 item you have not starred is removed. Starred items stay.")
            : I18n.tr("%1 items you have not starred are removed. Starred items stay.").arg(unstarred);
        root.hub.confirm({
            title: I18n.tr("Clear the clipboard history?"),
            text: text,
            actionText: I18n.tr("Clear history")
        }, function () { root.run("prune"); });
    }

    function own(id) {
        var r = Schema.row(id);
        return !!r && r.template === true;
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (!root.own(rowId)) {
            page.focusRow(rowId);
            return;
        }
        if (page.rowItems[rowId]) {
            page.land(rowId);
            return;
        }
        root.pendingRow = rowId;
        landRetry.tries = 0;
        landRetry.restart();
    }

    Timer {
        id: landRetry
        property int tries: 0
        interval: 60
        repeat: true
        onTriggered: {
            if (page.rowItems[root.pendingRow]) {
                stop();
                var id = root.pendingRow;
                root.pendingRow = "";
                page.land(id);
            } else if (++tries > 30) {
                stop();
            }
        }
    }

    Timer {
        id: slowMark
        interval: 1000
        onTriggered: root.slow = true
    }

    Component.onCompleted: root.run("stats")
    Component.onDestruction: root.life.alive = false

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "clipboard"
        schema: root.schema
        bodies: ({ history: historyBody })
    }

    Component {
        id: historyBody
        Slab {
            Column {
                width: parent.width

                OwnRow {
                    host: page
                    rowId: "history-size"
                    first: true
                    title: I18n.tr("History size")
                    detail: root.sizeDetail

                    Row {
                        spacing: LithicTokens.space.s2
                        Working {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: active
                            active: root.slow && root.verb === "stats"
                        }
                        Label {
                            anchors.verticalCenter: parent.verticalCenter
                            text: root.sizeText
                            typeRole: "label"
                            numeric: true
                        }
                    }
                }

                OwnRow {
                    host: page
                    rowId: "clear-now"
                    title: I18n.tr("Clear history now")
                    detail: I18n.tr(Schema.row("clear-now").detail)

                    Plaque {
                        kind: "dangerLine"
                        small: true
                        glyph: "delete_sweep"
                        text: I18n.tr("Clear")
                        accessibleName: I18n.tr("Clear history now")
                        busy: root.slow && root.verb === "prune"
                        enabled: root.verb === "" && (!root.stats || root.stats.items > (Number(root.stats.starred) || 0))
                        onClicked: root.askClear()
                    }
                }

                OwnRow {
                    host: page
                    rowId: "clear-weekly"
                    title: I18n.tr(root.weeklyRow.label)
                    detail: root.weeklyDetail
                    activatable: true
                    changed: Changes.isChanged(root.weeklyRow)
                    onActivated: Changes.set(root.weeklyBind, !root.weekly, root.weeklyRow.label, "clipboard")
                    onResetRequested: Changes.set(root.weeklyBind, Changes.defaultOf(root.weeklyRow), root.weeklyRow.label, "clipboard")

                    Toggle {
                        checked: root.weekly
                        Accessible.name: I18n.tr(root.weeklyRow.label)
                        onToggled: Changes.set(root.weeklyBind, checked, root.weeklyRow.label, "clipboard")
                    }
                }

                RowNote {
                    width: parent.width
                    topPadding: LithicTokens.space.s1
                    text: Changes.error(root.weeklyBind)
                    onRetry: Changes.retry(root.weeklyBind)
                }

                RowNote {
                    width: parent.width
                    topPadding: LithicTokens.space.s1
                    text: root.failure
                    onRetry: root.run(root.failedVerb)
                }
            }
        }
    }
}
