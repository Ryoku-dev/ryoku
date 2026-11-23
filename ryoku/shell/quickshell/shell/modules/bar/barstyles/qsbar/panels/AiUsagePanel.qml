import QtQuick
import "../modules"
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Ryoku.Ui.Singletons

// AI usage panel in the omarchy-agent-usage presentation: one stacked card per
// provider (Claude Code · OpenAI Codex · OpenCode · Grok). Each card carries the
// provider mark, its weekly allowance used and reset countdown, a full-width
// weekly meter, the prorated-pace line with the expected-used figure, a LAST 7
// DAYS token bar chart, and the remaining limit windows. A provider reddens
// when it is behind pace. Rendered from root.ai* — the single shared parse in
// Theme.qml that the bar chips read too, so the two views can never drift.
PanelWindow {
    id: aiPanel
    required property var root

    screen: root.activePopupScreen

    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-ai-usage"

    readonly property int barBottom: root.v2BarHeight
    readonly property int gap: 6

    // ── Claude ──
    readonly property bool   clHas:       root.aiClHas
    readonly property bool   clFresh:     root.aiClFresh
    readonly property int    clPct5h:     root.aiClPct5h
    readonly property int    clPct7d:     root.aiClPct7d
    readonly property int    clReset5hTs: root.aiClReset5hTs
    readonly property int    clReset7dTs: root.aiClReset7dTs
    readonly property var    clRecent:    root.aiClRecent
    readonly property var    clExtras:    clHas ? [{ label: "5h", pct: clPct5h, resetTs: clReset5hTs }] : []

    // ── Codex ──
    readonly property bool   cxHas:       root.aiCxHas
    readonly property bool   cxFresh:     root.aiCxFresh
    readonly property int    cxPct7d:     root.aiCxPct7d
    readonly property int    cxReset7dTs: root.aiCxReset7dTs
    readonly property int    cxPrimaryPct: root.aiCxPrimaryPct
    readonly property int    cxPrimaryResetTs: root.aiCxPrimaryResetTs
    readonly property string cxPlan:      root.aiCxPlan
    readonly property var    cxWindows:   root.aiCxWindows || []
    readonly property var    cxRecent:    root.aiCxRecent
    readonly property string cxLimitStatus: root.aiCxLimitStatus
    readonly property string cxLimitReachedType: root.aiCxLimitReachedType
    // number/countdown shown in the header: weekly when reported, else primary.
    readonly property int    cxPct:       cxReset7dTs > 0 ? cxPct7d : cxPrimaryPct
    readonly property int    cxResetTs:   cxReset7dTs > 0 ? cxReset7dTs : cxPrimaryResetTs
    readonly property var    cxExtras: {
        var a = []
        for (var i = 0; i < cxWindows.length; i++) {
            var w = cxWindows[i] || {}
            if (w.minutes !== 10080)
                a.push({ label: String(w.label || "window"), pct: w.pct || 0, resetTs: w.resetTs || 0 })
        }
        return a
    }

    // ── OpenCode ──
    readonly property bool   ocHas:       root.aiOcHas
    readonly property bool   ocFresh:     root.aiOcFresh
    readonly property int    ocPct5h:     root.aiOcPct5h
    readonly property int    ocPct7d:     root.aiOcPct7d
    readonly property string ocPlan:      root.aiOcPlan
    readonly property string ocModel:     root.aiOcModel
    readonly property var    ocModels:    root.aiOcModels
    readonly property var    ocRecent:    root.aiOcRecent
    readonly property var    ocExtras:    ocHas ? [{ label: "5h", pct: ocPct5h, resetTs: 0 }] : []

    // ── Grok ── (no allowance on disk: tokens and API value, no meter)
    readonly property bool   gkHas:       root.aiGkHas
    readonly property bool   gkFresh:     root.aiGkFresh
    readonly property var    gkModels:    root.aiGkModels
    readonly property var    gkRecent:    root.aiGkRecent
    readonly property string gkSummary:   I18n.tr("%1 tokens today · %2").arg(root.aiTokenCount(root.aiGkToday)).arg(root.aiUsd(root.aiGkCostToday))
                                          + (gkFresh ? "" : I18n.tr("  (stale)"))

    // The panel is the detail view for the pill, so it answers the same choice:
    // an agent the user took off the bar does not come back here. A chosen agent
    // still needs data before it earns a card.
    readonly property bool   clShow:      root.aiToolShown("claude")   && clHas
    readonly property bool   cxShow:      root.aiToolShown("codex")    && cxHas
    readonly property bool   ocShow:      root.aiToolShown("opencode") && ocHas
    readonly property bool   gkShow:      root.aiToolShown("grok")     && gkHas
    readonly property bool   anyShow:     clShow || cxShow || ocShow || gkShow

    readonly property real reveal: root.aiUsageReveal
    visible: reveal > 0.001
    WlrLayershell.keyboardFocus: root.aiUsageVisible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    MouseArea {
        anchors.fill: parent
        onClicked: root.aiUsageVisible = false
    }

    // ── one provider card ──────────────────────────────────────────────
    component ProviderCard: Column {
        id: pcard
        property string name: ""
        property bool has: false
        property bool fresh: true
        property bool tinted: true   // false ⇒ show the logo in its own colours (Claude)
        property url logo: ""
        property size logoSize: Qt.size(18, 18)
        property real markW: 18
        property real markH: 18
        property int pct: 0          // weekly (or best) percent used
        property int resetTs: 0      // reset shown in the header countdown
        property int paceTs: 0       // genuine weekly reset for pace (0 ⇒ none)
        property string plan: ""
        property string emptyText: I18n.tr("no data")
        property bool metered: true  // false ⇒ no allowance: summary replaces "% used" and the meter
        property string summary: ""
        property var recent: []
        property var extras: []      // [{label, pct, resetTs}] of the other windows

        readonly property bool behind: aiPanel.root.aiBehindPace(pct, paceTs)
        readonly property color hi:  behind ? aiPanel.root.sealRaw : aiPanel.root.ink
        readonly property color sub: behind ? aiPanel.root.sealRaw : aiPanel.root.sumi

        width: parent ? parent.width : 0
        spacing: 8

        // header: mark + name + "NN% used · resets in …"
        Row {
            width: parent.width
            spacing: 9
            Item {
                anchors.verticalCenter: parent.verticalCenter
                width: pcard.markW; height: pcard.markH
                Image {
                    anchors.fill: parent
                    visible: !pcard.tinted && String(pcard.logo) !== ""
                    source: pcard.logo
                    sourceSize: pcard.logoSize
                    fillMode: Image.PreserveAspectFit
                    smooth: true; mipmap: true
                }
                Image {
                    anchors.fill: parent
                    visible: pcard.tinted && String(pcard.logo) !== ""
                    source: pcard.logo
                    sourceSize: pcard.logoSize
                    fillMode: Image.PreserveAspectFit
                    smooth: true; mipmap: true
                    layer.enabled: true
                    layer.effect: ShaderEffect {
                        property color tintColor: pcard.hi
                        fragmentShader: Qt.resolvedUrl("../shaders/logo-tint.frag.qsb")
                    }
                }
            }
            Column {
                spacing: 2
                UiText {
                    text: pcard.name + (pcard.plan ? "  · " + pcard.plan : "")
                    color: pcard.hi
                    font.family: aiPanel.root.mono; font.pixelSize: 14; font.weight: Font.Medium
                }
                UiText {
                    text: !pcard.has ? pcard.emptyText
                        : !pcard.metered ? pcard.summary
                        : (I18n.tr("%1% used").arg(pcard.pct)
                           + (pcard.resetTs > 0 ? I18n.tr(" · resets in %1").arg(aiPanel.root.aiFmtReset(pcard.resetTs)) : "")
                           + (pcard.fresh ? "" : I18n.tr("  (stale)")))
                    color: pcard.sub
                    font.family: aiPanel.root.mono; font.pixelSize: 11
                }
            }
        }

        // weekly meter
        Rectangle {
            visible: pcard.has && pcard.metered
            width: parent.width; height: 6; radius: 3
            color: Qt.rgba(aiPanel.root.seal.r, aiPanel.root.seal.g, aiPanel.root.seal.b, 0.15)
            Rectangle {
                width: parent.width * Math.max(0, Math.min(100, pcard.pct)) / 100
                height: parent.height; radius: parent.radius
                color: pcard.behind ? aiPanel.root.sealRaw : aiPanel.root.seal
                Behavior on width { NumberAnimation { duration: 300 } }
            }
        }

        // prorated pace · expected-used
        Item {
            visible: pcard.has && pcard.paceTs > 0
            width: parent.width; height: 14
            UiText {
                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                text: aiPanel.root.aiPaceText(pcard.pct, pcard.paceTs)
                color: pcard.behind ? aiPanel.root.sealRaw : aiPanel.root.green
                font.family: aiPanel.root.mono; font.pixelSize: 11
            }
            UiText {
                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Expected %1% used").arg(100 - aiPanel.root.aiExpectedPct(pcard.paceTs))
                color: aiPanel.root.sumi
                font.family: aiPanel.root.mono; font.pixelSize: 11
            }
        }

        // LAST 7 DAYS token bar chart
        Column {
            visible: pcard.has && pcard.recent && pcard.recent.length > 0
                     && aiPanel.root.aiRecentTotal(pcard.recent) > 0
            width: parent.width
            spacing: 5
            UiText {
                text: I18n.tr("LAST 7 DAYS · %1 TOKENS").arg(aiPanel.root.aiTokenCount(aiPanel.root.aiRecentTotal(pcard.recent)))
                color: aiPanel.root.sumiHi
                font.family: aiPanel.root.mono; font.pixelSize: 10; font.letterSpacing: 1; font.weight: Font.Medium
            }
            Row {
                id: chartRow
                width: parent.width
                spacing: 4
                Repeater {
                    model: pcard.recent
                    delegate: Column {
                        id: dayCol
                        required property var modelData
                        readonly property real tokens: aiPanel.root.aiDayTokens(modelData)
                        readonly property real peak: Math.max(1, aiPanel.root.aiRecentPeak(pcard.recent))
                        width: (chartRow.width - chartRow.spacing * 6) / 7
                        spacing: 2
                        UiText {
                            width: parent.width
                            text: dayCol.tokens > 0 ? aiPanel.root.aiTokenCount(dayCol.tokens) : "0"
                            color: aiPanel.root.sumi
                            font.family: aiPanel.root.mono; font.pixelSize: 9
                            horizontalAlignment: Text.AlignHCenter; elide: Text.ElideRight
                        }
                        Item {
                            width: parent.width; height: 28
                            Rectangle {
                                anchors.bottom: parent.bottom
                                anchors.horizontalCenter: parent.horizontalCenter
                                width: Math.max(8, parent.width - 8)
                                height: dayCol.tokens > 0 ? Math.max(2, parent.height * dayCol.tokens / dayCol.peak) : 0
                                radius: 2
                                color: pcard.behind ? aiPanel.root.sealRaw : aiPanel.root.seal
                                opacity: 0.75
                                Behavior on height { NumberAnimation { duration: 300 } }
                            }
                        }
                        UiText {
                            width: parent.width
                            text: aiPanel.root.aiDayLabel(dayCol.modelData.date)
                            color: aiPanel.root.sumi
                            font.family: aiPanel.root.mono; font.pixelSize: 9
                            horizontalAlignment: Text.AlignHCenter
                        }
                    }
                }
            }
        }

        // additional limit windows
        Repeater {
            model: pcard.extras
            delegate: Item {
                required property var modelData
                width: pcard.width; height: 14
                UiText {
                    anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr(modelData.label)
                    color: aiPanel.root.sumiHi
                    font.family: aiPanel.root.mono; font.pixelSize: 11
                }
                UiText {
                    anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("%1% used").arg(modelData.pct || 0)
                          + (modelData.resetTs > 0 ? " · " + aiPanel.root.aiFmtReset(modelData.resetTs) : "")
                    color: aiPanel.root.sumi
                    font.family: aiPanel.root.mono; font.pixelSize: 11
                }
            }
        }
    }

    // ── compact per-model usage row (OpenCode, Grok) ──
    component ModelUsageRow: Item {
        property string name: ""
        property string totalLabel: ""
        property string inputLabel: ""
        property string outputLabel: ""
        property string reasoningLabel: ""
        property string cacheReadLabel: ""
        property string cacheWriteLabel: ""
        property string todayLabel: ""
        property int pct: 0

        width: parent ? parent.width : 0
        height: 42

        UiText {
            id: modelName
            anchors.left: parent.left; anchors.top: parent.top
            width: parent.width * 0.68
            text: name
            elide: Text.ElideRight
            color: aiPanel.root.ink
            font.family: aiPanel.root.mono; font.pixelSize: 10; font.weight: Font.Medium
        }
        UiText {
            anchors.right: parent.right; anchors.top: parent.top
            text: totalLabel
            color: aiPanel.root.seal
            font.family: aiPanel.root.mono; font.pixelSize: 10; font.weight: Font.Medium
        }
        Rectangle {
            id: modelMeter
            anchors.left: parent.left; anchors.right: parent.right
            anchors.top: modelName.bottom; anchors.topMargin: 5
            height: 6; radius: 3
            color: Qt.rgba(aiPanel.root.seal.r, aiPanel.root.seal.g, aiPanel.root.seal.b, 0.14)
            Rectangle {
                width: parent.width * Math.max(0, Math.min(100, pct)) / 100
                height: parent.height; radius: 3
                color: aiPanel.root.seal
                Behavior on width { NumberAnimation { duration: 300 } }
            }
        }
        UiText {
            anchors.left: parent.left; anchors.right: parent.right
            anchors.top: modelMeter.bottom; anchors.topMargin: 4
            text: I18n.tr("I %1  O %2").arg(inputLabel).arg(outputLabel)
                + (reasoningLabel !== "0" ? I18n.tr("  R %1").arg(reasoningLabel) : "")
                + (cacheReadLabel !== "0" ? I18n.tr("  CR %1").arg(cacheReadLabel) : "")
                + (cacheWriteLabel !== "0" ? I18n.tr("  CW %1").arg(cacheWriteLabel) : "")
                + (todayLabel !== "0" ? I18n.tr("  today %1").arg(todayLabel) : "")
            elide: Text.ElideRight
            color: aiPanel.root.sumiHi
            font.family: aiPanel.root.mono; font.pixelSize: 9
        }
    }

    // ── per-model breakdown under a provider card (no omarchy equivalent; kept) ──
    component ModelList: Column {
        property bool shown: false
        property var models: []
        visible: shown && models.length > 0
        width: parent ? parent.width : 0
        spacing: 12
        Item {
            width: parent.width; height: 16
            UiText {
                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("MODELS")
                color: aiPanel.root.sumiHi
                font.family: aiPanel.root.mono; font.pixelSize: 10; font.letterSpacing: 1
            }
            UiText {
                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("recent")
                color: aiPanel.root.sumi
                font.family: aiPanel.root.mono; font.pixelSize: 10
            }
        }
        Repeater {
            model: parent.visible ? parent.models : []
            ModelUsageRow {
                required property var modelData
                name: modelData.name || ""
                totalLabel: modelData.totalLabel || ""
                inputLabel: modelData.inputLabel || "0"
                outputLabel: modelData.outputLabel || "0"
                reasoningLabel: modelData.reasoningLabel || "0"
                cacheReadLabel: modelData.cacheReadLabel || "0"
                cacheWriteLabel: modelData.cacheWriteLabel || "0"
                todayLabel: modelData.todayLabel || "0"
                pct: parseInt(modelData.pct) || 0
            }
        }
    }

    Rectangle {
        id: card
        width: 372
        height: Math.min(col.implicitHeight + 24, parent.height - 2 * (barBottom + gap))
        radius: reveal > 0.001 ? root.panelRadius : 0
        color: "transparent"
        border.color: root.panelBorder
        border.width: 0
        PillShadow { theme: root }
        ConnectedPanelSurface {
            root: aiPanel.root
            ownerActive: aiPanel.root.aiUsageVisible
            targetX: aiPanel.root.aiBarX
            reveal: aiPanel.reveal
        }

        x: Math.round(Math.max(6, Math.min(root.aiBarX - width / 2, parent.width - width - 6)))
        y: root.barPosition === "bottom"
            ? (parent.height - barBottom - gap - height) + 2 * (1 - aiPanel.reveal)
            : (barBottom + gap) - 2 * (1 - aiPanel.reveal)
        opacity: aiPanel.reveal
        focus: root.aiUsageVisible

        Keys.onPressed: function(event) {
            if (event.key === Qt.Key_Escape) {
                root.aiUsageVisible = false;
                event.accepted = true;
            } else if (event.key === Qt.Key_R) {
                root.regenerateAiUsage();
                event.accepted = true;
            }
        }

        MouseArea { anchors.fill: parent; onClicked: {} }

        Flickable {
            id: scroller
            anchors.fill: parent
            anchors.margins: 12
            contentWidth: width
            contentHeight: col.implicitHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds

            Column {
                id: col
                width: scroller.width
                spacing: 12

                // ── header ──
                Item {
                    width: parent.width
                    height: 24
                    UiText {
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        text: I18n.tr("AI USAGE")
                        color: root.ink
                        font.family: root.mono
                        font.pixelSize: 13
                        font.letterSpacing: 2
                        font.weight: Font.Medium
                    }
                    UiText {
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        text: "✕"
                        color: closeMa.containsMouse ? root.seal : root.sumi
                        font.pixelSize: 12
                        Behavior on color { ColorAnimation { duration: 120 } }
                        MouseArea {
                            id: closeMa
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: root.aiUsageVisible = false
                        }
                    }
                    UiText {
                        id: refreshGlyph
                        anchors.right: parent.right
                        anchors.rightMargin: 22
                        anchors.verticalCenter: parent.verticalCenter
                        text: "\u21BB"
                        color: (refreshMa.containsMouse || root.aiRefreshing) ? root.seal : root.sumi
                        font.pixelSize: 13
                        Behavior on color { ColorAnimation { duration: 120 } }
                        transformOrigin: Item.Center
                        RotationAnimation on rotation {
                            running: root.aiRefreshing
                            loops: Animation.Infinite
                            from: 0; to: 360
                            duration: 900
                            onRunningChanged: if (!running) refreshGlyph.rotation = 0
                        }
                        MouseArea {
                            id: refreshMa
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: root.regenerateAiUsage()
                        }
                    }
                }

                Rectangle { width: parent.width; height: 1; color: root.sep }

                UiText {
                    visible: !aiPanel.anyShow
                    width: parent.width
                    text: I18n.tr("no AI usage yet — run claude, codex or opencode")
                    color: root.sumiHi; font.family: root.mono; font.pixelSize: 11; wrapMode: Text.WordWrap
                }

                // ── Claude ──
                ProviderCard {
                    visible: aiPanel.clShow
                    name: "Claude"; tinted: false
                    logo: Qt.resolvedUrl("../assets/claude.svg"); logoSize: Qt.size(36, 36)
                    has: aiPanel.clHas; fresh: aiPanel.clFresh
                    pct: aiPanel.clPct7d; resetTs: aiPanel.clReset7dTs; paceTs: aiPanel.clReset7dTs
                    recent: aiPanel.clRecent; extras: aiPanel.clExtras
                }

                Rectangle { visible: aiPanel.clShow && (aiPanel.cxShow || aiPanel.ocShow); width: parent.width; height: 1; color: root.sep }

                // ── OpenAI Codex ──
                ProviderCard {
                    visible: aiPanel.cxShow
                    name: "Codex"
                    logo: Qt.resolvedUrl("../assets/codex-cli.svg"); logoSize: Qt.size(36, 36)
                    has: aiPanel.cxHas; fresh: aiPanel.cxFresh; plan: aiPanel.cxPlan
                    pct: aiPanel.cxPct; resetTs: aiPanel.cxResetTs; paceTs: aiPanel.cxReset7dTs
                    recent: aiPanel.cxRecent; extras: aiPanel.cxExtras
                }

                Rectangle { visible: aiPanel.cxShow && aiPanel.ocShow; width: parent.width; height: 1; color: root.sep }

                // ── OpenCode ──
                ProviderCard {
                    id: ocCard
                    visible: aiPanel.ocShow
                    name: "OpenCode"
                    logo: Qt.resolvedUrl("../assets/opencode-mark.svg"); logoSize: Qt.size(20, 12)
                    markW: 20; markH: 12
                    has: aiPanel.ocHas; fresh: aiPanel.ocFresh; plan: aiPanel.ocPlan
                    pct: aiPanel.ocPct7d; resetTs: 0; paceTs: 0
                    recent: aiPanel.ocRecent; extras: aiPanel.ocExtras
                }

                ModelList { shown: aiPanel.ocShow; models: aiPanel.ocModels }

                Rectangle { visible: (aiPanel.clShow || aiPanel.cxShow || aiPanel.ocShow) && aiPanel.gkShow; width: parent.width; height: 1; color: root.sep }

                // ── Grok ──
                ProviderCard {
                    visible: aiPanel.gkShow
                    name: "Grok"
                    logo: Qt.resolvedUrl("../assets/grok.svg"); logoSize: Qt.size(36, 36)
                    has: aiPanel.gkHas; fresh: aiPanel.gkFresh
                    metered: false; summary: aiPanel.gkSummary
                    recent: aiPanel.gkRecent
                }

                ModelList { shown: aiPanel.gkShow; models: aiPanel.gkModels }
            }
        }
    }

    // Usage data + polling live in Theme.qml (shared with the bar chips); this
    // panel only renders from root.ai* and bumps the refresh cadence via
    // root.aiUsageVisible.
}
