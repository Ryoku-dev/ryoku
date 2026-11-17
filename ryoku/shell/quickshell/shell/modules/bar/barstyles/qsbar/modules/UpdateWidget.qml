import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import shell.services

// One-click update glyph next to the clock. The daemon runs the background check
// and streams it on the "updates" topic (via the Updates service), so this only
// renders it: hovering shows the incoming commits and pending packages, clicking
// starts the update and opens Ryoku Settings on it, right-click forces a re-check.
Item {
    id: rootMod
    required property var root
    readonly property color contentColor: root.widgetContentColor("G8", root.seal)

    readonly property bool updateAvailable: Updates.available
    readonly property int pending: Updates.notifyRyoku ? Updates.ryokuPending : 0
    readonly property var commits: Updates.commits
    readonly property var packages: Updates.notifySystem ? Updates.packages : []

    readonly property int tooltipCommitLimit: 8

    visible: updateAvailable
    implicitWidth: updateAvailable ? 20 : 0
    implicitHeight: 28

    readonly property string tooltipText: {
        if (!updateAvailable) return I18n.tr("Ryoku is up to date")
        var lines = []
        if (pending > 0) {
            var head = I18n.tr("Ryoku update") + " · " + (pending === 1
                ? I18n.tr("%1 commit").arg(pending) : I18n.tr("%1 commits").arg(pending))
            if (Updates.channel !== "") head += " · " + Updates.channel
            lines.push(head)
            if (Updates.installed !== "" && Updates.latest !== "" && Updates.installed !== Updates.latest) {
                var from = Updates.installed, to = Updates.latest;
                if (Updates.latestName !== "" && Updates.latestName !== Updates.installedName) {
                    from = (Updates.installedName ? Updates.installedName + " " : "") + from;
                    to = Updates.latestName + " " + to;
                }
                lines.push(from + " → " + to)
            }
            var shown = Math.min(commits.length, tooltipCommitLimit)
            for (var i = 0; i < shown; i++) {
                var c = commits[i]
                var sha = c.new ? String(c.new).slice(0, 7) : ""
                var subject = String(c.name || "").trim()
                lines.push(sha !== "" ? sha + "  " + subject : subject)
            }
            if (commits.length > shown)
                lines.push(I18n.tr("+%1 more").arg(commits.length - shown))
        }
        if (packages.length > 0) {
            if (lines.length > 0) lines.push("")
            lines.push(I18n.tr("System packages: %1").arg(Updates.systemPending))
        }
        lines.push("")
        lines.push(Updates.notifySystem && Updates.systemPending > 0
            ? I18n.tr("Click to review updates") : I18n.tr("Click to update"))
        return lines.join("\n")
    }

    Connections {
        target: rootMod.root
        function onUpdateRefreshTickChanged() { Updates.check() }
    }

    // The update runs in the background and reports to Ryoku Settings, which
    // opens on its Updates page to show the run and ask for the password.
    function startUpdate() {
        if (!(Updates.notifySystem && Updates.systemPending > 0))
            Spawn.run(["ryoku", "update", "--gui"])
        Spawn.run(["ryoku-shell", "hub", "open", "updates"])
    }

    IconText {
        anchors.centerIn: parent
        text: "\uE627"   // sync
        color: rootMod.contentColor
        font.pixelSize: 15
    }

    TooltipMixin { id: tip; root: rootMod.root; owner: rootMod; text: rootMod.tooltipText }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        onEntered: tip.show()
        onExited:  tip.hide()
        onClicked: function(mouse) {
            tip.hide()
            if (mouse.button === Qt.RightButton) {
                Updates.check()
                return
            }
            rootMod.startUpdate()
        }
    }
}
