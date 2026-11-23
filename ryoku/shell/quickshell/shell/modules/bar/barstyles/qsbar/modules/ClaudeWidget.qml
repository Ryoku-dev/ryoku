import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// AI-usage bar display in the omarchy-agent-usage presentation: one inline chip
// per tracked provider (Claude Code · OpenAI Codex · OpenCode · Grok), each showing
// the provider mark, its WEEKLY allowance used, and the countdown to reset:
// "Claude 31% · 15h 59m". Grok has no allowance on disk, so its chip shows the
// tokens it spent today instead. A provider turns red when it is burning faster than
// the seven-day clock (behind pace). Click opens the AiUsagePanel; right/middle
// click regenerates. root.modClaude gates the whole display.
Item {
    id: rootMod
    required property var root

    readonly property color contentColor: root.widgetContentColor("G7", root.widgetIconColor)

    // ── Claude ──
    property bool clActive: false
    readonly property bool clFresh:   root.aiClFresh
    readonly property int  clPct7d:   root.aiClPct7d
    readonly property int  clPct5h:   root.aiClPct5h
    readonly property bool clBlocked: root.aiClBlocked
    readonly property int  clReset7dTs: root.aiClReset7dTs
    readonly property int  clReset5hTs: root.aiClReset5hTs
    readonly property string clTokens: root.aiClTokens
    readonly property string clRate:   root.aiClRate
    readonly property int  clToday:  root.aiClToday
    readonly property bool clHas:    root.aiClHas

    // ── Codex ──
    property bool cxActive: false
    readonly property bool cxFresh:  root.aiCxFresh
    readonly property int  cxPct7d:  root.aiCxPct7d
    readonly property int  cxReset7dTs: root.aiCxReset7dTs
    readonly property string cxPlan: root.aiCxPlan
    readonly property string cxRate: root.aiCxRate
    readonly property bool cxHas:    root.aiCxHas
    readonly property var  cxWindows: root.aiCxWindows || []
    readonly property int  cxPrimaryPct: root.aiCxPrimaryPct
    readonly property int  cxPrimaryResetTs: root.aiCxPrimaryResetTs
    readonly property string cxLimitStatus: root.aiCxLimitStatus
    readonly property string cxLimitReachedType: root.aiCxLimitReachedType

    // ── OpenCode ──
    property bool ocActive: false
    readonly property bool ocFresh:  root.aiOcFresh
    readonly property int  ocPct7d:  root.aiOcPct7d
    readonly property int  ocPct5h:  root.aiOcPct5h
    readonly property string ocPlan: root.aiOcPlan
    readonly property string ocTokens: root.aiOcTokens
    readonly property string ocRate:   root.aiOcRate
    readonly property string ocModel:  root.aiOcModel
    readonly property int  ocToday:  root.aiOcToday
    readonly property bool ocHas:    root.aiOcHas

    // ── Grok ──
    property bool gkActive: false
    readonly property bool gkFresh:  root.aiGkFresh
    readonly property bool gkHas:    root.aiGkHas
    readonly property int  gkToday:  root.aiGkToday
    readonly property int  gkTokens7d: root.aiGkTokens7d
    readonly property real gkCostToday: root.aiGkCostToday
    readonly property real gkCost7d: root.aiGkCost7d
    readonly property string gkModel: root.aiGkModel

    // ── which providers earn a chip ──
    // the user chooses which agents ride the bar; a chosen one still has to have
    // data (or a live session) before it takes up room.
    readonly property bool showCl: root.aiToolShown("claude")   && (clHas || clActive)
    readonly property bool showCx: root.aiToolShown("codex")    && (cxHas || cxActive)
    readonly property bool showOc: root.aiToolShown("opencode") && (ocHas || ocActive)
    readonly property bool showGk: root.aiToolShown("grok")     && (gkHas || gkActive)

    // codex reports its weekly window only sometimes; fall back to the primary
    // window for the number, but only flag pace off a genuine 7-day window.
    readonly property int  cxChipPct:  cxReset7dTs > 0 ? cxPct7d : cxPrimaryPct
    readonly property int  cxChipTs:   cxReset7dTs > 0 ? cxReset7dTs : cxPrimaryResetTs

    // The collectors ship with ryoku-rashin. Without them there is no usage to
    // read, and a running tool alone would raise an empty "0%" chip.
    property bool collectorsInstalled: false
    readonly property bool shown: root.modClaude && collectorsInstalled && (showCl || showCx || showOc || showGk)

    readonly property string tooltipText: {
        var lines = []
        if (showCl) {
            lines.push(I18n.tr("Claude Code") + (clFresh ? "" : "  · " + I18n.tr("stale, last refresh failed")))
            var cr = root.aiFmtReset(clReset5hTs)
            lines.push("5h: " + clPct5h + "%" + (cr ? "  (" + I18n.tr("reset in %1").arg(cr) + ")" : ""))
            var c7 = root.aiFmtReset(clReset7dTs)
            if (clPct7d > 0) lines.push("7d: " + clPct7d + "%" + (c7 ? "  (" + I18n.tr("reset in %1").arg(c7) + ")" : ""))
            var clp = root.aiPaceText(clPct7d, clReset7dTs)
            if (clp) lines.push(I18n.tr("pace: %1").arg(clp))
            if (clTokens)    lines.push(I18n.tr("%1 tokens").arg(clTokens) + (clRate ? "  · " + clRate : ""))
            if (clToday > 0) lines.push(I18n.tr("today: %1M tok").arg((clToday / 1e6).toFixed(2)))
        }
        if (showCx) {
            if (lines.length) lines.push("")
            lines.push(I18n.tr("OpenAI Codex") + (cxPlan ? "  (" + cxPlan + ")" : "")
                + (cxFresh ? "" : "  · " + I18n.tr("stale, last refresh failed")))
            for (var i = 0; i < cxWindows.length; i++) {
                var xw = cxWindows[i] || {}
                var xr = root.aiFmtReset(xw.resetTs || 0)
                lines.push(String(xw.label || I18n.tr("window")) + ": " + (xw.pct || 0) + "%" + (xr ? "  (" + I18n.tr("reset in %1").arg(xr) + ")" : ""))
            }
            var cxp = root.aiPaceText(cxPct7d, cxReset7dTs)
            if (cxp) lines.push(I18n.tr("pace: %1").arg(cxp))
            lines.push(I18n.tr("General limit: %1").arg(root.aiCodexStatusLabel(cxLimitStatus, cxLimitReachedType)))
            if (cxRate) lines.push(I18n.tr("Local activity (1h, incl. cached): %1").arg(cxRate))
        }
        if (showOc) {
            if (lines.length) lines.push("")
            lines.push(I18n.tr("OpenCode") + (ocPlan ? "  (" + ocPlan + ")" : "")
                + (ocFresh ? "" : "  · " + I18n.tr("stale, last refresh failed")))
            lines.push("5h: " + ocPct5h + "%  ·  7d: " + ocPct7d + "%")
            if (ocTokens) lines.push(I18n.tr("%1 tokens").arg(ocTokens) + (ocRate ? "  · " + ocRate : ""))
            if (ocToday > 0) lines.push(I18n.tr("today: %1M tok").arg((ocToday / 1e6).toFixed(2)))
            if (ocModel) lines.push(ocModel)
        }
        if (showGk) {
            if (lines.length) lines.push("")
            lines.push(I18n.tr("Grok") + (gkFresh ? "" : "  · " + I18n.tr("stale, last refresh failed")))
            lines.push(I18n.tr("today: %1 tokens · %2").arg(root.aiTokenCount(gkToday)).arg(root.aiUsd(gkCostToday)))
            lines.push(I18n.tr("7d: %1 tokens · %2").arg(root.aiTokenCount(gkTokens7d)).arg(root.aiUsd(gkCost7d)))
            lines.push(I18n.tr("Plan limit not readable locally; cost is API value"))
            if (gkModel) lines.push(gkModel)
        }
        return lines.length ? lines.join("\n") : I18n.tr("AI usage")
    }

    // keep rendered until the collapse animation finishes
    visible: implicitWidth > 0.5
    implicitWidth: shown ? row.implicitWidth + 18 : 0
    implicitHeight: 28
    opacity: shown ? 1 : 0

    Behavior on opacity { NumberAnimation { duration: 140; easing.type: Easing.OutCubic } }

    Process {
        command: ["sh", "-c", "command -v claude-usage >/dev/null"]
        running: true
        onExited: code => { rootMod.collectorsInstalled = code === 0 }
    }

    // ── process detection: drives per-provider "active" (running) state ──
    Process {
        id: detectClaude
        command: ["bash", "-c", "pgrep -x claude >/dev/null 2>&1 && echo 1 || echo 0"]
        stdout: StdioCollector { onStreamFinished: { rootMod.clActive = (this.text.trim() === "1") } }
    }
    Process {
        id: detectCodex
        command: ["bash", "-c", "pgrep -xa codex 2>/dev/null | grep -vq app-server && echo 1 || echo 0"]
        stdout: StdioCollector { onStreamFinished: { rootMod.cxActive = (this.text.trim() === "1") } }
    }
    Process {
        id: detectOpenCode
        command: ["bash", "-c", "ps -eo args | grep -E '(^|/| )opencode( |$)|opencode-ai' | grep -vE 'grep|opencode-usage' >/dev/null && echo 1 || echo 0"]
        stdout: StdioCollector { onStreamFinished: { rootMod.ocActive = (this.text.trim() === "1") } }
    }
    Process {
        id: detectGrok
        command: ["bash", "-c", "pgrep -x grok >/dev/null 2>&1 && echo 1 || echo 0"]
        stdout: StdioCollector { onStreamFinished: { rootMod.gkActive = (this.text.trim() === "1") } }
    }
    Timer {
        interval: 5000; running: root.modClaude || root.aiUsageVisible; repeat: true; triggeredOnStart: true
        onTriggered: {
            detectClaude.running = false; detectClaude.running = true
            detectCodex.running = false;  detectCodex.running = true
            detectOpenCode.running = false; detectOpenCode.running = true
            detectGrok.running = false; detectGrok.running = true
        }
    }

    // ── one provider chip: mark + WEEKLY % · reset countdown, red when behind ──
    component Chip: Row {
        id: chip
        property string name: ""
        property int pct: 0
        property int resetTs: 0     // reset used for the countdown
        property int paceTs: 0      // genuine weekly reset for pace (0 ⇒ no pace)
        property bool fresh: true   // false ⇒ the collector could not refetch
        property string valueText: "" // replaces "pct% · countdown" for a tool with no allowance
        property bool tinted: true  // false ⇒ show the logo in its own colours (Claude)
        property url logo: ""       // Codex / OpenCode vector mark
        property size logoSize: Qt.size(14, 14)
        property real markW: 14
        property real markH: 14

        readonly property bool behind: rootMod.root.aiBehindPace(pct, paceTs)
        readonly property color col: behind ? rootMod.root.sealRaw : rootMod.contentColor
        readonly property string countdown: rootMod.root.aiFmtReset(resetTs)

        // A number the collector could not refresh must not read as current: the
        // last good value stays on the bar, dimmed, and the tooltip says why.
        opacity: chip.fresh ? 1 : 0.45
        Behavior on opacity { NumberAnimation { duration: 160 } }

        spacing: 5

        Item {
            anchors.verticalCenter: parent.verticalCenter
            width: chip.markW
            height: chip.markH

            // logo in its own colours (Claude's orange starburst)
            Image {
                anchors.fill: parent
                visible: !chip.tinted
                source: chip.logo
                sourceSize: chip.logoSize
                fillMode: Image.PreserveAspectFit
                smooth: true
                mipmap: true
            }
            // logo tinted to the content/behind colour (Codex, OpenCode)
            Image {
                anchors.fill: parent
                visible: chip.tinted
                source: chip.logo
                sourceSize: chip.logoSize
                fillMode: Image.PreserveAspectFit
                smooth: true
                mipmap: true
                layer.enabled: true
                layer.effect: ShaderEffect {
                    property color tintColor: chip.col
                    fragmentShader: Qt.resolvedUrl("../shaders/logo-tint.frag.qsb")
                }
            }
        }

        UiText {
            anchors.verticalCenter: parent.verticalCenter
            visible: !rootMod.root.iconOnly("G7")
            text: chip.name
            color: chip.col
            font.family: rootMod.root.mono
            font.pixelSize: 12
            font.weight: Font.Bold
            Behavior on color { ColorAnimation { duration: 200 } }
        }
        UiText {
            anchors.verticalCenter: parent.verticalCenter
            visible: !rootMod.root.iconOnly("G7")
            text: chip.valueText || (chip.pct + "%" + (chip.countdown ? " · " + chip.countdown : ""))
            color: chip.col
            font.family: rootMod.root.mono
            font.pixelSize: 12
            Behavior on color { ColorAnimation { duration: 200 } }
        }
    }

    component Divider: Rectangle {
        anchors.verticalCenter: parent ? parent.verticalCenter : undefined
        width: 1
        height: 13
        color: Qt.rgba(rootMod.contentColor.r, rootMod.contentColor.g, rootMod.contentColor.b, 0.35)
    }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 9

        Chip {
            visible: rootMod.showCl
            name: "Claude"; tinted: false
            logo: Qt.resolvedUrl("../assets/claude.svg"); logoSize: Qt.size(28, 28)
            pct: rootMod.clPct7d; resetTs: rootMod.clReset7dTs; paceTs: rootMod.clReset7dTs
            fresh: rootMod.clFresh
        }

        Divider { visible: rootMod.showCl && rootMod.showCx }

        Chip {
            visible: rootMod.showCx
            name: "Codex"
            logo: Qt.resolvedUrl("../assets/codex-cli.svg"); logoSize: Qt.size(28, 28)
            pct: rootMod.cxChipPct; resetTs: rootMod.cxChipTs; paceTs: rootMod.cxReset7dTs
            fresh: rootMod.cxFresh
        }

        Divider { visible: (rootMod.showCl || rootMod.showCx) && rootMod.showOc }

        Chip {
            visible: rootMod.showOc
            name: "OpenCode"
            logo: Qt.resolvedUrl("../assets/opencode-mark.svg"); logoSize: Qt.size(20, 12)
            markW: 20; markH: 12
            pct: rootMod.ocPct7d; resetTs: 0; paceTs: 0
            fresh: rootMod.ocFresh
        }

        Divider { visible: (rootMod.showCl || rootMod.showCx || rootMod.showOc) && rootMod.showGk }

        Chip {
            visible: rootMod.showGk
            name: "Grok"
            logo: Qt.resolvedUrl("../assets/grok.svg"); logoSize: Qt.size(28, 28)
            pct: 0; resetTs: 0; paceTs: 0
            valueText: I18n.tr("%1 today").arg(rootMod.root.aiTokenCount(rootMod.gkToday))
            fresh: rootMod.gkFresh
        }
    }

    TooltipMixin { id: tip; root: rootMod.root; owner: rootMod; text: rootMod.tooltipText }

    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
        hoverEnabled: true; cursorShape: Qt.PointingHandCursor
        onEntered: if (shown) { root.refreshAiUsage(); tip.show() }
        onExited: { tip.hide() }
        onClicked: (mouse) => {
            tip.hide()
            if (mouse.button === Qt.LeftButton)
                root.aiUsageVisible = !root.aiUsageVisible
            else
                root.regenerateAiUsage()
        }
    }
}
