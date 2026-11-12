import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// GPU passthrough: where it stands, the plan it would carry out, setting it up in a
// terminal (it builds a kernel module), turning it off, and the readiness checks
Column {
    id: root

    required property var hub
    required property var store
    readonly property var caps: root.store ? root.store.caps : ({})
    readonly property var availability: root.store ? root.store.passthroughAvailability : null
    readonly property string card: root.caps.passthrough ? root.caps.passthrough.model : I18n.tr("the discrete graphics card")
    readonly property bool enabled: root.caps.enabled === true
    // availability is whether setting it up can install what it needs; once on, it no longer applies
    readonly property bool unavailable: !root.enabled && !!root.availability && root.availability.available !== true
    // the screen can be rewired from this page (Display wired to)
    readonly property bool canRewire: !!root.store && root.store.mux.capable === true

    property string plan: ""
    property bool planning: false
    property bool setupStarted: false
    property string error: ""
    property bool turningOff: false
    property bool slow: false
    property bool verySlow: false
    property bool cancelled: false
    property bool reviewing: false
    // the running turn-off, so Cancel can stop it
    property var offJob: null
    // the turn-off started from the keys, so focus follows it
    property bool offKeys: false
    // the review started from the keys, so focus moves on to the plan or the error
    property bool reviewKeys: false

    spacing: LithicTokens.space.s3

    readonly property string status: {
        if (root.store && root.store.capsError !== "")
            return I18n.tr("Could not read your graphics hardware.");
        if (root.unavailable)
            return root.availability.reason !== "" ? I18n.tr(root.availability.reason) : I18n.tr("GPU passthrough is not available on this system.");
        switch (root.caps.verdict) {
        case "ready": return I18n.tr("Ready. %1 is free for a virtual machine to claim, and comes back to the desktop when the machine stops.").arg(root.card);
        case "needs-relogin": return I18n.tr("Set up. Log out and back in once, then it is ready.");
        case "needs-reboot": return root.canRewire
            ? I18n.tr("Your screen runs on %1. Wire it to the integrated graphics with Display wired to, above, and restart, so %1 is free.").arg(root.card)
            : I18n.tr("Your screen runs on %1. Set your firmware's graphics mode to hybrid and restart, so %1 is free.").arg(root.card);
        case "needs-setup": return I18n.tr("Not set up yet. Review the changes, then set it up.");
        case "incapable": return I18n.tr("This computer cannot give a graphics card to a virtual machine. The checks below say why.");
        }
        return I18n.tr("Checking…");
    }
    readonly property string statusTone: root.caps.verdict === "ready" ? "alive"
        : (root.caps.verdict === "incapable" || root.unavailable) ? "idle" : "attention"

    function review() {
        root.error = "";
        root.reviewKeys = reviewButton.visualFocus || retry.visualFocus;
        root.runReview();
    }

    function setUp() {
        var keys = setupButton.visualFocus;
        Spawn.runApp(["kitty", "--class", "ryoku-gpu", "-e", "sh", "-c",
            "ryoku-hub gpu apply enable; echo; printf '%s ' \"$1\"; read -r _", "sh", I18n.tr("Done. Press Enter to close.")], { label: "kitty" });
        root.planning = false;
        root.plan = "";
        root.setupStarted = true;
        if (keys)
            root.keyFocus([checkButton]);
    }

    // a button that hid under keyboard focus hands it to the first of these still shown
    function keyFocus(items) {
        Qt.callLater(function () {
            var to = items.find(i => i.visible);
            if (to)
                to.forceActiveFocus(Qt.TabFocusReason);
        });
    }

    function recheck() {
        root.setupStarted = false;
        root.error = "";
        if (root.store)
            root.store.refresh();
    }

    // each readiness check in plain words; anything unknown shows as reported
    function plain(c) {
        var v = String(c.value || ""), ok = c.level === "ok";
        switch (c.id) {
        case "cpu-virt": return { label: I18n.tr("Processor virtualisation"), value: ok ? I18n.tr("On") : I18n.tr("Off"),
            hint: I18n.tr("Turn on virtualisation (SVM on AMD, VT-x on Intel) in your firmware settings, then restart.") };
        case "kvm": return { label: I18n.tr("Virtual machine support"), value: ok ? I18n.tr("Ready") : I18n.tr("Missing"),
            hint: I18n.tr("Turn on virtualisation in your firmware settings, then restart.") };
        case "iommu": return { label: I18n.tr("Device isolation"), value: ok ? I18n.tr("On") : I18n.tr("Off"),
            hint: I18n.tr("Turn on IOMMU (VT-d on Intel, AMD-Vi on AMD) in your firmware settings, then restart.") };
        case "two-gpus": return { label: I18n.tr("Two graphics cards"), value: ok ? I18n.tr("Yes") : I18n.tr("Only one"),
            hint: I18n.tr("Passthrough needs a second graphics card to keep the desktop running.") };
        case "iommu-isolation": return { label: I18n.tr("Card isolation"), value: ok ? I18n.tr("On its own") : I18n.tr("Shared with other devices"),
            hint: I18n.tr("Other devices share the card's isolation group, so a virtual machine would take them too.") };
        case "display-owner":
            if (ok)
                return { label: I18n.tr("Screen"), value: I18n.tr("Runs on %1").arg(v.replace(/^host runs on /, "")), hint: "" };
            if (/panel/.test(v))
                return { label: I18n.tr("Screen"), value: I18n.tr("Runs on %1").arg(root.card),
                    hint: root.canRewire ? I18n.tr("Wire the screen to the integrated graphics with Display wired to, above, then restart.")
                        : I18n.tr("Set your firmware's graphics mode to hybrid, then restart, so the integrated graphics run the screen.") };
            if (/monitor/.test(v))
                return { label: I18n.tr("Screen"), value: I18n.tr("A monitor runs on %1").arg(root.card),
                    hint: I18n.tr("Your desktop moves to the other card the next time you sign in.") };
            return { label: I18n.tr("Screen"), value: I18n.tr("Only screen runs on %1").arg(root.card),
                hint: I18n.tr("Connect a monitor to the other card, or the desktop would have no screen.") };
        case "ram": {
            var mb = parseInt(v) || 0;
            return { label: I18n.tr("Memory"), value: I18n.tr("%1 GB free").arg((mb / 1024).toFixed(1)),
                hint: I18n.tr("A virtual machine wants 8 GB or more; close some apps or give it less memory.") };
        }
        case "tooling":
            if (ok)
                return { label: I18n.tr("Virtual machine software"), value: I18n.tr("Installed"), hint: "" };
            if (/QEMU/.test(v))
                return { label: I18n.tr("Virtual machine software"), value: I18n.tr("Not installed"),
                    hint: I18n.tr("Install QEMU from the Store, or with ryoku-host pkg install qemu-desktop in a terminal.") };
            return { label: I18n.tr("Passthrough software"), value: I18n.tr("Not installed yet"),
                hint: I18n.tr("Setting up passthrough installs it.") };
        case "session": return { label: I18n.tr("Virtual machine access"), value: ok ? I18n.tr("Ready") : I18n.tr("Sign in again"),
            hint: I18n.tr("You were added to the virtual machine group. Log out and back in once.") };
        }
        return { label: I18n.tr(c.label || ""), value: v || "-", hint: c.hint ? I18n.tr(c.hint) : "" };
    }

    function turnOff() {
        // the button hides while it runs, so focus that came from the keys moves to Cancel, then back
        root.offKeys = offButton.visualFocus || retry.visualFocus;
        root.hub.confirm({
            title: I18n.tr("Turn off GPU passthrough?"),
            text: I18n.tr("%1 goes back to the desktop and virtual machines can no longer claim it. You can set it up again later.").arg(root.card),
            actionText: I18n.tr("Turn off")
        }, function () {
            root.error = "";
            root.cancelled = false;
            root.turningOff = true;
            slowTimer.restart();
            verySlowTimer.restart();
            root.offJob = root.store.run(["ryoku-hub", "gpu", "apply", "disable"], function (ok, out, err, code) {
                root.offJob = null;
                root.turningOff = false;
                root.slow = false;
                root.verySlow = false;
                slowTimer.stop();
                verySlowTimer.stop();
                if (!ok)
                    root.error = root.cancelled ? I18n.tr("Stopped. Nothing changed, so passthrough is still on.")
                        : root.store.failure(err, I18n.tr("Passthrough"), code);
                if (root.offKeys)
                    root.keyFocus([retry, offButton]);
                // read the state again, keeping any error on show
                root.store.refresh();
            });
        });
    }

    function cancelTurnOff() {
        root.cancelled = true;
        if (root.offJob)
            root.offJob.signal(15);
    }

    function runReview() {
        root.reviewing = true;
        root.store.run(["ryoku-hub", "gpu", "apply", "enable", "--dry-run"], function (ok, out, err, code) {
            root.reviewing = false;
            if (ok) {
                // the plan, minus the tool's own line markers
                root.plan = out.trim().split("\n").map(l => l.replace(/^\[plan\]\s*/, "")).join("\n");
                root.planning = true;
                if (root.reviewKeys)
                    root.keyFocus([setupButton]);
                return;
            }
            var e = root.store.cleaned(err);
            root.error = code === -1 ? I18n.tr("Could not work out the changes.") + " " + String(err)
                : e !== "" ? I18n.tr("Could not work out the changes: %1. Retry once that is fixed.").arg(e)
                : I18n.tr("Could not work out the changes. Retry, and if it keeps failing, run ryoku doctor.");
            if (root.reviewKeys)
                root.keyFocus([retry]);
        }, 30000);
    }

    Timer { id: slowTimer; interval: 1000; onTriggered: root.slow = true }
    Timer {
        id: verySlowTimer
        interval: 10000
        onTriggered: {
            root.verySlow = true;
            if (root.offKeys)
                root.keyFocus([cancelButton]);
        }
    }

    Label {
        width: parent.width
        text: I18n.tr("Free %1 from the desktop so a virtual machine can own it at nearly full speed. This prepares the computer only; you run the virtual machine yourself (libvirt and Looking Glass). Everyday virtual machines need none of this.").arg(root.card)
        typeRole: "detail"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Row {
        width: parent.width
        spacing: LithicTokens.space.s2
        Claim {
            id: claim
            anchors.verticalCenter: parent.verticalCenter
            tone: root.statusTone
            text: root.caps.verdict === "ready" ? I18n.tr("Ready")
                : root.enabled ? I18n.tr("Needs you") : I18n.tr("Off")
        }
        Label {
            width: parent.width - claim.width - parent.spacing
            anchors.verticalCenter: parent.verticalCenter
            text: root.status
            typeRole: "row"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }

    Flow {
        width: parent.width
        spacing: LithicTokens.space.s2
        visible: !root.planning

        Plaque {
            id: reviewButton
            visible: !root.enabled && !root.unavailable && root.caps.verdict !== "incapable" && !root.setupStarted
            enabled: root.availability !== null && !root.reviewing
            busy: root.reviewing
            kind: "line"
            small: true
            glyph: "fact_check"
            text: I18n.tr("Review the changes")
            onClicked: root.review()
        }
        Plaque {
            id: offButton
            visible: root.enabled && !root.turningOff
            kind: "dangerLine"
            small: true
            glyph: "power_settings_new"
            text: I18n.tr("Turn off passthrough")
            onClicked: root.turnOff()
        }
        Plaque {
            id: checkButton
            visible: !root.turningOff
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Check again")
            onClicked: root.recheck()
        }
    }

    // turning it off asks for your password and rebuilds the boot image, so it can take a while
    Column {
        width: parent.width
        visible: root.turningOff && root.slow
        spacing: LithicTokens.space.s2
        Row {
            spacing: LithicTokens.space.s2
            Working {
                anchors.verticalCenter: parent.verticalCenter
                active: root.turningOff
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                // once the password is in, the change runs to the end; cancelling then only waits for it
                text: root.cancelled ? I18n.tr("Stopping…") : I18n.tr("Turning off passthrough…")
                typeRole: "detail"
            }
        }
        BladeProgress {
            width: parent.width
            visible: root.verySlow
            active: root.verySlow
            indeterminate: true
        }
        Plaque {
            id: cancelButton
            visible: root.verySlow && !root.cancelled
            kind: "quiet"
            small: true
            text: I18n.tr("Cancel")
            onClicked: root.cancelTurnOff()
        }
    }

    Row {
        width: parent.width
        visible: root.error !== ""
        spacing: LithicTokens.space.s3
        Label {
            width: Math.min(implicitWidth, parent.width - retry.width - parent.spacing)
            anchors.verticalCenter: parent.verticalCenter
            text: root.error
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            anchors.verticalCenter: parent.verticalCenter
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: root.enabled ? root.turnOff() : root.review()
        }
    }

    // the plan, word for word, before anything changes
    Column {
        width: parent.width
        visible: root.planning
        spacing: LithicTokens.space.s2

        Label {
            width: parent.width
            text: I18n.tr("Setting it up makes these changes. It runs in a terminal because it builds a kernel module, which can take a few minutes.")
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        CutRect {
            width: parent.width
            height: Math.min(planText.implicitHeight + 2 * LithicTokens.space.s3, 240)
            fill: Lithic.field
            cut: LithicTokens.cut.small
            clip: true
            Flickable {
                id: planFlick
                anchors.fill: parent
                anchors.margins: LithicTokens.space.s3
                contentWidth: width
                contentHeight: planText.implicitHeight
                boundsBehavior: Flickable.StopAtBounds
                clip: true
                Label {
                    id: planText
                    width: planFlick.width
                    text: root.plan
                    typeRole: "monoSm"
                    inkRole: "fg2"
                    wrapMode: Text.WrapAnywhere
                    elide: Text.ElideNone
                }
            }
            ScrollRail {
                flickable: planFlick
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                anchors.right: parent.right
            }
        }
        Row {
            spacing: LithicTokens.space.s2
            Plaque {
                id: setupButton
                kind: "gold"
                small: true
                glyph: "terminal"
                text: I18n.tr("Set up in a terminal")
                onClicked: root.setUp()
            }
            Plaque {
                kind: "quiet"
                small: true
                text: I18n.tr("Close")
                onClicked: {
                    var keys = visualFocus;
                    root.planning = false;
                    root.plan = "";
                    if (keys)
                        root.keyFocus([reviewButton, checkButton]);
                }
            }
        }
    }

    Label {
        width: parent.width
        visible: root.setupStarted
        text: I18n.tr("Setting up in a terminal window. Choose Check again when it finishes.")
        typeRole: "detail"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    // the readiness checks, each with what to do when it falls short
    Label {
        width: parent.width
        topPadding: LithicTokens.space.s2
        text: I18n.tr("Readiness checks")
        typeRole: "row"
    }
    Repeater {
        model: root.caps.checks || []
        delegate: Column {
            id: check
            required property var modelData
            readonly property bool short: ["warn", "bad", "fail"].indexOf(check.modelData.level) >= 0
            readonly property var words: root.plain(check.modelData)
            width: root.width
            spacing: LithicTokens.space.s1
            Row {
                width: parent.width
                spacing: LithicTokens.space.s3
                Label {
                    width: LithicTokens.size.factsLabelWidth
                    text: check.words.label
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                    typeRole: "body"
                    inkRole: "fg3"
                }
                Label {
                    width: parent.width - LithicTokens.size.factsLabelWidth - parent.spacing
                    text: check.words.value
                    typeRole: "body"
                    ink: check.short ? Lithic.amber : Lithic.fg
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }
            Label {
                width: parent.width
                visible: check.short && check.words.hint !== ""
                leftPadding: LithicTokens.size.factsLabelWidth + LithicTokens.space.s3
                text: check.words.hint
                typeRole: "detail"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }
    }
}
