pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// one update run: its state and clock, the progress blade, the steps with how
// long each took, the question or password it waits on, and what to do once it settles
Column {
    id: view

    required property var run
    required property var hub

    spacing: LithicTokens.space.s4

    readonly property string tone: view.run.phase === "done" ? "alive"
        : view.run.stopped ? "idle"
        : view.run.phase === "error" ? "failed"
        : view.run.phase === "prompt" || view.run.phase === "auth" || view.run.stalled ? "attention"
        : "becoming"
    readonly property string claim: view.run.phase === "done" ? I18n.tr("Updated")
        : view.run.stopped ? I18n.tr("Stopped")
        : view.run.phase === "error" ? I18n.tr("Failed")
        : view.run.phase === "prompt" || view.run.phase === "auth" ? I18n.tr("Needs you")
        : view.run.stalled ? I18n.tr("Waiting")
        : view.run.starting && view.run.phase === "idle" ? I18n.tr("Starting")
        : I18n.tr("Updating")
    readonly property string headline: view.run.phase === "done" ? I18n.tr("Ryoku is up to date.")
        : view.run.stopped ? I18n.tr("The update stopped.")
        : view.run.phase === "error" ? (view.run.label !== "" ? I18n.tr("The update failed while %1.").arg(I18n.tr(view.run.label).toLowerCase()) : I18n.tr("The update failed."))
        : view.run.phase === "prompt" ? view.run.promptTitle
        : view.run.phase === "auth" ? (view.run.promptTitle !== "" ? view.run.promptTitle : I18n.tr("The update needs your password."))
        : view.run.starting && view.run.phase === "idle" ? I18n.tr("Starting the update.")
        : view.run.label !== "" ? I18n.tr(view.run.label)
        : I18n.tr("Getting ready.")

    function askStop(opener) {
        view.run.pressedBy(opener);
        view.hub.confirm({
            title: I18n.tr("Stop the update?"),
            text: view.run.watch.pkg
                ? I18n.tr("A package install is under way; it finishes on its own and only the rest of the update stops. What is already installed stays.")
                : I18n.tr("What is already installed stays. Your desktop comes back the way it was before the update paused it."),
            actionText: I18n.tr("Stop update"),
            cancelText: I18n.tr("Keep updating"),
            destructive: true
        }, function () { view.run.stop(); }, function () {});
    }

    // Escape in the password field leaves the field for Cancel, never the page: the run
    // keeps waiting and Shift+Tab returns to the field; true when it did
    function back() {
        if (!auth.visible || !password.activeFocus)
            return false;
        authCancel.forceActiveFocus(Qt.TabFocusReason);
        return true;
    }

    // the control holding the keys hid as the run moved on, so they go on to what the run asks next
    function pickUp() {
        var f = view.Window.activeFocusItem;
        if (!f)
            return;
        // keys that left Details on their own stay where they went
        if (view.parked && f !== view.parked)
            view.parked = null;
        // the hidden control can keep the focus until something else takes it
        var held = false, inside = false, depth = 0, it;
        for (it = f; it; it = it.parent)
            inside = inside || it === view;
        for (it = view; it && depth < 4; it = it.parent, depth++)
            held = held || f === it;
        if (!held && !(inside && !f.visible) && f !== view.parked)
            return;
        var next = view.run.phase === "prompt" ? options.itemAt(0)
            : view.run.phase === "auth" ? password
            : view.run.stalled && !view.run.keepWaiting ? keepButton
            : view.run.settled ? doneButton
            : view.run.live ? stopButton : details;
        // Details is a fold: the keys go to its own button, the next stop inside it, and wait
        // there only until the run has something to act on
        var parking = next === details;
        if (parking)
            next = details.nextItemInFocusChain(true);
        if (!next || !next.visible || next === f)
            return;
        view.parked = parking ? next : null;
        next.forceActiveFocus(view.run.handReason);
    }
    // where pickUp left the keys while the run was starting
    property Item parked: null
    Connections {
        target: view.Window.window
        function onActiveFocusItemChanged() {
            Qt.callLater(view.pickUp);
        }
    }
    Connections {
        target: view.run
        function onPhaseChanged() {
            Qt.callLater(view.pickUp);
        }
        function onPromptIdChanged() {
            Qt.callLater(view.pickUp);
        }
        function onStalledChanged() {
            Qt.callLater(view.pickUp);
        }
    }
    Component.onCompleted: Qt.callLater(view.pickUp)

    function stepTone(state, live) {
        if (live)
            return "becoming";
        switch (state) {
        case "ok": return "alive";
        case "failed": return "failed";
        }
        return "idle";
    }

    // state and clock
    Item {
        width: view.width
        height: Math.max(claim.implicitHeight, clock.implicitHeight)

        Row {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: LithicTokens.space.s3

            Claim {
                id: claim
                anchors.verticalCenter: parent.verticalCenter
                tone: view.tone
                text: view.claim
            }
            Working {
                anchors.verticalCenter: parent.verticalCenter
                active: visible
                visible: view.run.live && !view.run.stalled && view.run.phase === "running" || view.run.starting && view.run.phase === "idle"
            }
        }
        Label {
            id: clock
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            visible: view.run.started > 0
            text: view.run.settled && view.run.beat > 0 ? view.run.human(view.run.beat - view.run.started) : view.run.since(view.run.started)
            typeRole: "detail"
            numeric: true
        }
    }

    Label {
        width: view.width
        text: view.headline
        typeRole: "title"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Row {
        width: view.width
        spacing: LithicTokens.space.s3

        BladeProgress {
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - pct.width - parent.spacing
            value: view.run.fillFraction
            active: view.run.live
            indeterminate: view.run.starting && view.run.phase === "idle"
        }
        Label {
            id: pct
            anchors.verticalCenter: parent.verticalCenter
            width: 44
            horizontalAlignment: Text.AlignRight
            text: Math.round(view.run.fillFraction * 100) + "%"
            typeRole: "label"
            numeric: true
        }
    }

    Label {
        width: view.width
        visible: view.run.firstBuild && !view.run.settled
        text: I18n.tr("This is the first build on this computer, so every piece builds from scratch and it takes a few minutes. Later updates reuse that work and are much quicker.")
        typeRole: "detail"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    // the question the run waits on, with the choices it offers
    Column {
        width: view.width
        visible: view.run.phase === "prompt"
        spacing: LithicTokens.space.s3

        Label {
            width: parent.width
            visible: text.length > 0
            text: view.run.promptDetail
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Flow {
            width: parent.width
            spacing: LithicTokens.space.s2
            Repeater {
                id: options
                model: view.run.promptOptions
                delegate: Plaque {
                    id: option
                    required property var modelData
                    required property int index
                    kind: index === 0 ? "gold" : "line"
                    text: String(modelData)
                    onClicked: {
                        view.run.pressedBy(option);
                        view.run.answer(modelData);
                    }
                }
            }
        }
    }

    // the password the run needs, asked for here and handed to it on stdin
    Column {
        id: auth
        width: view.width
        visible: view.run.phase === "auth"
        spacing: LithicTokens.space.s3
        onVisibleChanged: if (visible) password.forceActiveFocus()

        // a refused password hands the field back with focus for the next try
        Connections {
            target: view.run
            function onAuthBusyChanged() {
                if (!view.run.authBusy && auth.visible)
                    Qt.callLater(() => password.forceActiveFocus());
            }
        }

        function submit() {
            if (password.text.length === 0 || view.run.authBusy)
                return;
            view.run.submitPassword(password.text);
            password.text = "";
        }

        Label {
            width: parent.width
            visible: text.length > 0
            text: view.run.promptDetail
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        TextField {
            id: password
            width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
            label: I18n.tr("Password")
            placeholderText: view.run.authBusy ? I18n.tr("Checking") : I18n.tr("Your password")
            secret: true
            enabled: !view.run.authBusy
            errorText: view.run.authBusy ? "" : view.run.promptError
            onAccepted: {
                view.run.handReason = Qt.TabFocusReason;
                auth.submit();
            }
        }
        Row {
            spacing: LithicTokens.space.s2
            Plaque {
                id: authContinue
                kind: "gold"
                text: I18n.tr("Continue")
                busy: view.run.authBusy
                enabled: password.text.length > 0 && !view.run.authBusy
                onClicked: {
                    view.run.pressedBy(authContinue);
                    auth.submit();
                }
            }
            Plaque {
                id: authCancel
                kind: "quiet"
                text: I18n.tr("Cancel")
                enabled: !view.run.authBusy
                onClicked: {
                    view.run.pressedBy(authCancel);
                    view.run.cancelPassword();
                }
            }
        }
    }

    // a step that stopped moving: say so, name what it waits on, offer the ways out
    Column {
        width: view.width
        visible: view.run.stalled && !view.run.keepWaiting
        spacing: LithicTokens.space.s2

        Label {
            width: parent.width
            text: view.run.unresponsive ? I18n.tr("The update stopped responding")
                : I18n.tr("No progress for %1").arg(view.run.human((view.run.watch.quiet || 0) * 1000))
            typeRole: "row"
            ink: Lithic.amber
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Label {
            width: parent.width
            text: view.run.unresponsive
                ? I18n.tr("Its process is still there but has not reported in for %1.").arg(view.run.since(view.run.beat))
                : I18n.tr("%1 has printed nothing and used no processor time. It may be waiting for something that will not come.").arg(view.run.watch.current || I18n.tr("The current step"))
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Label {
            width: parent.width
            visible: !!view.run.watch.pkg
            text: I18n.tr("A package install is part of this step. Stopping lets it finish on its own; only the rest of the update stops.")
            typeRole: "detail"
            inkRole: "fg3"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Row {
            topPadding: LithicTokens.space.s1
            spacing: LithicTokens.space.s2
            Plaque {
                id: keepButton
                kind: "line"
                small: true
                text: I18n.tr("Keep waiting")
                onClicked: {
                    view.run.pressedBy(keepButton);
                    view.run.keepWaiting = true;
                }
            }
            Plaque {
                id: stalledStop
                kind: "dangerLine"
                small: true
                text: I18n.tr("Stop update")
                onClicked: view.askStop(stalledStop)
            }
        }
    }

    // the failure in its own words, and the way back
    Column {
        width: view.width
        visible: view.run.phase === "error" && !view.run.stopped
        spacing: LithicTokens.space.s2

        Label {
            width: parent.width
            visible: view.run.errorMsg !== ""
            text: view.run.said(view.run.errorMsg, "")
            typeRole: "body"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Label {
            width: parent.width
            text: view.run.snapshotsSupported && view.run.snapshot !== ""
                ? I18n.tr("A snapshot of the whole system was saved before the update. Roll back shows how to return to it from the boot menu.")
                : I18n.tr("Details shows everything the update printed. Fix what it names, then update again.")
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Label {
        width: view.width
        visible: view.run.actionError !== ""
        text: view.run.actionError
        typeRole: "detail"
        ink: Lithic.jasper
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    // the health check found trouble in this run and Rashin can look into it
    Row {
        width: view.width
        visible: view.run.canFix
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - fixPlaque.width - parent.spacing
            text: view.run.fixMsg !== "" ? view.run.fixMsg : I18n.tr("The health check found problems after this update.")
            typeRole: "detail"
            ink: view.run.fixState === "failed" ? Lithic.jasper : Lithic.fg2
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: fixPlaque
            anchors.verticalCenter: parent.verticalCenter
            kind: "line"
            small: true
            glyph: "open_in_new"
            text: I18n.tr("Fix with Rashin")
            busy: view.run.fixState === "opening"
            onClicked: view.run.fix()
        }
    }

    Flow {
        width: view.width
        spacing: LithicTokens.space.s2
        visible: view.run.live || view.run.settled

        Plaque {
            id: doneButton
            visible: view.run.phase === "done" || view.run.phase === "error"
            kind: view.run.phase === "done" ? "gold" : "tonal"
            text: view.run.phase === "done" ? I18n.tr("Done") : I18n.tr("Dismiss")
            onClicked: {
                view.run.pressedBy(doneButton);
                view.run.dismiss();
            }
        }
        Plaque {
            visible: view.run.phase === "error" && view.run.snapshotsSupported && view.run.snapshot !== ""
            kind: "line"
            glyph: "history"
            text: I18n.tr("Roll back")
            onClicked: view.run.rollback()
        }
        Plaque {
            id: stopButton
            visible: view.run.live && !(view.run.stalled && !view.run.keepWaiting)
            kind: "dangerLine"
            text: I18n.tr("Stop")
            onClicked: view.askStop(stopButton)
        }
    }

    // the steps, each with how long it took
    Column {
        width: view.width
        visible: view.run.steps.length > 0
        topPadding: LithicTokens.space.s2

        Repeater {
            model: view.run.steps
            delegate: StepRow {
                width: view.width
                owner: view
            }
        }
    }

    // the run's own account of what it found and did
    Column {
        width: view.width
        spacing: LithicTokens.space.s1
        visible: view.run.logLines.length > 0

        Label {
            text: I18n.tr("Along the way")
            typeRole: "group"
            inkRole: "fg3"
        }
        Repeater {
            model: view.run.logLines.slice(-5)
            delegate: Label {
                required property var modelData
                width: view.width
                text: String(modelData)
                typeRole: "detail"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
                maximumLineCount: 2
            }
        }
    }

    Disclosure {
        id: details
        width: view.width
        text: I18n.tr("Details")
        visible: view.run.shown

        UpdateLog {
            width: details.width
            height: 280
            active: details.open && details.visible
            path: view.run.shownLogPath
            following: view.run.live
        }
    }

    component StepRow: Column {
        id: st
        required property var modelData
        required property int index
        // inline components do not see the outer ids
        required property var owner
        readonly property var view: st.owner

        property bool expanded: false
        readonly property string stepState: st.modelData.state || "pending"
        readonly property bool live: st.stepState === "running" && !view.run.settled
        readonly property bool hasPieces: st.modelData.key === view.run.piecesStep && view.run.pieces.length > 0
        readonly property bool piecesSettled: st.hasPieces && st.stepState !== "running" && st.stepState !== "pending"

        function stoppedPiece() {
            for (var i = view.run.pieces.length - 1; i >= 0; i--)
                if (view.run.pieces[i].state === "failed")
                    return I18n.tr(view.run.pieces[i].label || "");
            var last = view.run.pieces[view.run.pieces.length - 1];
            return last ? I18n.tr(last.label || "") : I18n.tr(st.modelData.label || "");
        }

        spacing: LithicTokens.space.s1
        topPadding: st.index === 0 ? 0 : LithicTokens.space.s2

        Item {
            width: st.width
            height: Math.max(stepLabel.implicitHeight, LithicTokens.size.factsRowHeight * 0.66)

            Flake {
                id: flake
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                tone: view.stepTone(st.stepState, st.live)
                fillFraction: st.stepState === "pending" || st.stepState === "skipped" ? 0 : 1
            }
            Label {
                id: stepLabel
                anchors.left: flake.right
                anchors.leftMargin: LithicTokens.space.s3
                anchors.right: stepTime.left
                anchors.rightMargin: LithicTokens.space.s3
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr(st.modelData.label || "")
                typeRole: "row"
                inkRole: st.live || st.stepState === "failed" || st.stepState === "ok" ? "fg" : "fg3"
                font.strikeout: st.stepState === "skipped"
            }
            Label {
                id: stepTime
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: st.live ? view.run.since(st.modelData.began || 0)
                    : st.stepState === "skipped" ? I18n.tr("Skipped")
                    : (st.modelData.took || 0) > 0 ? view.run.human(st.modelData.took)
                    : view.run.snapshotsSupported && st.stepState === "ok" && st.modelData.key === "snapshot" && view.run.snapshot !== "" ? I18n.tr("Snapshot %1").arg(view.run.snapshot)
                    : ""
                typeRole: "detail"
                inkRole: st.live ? "fg" : "fg2"
                numeric: true
            }
        }

        // the line the running step last printed
        Label {
            x: flake.width + LithicTokens.space.s3
            width: st.width - x
            visible: st.live && !st.hasPieces
            text: view.run.quiet
                ? I18n.tr("Quiet for %1: %2").arg(view.run.human((view.run.watch.quiet || 0) * 1000)).arg(view.run.watch.current || I18n.tr("waiting"))
                : view.run.activity !== "" ? view.run.activity : I18n.tr("Working")
            typeRole: "monoSm"
            inkRole: "fg3"
        }

        // a build step's pieces: listed while it runs, one result line after
        Row {
            x: flake.width + LithicTokens.space.s3
            width: st.width - x
            visible: st.piecesSettled
            spacing: LithicTokens.space.s3

            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width - piecesToggle.width - parent.spacing
                text: st.stepState === "failed"
                    ? I18n.tr("Stopped while building %1").arg(st.stoppedPiece())
                    : I18n.tr("%1 pieces built in %2").arg(view.run.pieces.filter(p => p.state === "ok").length).arg(view.run.human(st.modelData.took || 0))
                typeRole: "detail"
            }
            Plaque {
                id: piecesToggle
                anchors.verticalCenter: parent.verticalCenter
                kind: "quiet"
                small: true
                glyph: st.expanded ? "expand_less" : "expand_more"
                text: st.expanded ? I18n.tr("Hide pieces") : I18n.tr("Show pieces")
                onClicked: st.expanded = !st.expanded
            }
        }

        Column {
            id: pieceList
            x: flake.width + LithicTokens.space.s3
            width: st.width - x
            visible: st.hasPieces && (st.live || st.expanded)
            spacing: LithicTokens.space.s1

            Repeater {
                model: st.hasPieces && (st.live || st.expanded) ? view.run.pieces : []
                delegate: Column {
                    id: pc
                    required property var modelData
                    readonly property string pieceState: pc.modelData.state || "pending"
                    readonly property bool running: pc.pieceState === "running"
                    readonly property real typical: typeof pc.modelData.typical === "number" ? Math.max(0, pc.modelData.typical) : 0
                    readonly property real elapsed: pc.running && (pc.modelData.began || 0) > 0 ? Math.max(0, view.run.now - pc.modelData.began) : 0
                    readonly property real fraction: typeof pc.modelData.progress === "number" && pc.modelData.progress > 0
                        ? Math.min(1, pc.modelData.progress)
                        : pc.typical > 0 ? Math.min(0.95, pc.elapsed / pc.typical) : 0
                    width: pieceList.width
                    spacing: LithicTokens.space.s1

                    Item {
                        width: pc.width
                        height: pieceLabel.implicitHeight

                        Label {
                            id: pieceLabel
                            anchors.left: parent.left
                            anchors.right: pieceTime.left
                            anchors.rightMargin: LithicTokens.space.s3
                            text: I18n.tr(pc.modelData.label || "")
                            typeRole: "detail"
                            inkRole: pc.running ? "fg" : pc.pieceState === "pending" ? "fg3" : "fg2"
                            font.strikeout: pc.pieceState === "skipped"
                        }
                        Label {
                            id: pieceTime
                            anchors.right: parent.right
                            text: pc.running
                                ? (pc.typical > 0 ? I18n.tr("%1 of about %2").arg(view.run.since(pc.modelData.began || 0)).arg(view.run.human(pc.typical)) : view.run.since(pc.modelData.began || 0))
                                : pc.pieceState === "pending" ? (pc.typical > 0 ? I18n.tr("about %1").arg(view.run.human(pc.typical)) : "")
                                : (pc.modelData.took || 0) > 0 ? view.run.human(pc.modelData.took)
                                : pc.pieceState === "skipped" ? I18n.tr("Skipped") : ""
                            typeRole: "detail"
                            inkRole: pc.running ? "fg" : "fg3"
                            numeric: true
                        }
                    }
                    BladeProgress {
                        width: pc.width
                        visible: pc.running
                        value: pc.fraction
                        active: pc.running
                    }
                    Label {
                        width: pc.width
                        visible: pc.running && ((pc.modelData.detail || "") !== "" || (pc.modelData.note || "") !== "")
                        text: I18n.tr(pc.modelData.detail || pc.modelData.note || "")
                        typeRole: "monoSm"
                        inkRole: "fg3"
                    }
                }
            }
        }
    }
}
