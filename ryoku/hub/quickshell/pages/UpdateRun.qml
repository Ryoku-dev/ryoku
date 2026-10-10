pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

// One update run on the Updates page: the headline, the progress track, the
// steps as a timeline with how long each took, the line the running step last
// printed, and the run's own watchdog read of its process tree. `run` is the
// page, which owns the run-state; this file only draws it.
//
// It covers starting, running, done and failed: a settled run keeps its
// timeline, so a failure shows where it stopped and a success what it did.
Column {
    id: view

    required property var run

    spacing: Tokens.s5

    readonly property bool settled: view.run.phase === "done" || view.run.phase === "error"
    readonly property bool stopped: view.run.phase === "error" && view.run.errorMsg === "stopped by request"
    readonly property var watch: view.run.watch || ({})
    readonly property bool quiet: view.run.phase === "running" && view.watch.state === "quiet"
    readonly property bool stalled: view.run.phase === "running" && (view.watch.state === "stalled" || view.run.unresponsive)

    function since(ms) {
        return ms > 0 ? view.run.human(view.run.now - ms) : "";
    }

    // ── headline: the step in flight (or the outcome) and the run's clock ──
    Item {
        width: view.width
        height: Math.max(head.implicitHeight, clock.implicitHeight)

        Row {
            id: head
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            // the heartbeat while the run works, the outcome once it settles
            Item {
                width: 14; height: 14
                anchors.verticalCenter: parent.verticalCenter
                Rectangle {
                    anchors.centerIn: parent
                    visible: !view.settled
                    width: 8; height: 8; radius: 4
                    color: Tokens.ink
                    SequentialAnimation on opacity {
                        running: !view.settled && !Tokens.reduceMotion
                        loops: Animation.Infinite
                        NumberAnimation { to: 0.25; duration: 600; easing.type: Easing.InOutSine }
                        NumberAnimation { to: 1.0; duration: 600; easing.type: Easing.InOutSine }
                    }
                }
                Rectangle {
                    anchors.centerIn: parent
                    visible: !view.settled
                    width: 14; height: 14; radius: 7
                    color: "transparent"
                    border.width: 1; border.color: Tokens.line
                }
                Text {
                    anchors.centerIn: parent
                    visible: view.settled
                    text: view.run.phase === "done" ? "\u2713" : (view.stopped ? "\u25a0" : "\u2715")
                    color: Tokens.ink; font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow; font.weight: Font.Bold
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: view.run.phase === "done" ? I18n.tr("Ryoku is up to date")
                    : view.stopped ? I18n.tr("Update stopped")
                    : view.run.phase === "error" ? (view.run.label !== "" ? I18n.tr("Update failed while %1").arg(view.run.label.toLowerCase()) : I18n.tr("Update failed"))
                    : view.run.starting && view.run.phase === "idle" ? I18n.tr("Starting the update\u2026")
                    : view.run.label !== "" ? I18n.tr(view.run.label) + "\u2026" : I18n.tr("Preparing\u2026")
                color: Tokens.ink; font.family: Tokens.display
                font.pixelSize: Tokens.fValue
            }
        }
        Text {
            id: clock
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            visible: view.run.started > 0
            text: view.settled && view.run.beat > 0 ? view.run.human(view.run.beat - view.run.started) : view.since(view.run.started)
            color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall
        }
    }

    // ── the shared progress spec: a hairline track, a square ink fill ──
    Item {
        width: view.width
        height: 12

        Rectangle {
            id: track
            anchors.left: parent.left; anchors.right: pct.left
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            height: 3
            color: Tokens.lineSoft
            antialiasing: false

            Rectangle {
                anchors.left: parent.left; anchors.top: parent.top; anchors.bottom: parent.bottom
                width: parent.width * view.run.fillFraction
                color: view.run.phase === "error" ? Tokens.inkMuted : Tokens.ink
                antialiasing: false
                Behavior on width { NumberAnimation { duration: Tokens.durNormal; easing.type: Easing.OutCubic } }

                // a glint travelling the filled part while the run works
                Rectangle {
                    visible: !view.settled && parent.width > 40 && !Tokens.reduceMotion
                    width: 40; height: parent.height
                    gradient: Gradient {
                        orientation: Gradient.Horizontal
                        GradientStop { position: 0; color: "transparent" }
                        GradientStop { position: 0.5; color: Tokens.paper }
                        GradientStop { position: 1; color: "transparent" }
                    }
                    opacity: 0.55
                    NumberAnimation on x {
                        running: !view.settled && !Tokens.reduceMotion
                        from: -40; to: track.width
                        duration: 1800; loops: Animation.Infinite
                        easing.type: Easing.InOutQuad
                    }
                }
            }
        }
        Text {
            id: pct
            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
            text: Math.round(view.run.fillFraction * 100) + "%"
            color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall
        }
    }

    // ── the steps, as a timeline on the same rail the commit list uses ──
    Column {
        id: steps
        width: view.width
        spacing: 0

        Repeater {
            model: view.run.steps

            delegate: Item {
                id: st
                required property var modelData
                required property int index

                readonly property string state: st.modelData.state || "pending"
                readonly property bool live: st.state === "running" && !view.settled
                readonly property bool isLast: st.index === view.run.steps.length - 1
                readonly property real nodeY: 18

                width: steps.width
                height: 36 + (st.live ? liveCol.implicitHeight + Tokens.s2 : 0)
                Behavior on height { NumberAnimation { duration: Tokens.move; easing.type: Easing.OutCubic } }

                Rectangle { // rail above the node
                    x: 6; width: 1; y: 0; height: st.nodeY - 6
                    color: Tokens.line; visible: st.index > 0
                }
                Rectangle { // rail below the node
                    x: 6; width: 1; y: st.nodeY + 6; height: st.height - st.nodeY - 6
                    color: Tokens.line; visible: !st.isLast
                }

                // the node: filled when done, a beating ring while running, a
                // struck square when it failed, a faint ring ahead
                Rectangle {
                    x: 2; y: st.nodeY - 4
                    width: 9; height: 9
                    radius: st.state === "failed" ? 1 : 4.5
                    color: st.state === "ok" ? Tokens.ink
                        : st.state === "failed" ? Tokens.bone
                        : "transparent"
                    border.width: Tokens.border
                    border.color: st.state === "pending" || st.state === "skipped" ? Tokens.line : Tokens.ink
                    Rectangle {
                        anchors.centerIn: parent
                        visible: st.live
                        width: 5; height: 5; radius: 2.5
                        color: Tokens.ink
                        SequentialAnimation on opacity {
                            running: st.live && !Tokens.reduceMotion
                            loops: Animation.Infinite
                            NumberAnimation { to: 0.2; duration: 600 }
                            NumberAnimation { to: 1.0; duration: 600 }
                        }
                    }
                }

                Text {
                    id: stLabel
                    anchors.left: parent.left; anchors.leftMargin: 26
                    anchors.right: stTime.left; anchors.rightMargin: Tokens.s3
                    y: st.nodeY - height / 2
                    text: I18n.tr(st.modelData.label)
                    color: st.live || st.state === "failed" ? Tokens.ink
                        : st.state === "ok" ? Tokens.inkDim : Tokens.inkFaint
                    font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                    font.weight: st.live ? Font.DemiBold : Font.Normal
                    font.strikeout: st.state === "skipped"
                    elide: Text.ElideRight
                }
                Text {
                    id: stTime
                    anchors.right: parent.right
                    y: st.nodeY - height / 2
                    text: st.live ? view.since(st.modelData.began || 0)
                        : st.state === "skipped" ? I18n.tr("skipped")
                        : (st.modelData.took || 0) > 0 ? view.run.human(st.modelData.took)
                        : (view.run.snapshotsSupported && st.state === "ok" && st.modelData.key === "snapshot" && view.run.snapshot !== "" ? "#" + view.run.snapshot : "")
                    color: st.live ? Tokens.ink : Tokens.inkFaint
                    font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                }

                // under the running step: what it is doing right now
                Column {
                    id: liveCol
                    visible: st.live
                    anchors.left: parent.left; anchors.leftMargin: 26
                    anchors.right: parent.right
                    y: st.nodeY + 12
                    spacing: Tokens.s1

                    Text {
                        width: parent.width
                        text: view.run.activity !== "" ? view.run.activity : I18n.tr("working\u2026")
                        color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        visible: view.quiet
                        text: I18n.tr("quiet for %1  \u00b7  %2").arg(view.run.human((view.watch.quiet || 0) * 1000)).arg(view.watch.current || I18n.tr("waiting"))
                        color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
                        elide: Text.ElideRight
                    }
                }
            }
        }
    }

    // ── a step that stopped moving: say so, name what it waits on, and offer
    // the two ways out ──
    Row {
        width: view.width
        visible: view.stalled && !view.run.keepWaiting
        spacing: Tokens.s4

        Rectangle { width: 2; height: stallCol.implicitHeight; color: Tokens.ink; antialiasing: false }

        Column {
            id: stallCol
            width: parent.width - 2 - Tokens.s4
            spacing: Tokens.s2

            Text {
                text: view.run.unresponsive ? I18n.tr("THE UPDATE STOPPED RESPONDING")
                    : I18n.tr("NO PROGRESS FOR %1").arg(view.run.human((view.watch.quiet || 0) * 1000).toUpperCase())
                color: Tokens.ink; font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                font.letterSpacing: Tokens.trackMark
            }
            Text {
                width: parent.width
                text: view.run.unresponsive
                    ? I18n.tr("Its process is still there but has not reported in for %1.").arg(view.since(view.run.beat))
                    : I18n.tr("%1 has printed nothing and used no CPU. It may be waiting on something that will not come.").arg(view.watch.current || I18n.tr("The current step"))
                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap; lineHeight: 1.3
            }
            Text {
                width: parent.width
                visible: !!view.watch.pkg
                text: I18n.tr("A package transaction is part of this step. Stopping lets it finish on its own; only the rest of the update stops.")
                color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap; lineHeight: 1.3
            }
            Row {
                topPadding: Tokens.s1
                spacing: Tokens.s3
                Btn { text: I18n.tr("KEEP WAITING"); compact: true; onAct: view.run.keepWaiting = true }
                Btn { text: I18n.tr("STOP UPDATE"); compact: true; primary: true; onAct: view.run.stopUpdate() }
            }
        }
    }

    // ── the failure, in its own words ──
    Text {
        width: view.width
        visible: view.run.phase === "error" && !view.stopped && view.run.errorMsg !== ""
        text: view.run.errorMsg
        color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall
        lineHeight: 1.35; wrapMode: Text.WordWrap
    }

    // ── the run's narrative: what it found and did along the way ──
    Column {
        width: view.width
        spacing: Tokens.s1
        visible: view.run.logLines.length > 0

        Text {
            bottomPadding: Tokens.s1
            text: I18n.tr("ALONG THE WAY")
            color: Tokens.inkFaint; font.family: Tokens.ui
            font.pixelSize: Tokens.fTiny; font.weight: Font.Medium
            font.letterSpacing: Tokens.trackMark
        }
        Repeater {
            model: view.run.logLines.slice(-5)
            delegate: Text {
                required property var modelData
                width: view.width
                text: "\u2022  " + modelData
                color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                elide: Text.ElideRight
            }
        }
    }
}
