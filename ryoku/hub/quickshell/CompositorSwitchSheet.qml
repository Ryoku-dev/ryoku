pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import Quickshell.Io

// The destructive confirmation for a compositor switch. It shows, verbatim from
// `ryoku-hub wm preview <target>`, what carries over and what the target cannot
// honour, then asks the one extra question a switch raises: keep the compositor
// you are leaving so you can return with no download, or remove it to reclaim
// the space. Confirming runs `ryoku wm use <target>` in a terminal, using the
// same reversible package transaction as the CLI. Nothing here is spelled per
// compositor: every name, package and path comes from the preview.
Item {
    id: sh

    property bool active: false
    property var target: null       // the provider row we switch TO
    property var current: null      // the active provider row we would leave
    property var report: null       // parsed `wm preview <target>`
    property bool loading: false
    property string keep: "keep"    // "keep" | "remove"; keep is the reversible default

    signal closed()

    function open(targetRow, currentRow) {
        if (!targetRow || targetRow.active === true || targetRow.available !== true)
            return;
        sh.target = targetRow;
        sh.current = currentRow;
        sh.report = null;
        sh.keep = "keep";
        sh.switching = false;
        sh.switched = false;
        sh.failure = "";
        sh.loading = true;
        previewProc.command = ["ryoku-hub", "wm", "preview", targetRow.name];
        previewProc.running = false;
        previewProc.running = true;
        sh.active = true;
        sh.forceActiveFocus();
    }
    function close() { sh.active = false; sh.closed(); }
    function cap(name) { return name && name.length ? name.charAt(0).toUpperCase() + name.slice(1) : (name || ""); }
    // The reclaimed size from the package manager's removal summary.
    function humanSize(bytes) {
        var b = Number(bytes) || 0;
        if (b >= 1073741824) return (b / 1073741824).toFixed(2) + " GiB";
        return (b / 1048576).toFixed(2) + " MiB";
    }

    readonly property string targetName: sh.report ? sh.report.target : (sh.target ? sh.target.name : "")
    readonly property string activeName: sh.report ? sh.report.active : (sh.current ? sh.current.name : "")
    readonly property bool leaving: sh.activeName !== "" && sh.activeName !== sh.targetName
    readonly property bool deployed: !!sh.report && sh.report.deployed === true
    readonly property bool available: !!sh.report && sh.report.available === true
    readonly property var unhonored: sh.report && sh.report.unhonored ? sh.report.unhonored : []
    // What leaving the active compositor reclaims, computed by the backend for
    // the outgoing compositor's own packages. Removable drives the keep-or-remove
    // question: it is true whenever any of those packages are installed, so a
    // checkout box that never installed the meta-package is still offered the
    // choice its compositor packages make real.
    readonly property var reclaim: sh.report && sh.report.reclaim ? sh.report.reclaim : null
    readonly property bool reclaimRemovable: !!sh.reclaim && sh.reclaim.removable === true
    property bool switching: false
    property bool switched: false
    property string failure: ""

    anchors.fill: parent
    visible: sh.active
    z: 250
    focus: sh.active
    onActiveChanged: if (sh.active) sh.forceActiveFocus()
    Keys.onEscapePressed: (event) => { sh.close(); event.accepted = true; }

    Process {
        id: previewProc
        stdout: StdioCollector {
            onStreamFinished: {
                try { sh.report = JSON.parse(this.text); }
                catch (e) { sh.report = null; }
                sh.loading = false;
            }
        }
        stderr: StdioCollector { }
    }

    // A switch with nothing to install runs here rather than in a terminal: a
    // console window for work that needs no password and prints one line was
    // the wrong surface for it. Installing the desktop packages still gets the
    // terminal, where progress and the privilege prompt belong.
    Process {
        id: switchProc
        stdout: StdioCollector { }
        stderr: StdioCollector { }
        onExited: (code) => {
            sh.switching = false;
            sh.switched = code === 0;
            sh.failure = code === 0 ? "" : I18n.tr("The switch did not complete. Run ryoku wm use %1 to see why.").arg(sh.targetName);
        }
    }

    function shq(s) { return "'" + String(s).replace(/'/g, "'\\''") + "'"; }
    function confirm() {
        if (!sh.available || sh.loading || sh.switching || !sh.target
                || sh.target.available !== true)
            return;
        // The CLI owns the transaction order, so the Hub never spells a package
        // manager command of its own. Removal drops the outgoing compositor's
        // packages only; its config tree holds hand-written files the user owns,
        // and the switch leaves them in place.
        var removing = sh.keep === "remove" && sh.reclaimRemovable;
        // The in-process path is only safe when nothing privileged happens: a
        // deployed target installs nothing, and keeping the old compositor
        // removes nothing. Anything that installs the desktop packages gets a
        // terminal for its progress and privilege prompt.
        if (sh.deployed && !removing) {
            sh.switching = true;
            switchProc.command = ["ryoku", "wm", "use", sh.target.name, "--keep-previous"];
            switchProc.running = false;
            switchProc.running = true;
            return;
        }
        var line = "ryoku wm use " + sh.shq(sh.target.name)
            + (removing ? " --remove-previous" : " --keep-previous");
        line += "; echo; read -n1 -rsp " + sh.shq(I18n.tr("Done. Press any key to close.")) + "; echo";
        Spawn.run(["kitty", "--class", "ryoku-wm-switch", "-e", "sh", "-c", line]);
        sh.close();
    }
    function logOut() { Spawn.run(["ryoku", "wm", "act", "session.exit"]); }

    // dim backdrop: a click outside the card cancels.
    MouseArea { anchors.fill: parent; onClicked: sh.close() }

    Rectangle {
        id: card
        anchors.centerIn: parent
        width: Math.min(parent.width - Tokens.s6 * 2, 660)
        height: Math.min(parent.height - Tokens.s6 * 2, 640)
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        MouseArea { anchors.fill: parent; onClicked: {} }

        // ── head ────────────────────────────────────────────────────────────
        Text {
            id: eyebrow
            anchors { left: parent.left; top: parent.top; leftMargin: Tokens.s5; topMargin: Tokens.s4 }
            text: I18n.tr("SWITCH COMPOSITOR")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
        }
        IconBtn {
            id: closeBtn
            anchors { right: parent.right; top: parent.top; rightMargin: Tokens.s4; topMargin: Tokens.s4 }
            glyph: "\u00d7"
            onAct: sh.close()
        }
        Text {
            id: headline
            anchors { left: eyebrow.left; top: eyebrow.bottom; topMargin: Tokens.s1; right: closeBtn.left; rightMargin: Tokens.s3 }
            text: sh.switched ? I18n.tr("%1 is ready").arg(sh.cap(sh.targetName))
                : I18n.tr("Switch to %1").arg(sh.cap(sh.targetName))
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fHero
            elide: Text.ElideRight
        }

        // ── body ────────────────────────────────────────────────────────────
        Flickable {
            id: body
            anchors {
                left: parent.left; right: parent.right
                top: headline.bottom; bottom: decision.top
                leftMargin: Tokens.s5; rightMargin: Tokens.s4
                topMargin: Tokens.s4; bottomMargin: Tokens.s3
            }
            clip: true
            visible: !sh.switched
            contentHeight: bodyCol.height
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: bodyCol
                width: body.width - Tokens.s3
                spacing: Tokens.s5

                Text {
                    visible: sh.loading
                    text: I18n.tr("Reading the switch report\u2026")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                }

                // carries over -------------------------------------------------
                Column {
                    visible: !sh.loading
                    width: parent.width
                    spacing: Tokens.s2
                    CompositorSwitchSheetHead { width: parent.width; text: I18n.tr("CARRIES OVER") }
                    Body {
                        width: parent.width
                        text: I18n.tr("Every desktop.* setting carries over; %1 honours what it can.").arg(sh.cap(sh.targetName))
                    }
                    Body {
                        width: parent.width
                        visible: sh.report && sh.report.keybindCount > 0
                        text: I18n.tr("%1 keybinds carry over unchanged (compositor-neutral).").arg(sh.report ? sh.report.keybindCount : 0)
                    }
                    Body {
                        width: parent.width
                        visible: sh.leaving
                        text: I18n.tr("Your wm.%1.* settings stay in the store and return if you switch back.").arg(sh.activeName)
                    }
                }

                // what the target cannot do -----------------------------------
                Column {
                    visible: !sh.loading
                    width: parent.width
                    spacing: Tokens.s2
                    CompositorSwitchSheetHead {
                        width: parent.width
                        text: I18n.tr("UNAVAILABLE ON %1").arg(sh.cap(sh.targetName).toUpperCase())
                    }
                    Body {
                        width: parent.width
                        visible: !!sh.report && sh.report.exact !== true
                        text: I18n.tr("Install %1 to see the exact list.").arg(sh.report ? sh.report.package : "")
                    }
                    Body {
                        width: parent.width
                        visible: !!sh.report && sh.report.exact === true && sh.unhonored.length === 0
                        text: I18n.tr("None. %1 honours every current setting.").arg(sh.cap(sh.targetName))
                    }
                    Repeater {
                        model: sh.report && sh.report.exact === true ? sh.unhonored : []
                        Row {
                            id: urow
                            required property var modelData
                            width: bodyCol.width
                            spacing: Tokens.s2
                            Text {
                                text: "\u2013"
                                color: Tokens.inkFaint
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fSmall
                            }
                            Column {
                                width: parent.width - Tokens.s4
                                spacing: 1
                                Text {
                                    text: urow.modelData.key
                                    color: Tokens.ink
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fSmall
                                }
                                Text {
                                    width: parent.width
                                    wrapMode: Text.WordWrap
                                    text: urow.modelData.reason
                                    color: Tokens.inkMuted
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fSmall
                                    lineHeight: 1.25
                                }
                            }
                        }
                    }
                }

            }
        }

        // ── done ────────────────────────────────────────────────────────────
        // Replaces the preview once the switch has landed. A compositor change
        // only takes effect at the next session, so the one useful next step is
        // logging out, and that is the action offered.
        Column {
            visible: sh.switched
            anchors {
                left: parent.left; right: parent.right
                top: headline.bottom; bottom: decision.top
                leftMargin: Tokens.s5; rightMargin: Tokens.s5
                topMargin: Tokens.s5
            }
            spacing: Tokens.s3
            CompositorSwitchSheetHead { text: I18n.tr("WHAT HAPPENS NEXT") }
            Body {
                width: parent.width
                text: I18n.tr("Log out, then pick %1 at the greeter. Your session is untouched until you do.").arg(sh.cap(sh.targetName))
            }
            Body {
                width: parent.width
                text: I18n.tr("Every desktop.* setting is already in place, and %1 stays exactly as it is, so you can come back the same way.").arg(sh.cap(sh.activeName))
                faint: true
            }
        }

        // ── decision ────────────────────────────────────────────────────────
        // pinned above the footer so the reason a switch is blocked and the
        // keep-or-remove choice are always in view, never scrolled off with the
        // report.
        Column {
            id: decision
            anchors {
                left: parent.left; right: parent.right; bottom: foot.top
                leftMargin: Tokens.s5; rightMargin: Tokens.s5; bottomMargin: Tokens.s3
            }
            visible: !sh.loading
            spacing: Tokens.s3

            Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }

            Rectangle {
                visible: sh.failure !== "" || (!!sh.report && !sh.available)
                width: parent.width
                height: unavailText.height + Tokens.s3
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: Tokens.line
                Text {
                    id: unavailText
                    anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; leftMargin: Tokens.s3; rightMargin: Tokens.s3 }
                    wrapMode: Text.WordWrap
                    text: sh.failure !== "" ? sh.failure
                        : I18n.tr("The desktop packages are not available on this system, so this switch cannot be started here.")
                    color: Tokens.alert
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    lineHeight: 1.3
                }
            }

            // The keep-or-remove question, shown whenever leaving the active
            // compositor would reclaim something. The count and size are the
            // outgoing compositor's own installed packages, so the choice is
            // offered on a checkout box too, where the meta-package is absent but
            // the compositor packages are not.
            Column {
                visible: sh.leaving && sh.reclaimRemovable
                width: parent.width
                spacing: Tokens.s2
                CompositorSwitchSheetHead {
                    width: parent.width
                    text: I18n.tr("LEAVING %1").arg(sh.cap(sh.activeName).toUpperCase())
                }
                Seg {
                    options: ["Keep", "Remove"]
                    current: sh.keep === "remove" ? "Remove" : "Keep"
                    onChose: (key) => sh.keep = (key === "Remove") ? "remove" : "keep"
                }
                Body {
                    width: parent.width
                    // The count and size are read null-safe: the block is hidden
                    // while the report loads, but its bindings still evaluate.
                    text: sh.keep === "remove"
                        ? I18n.tr("Remove %1: frees %2 packages (%3) and drops its session entry; switching back later reinstalls them.").arg(sh.cap(sh.activeName)).arg(sh.reclaim ? sh.reclaim.count : 0).arg(sh.humanSize(sh.reclaim ? sh.reclaim.size : 0))
                        : I18n.tr("Keep %1 installed: switch back with no download, at the cost of %2 packages (%3) staying on disk.").arg(sh.cap(sh.activeName)).arg(sh.reclaim ? sh.reclaim.count : 0).arg(sh.humanSize(sh.reclaim ? sh.reclaim.size : 0))
                }
                Body {
                    width: parent.width
                    text: I18n.tr("Either way your wm.%1.* settings and its config files stay put, so a switch back restores them.").arg(sh.activeName)
                    faint: true
                }
            }

            // Nothing of the outgoing compositor is installed to reclaim: the
            // block still appears and says so, because an option that silently
            // disappears reads as a missing feature.
            Column {
                visible: sh.leaving && !sh.reclaimRemovable
                width: parent.width
                spacing: Tokens.s2
                CompositorSwitchSheetHead {
                    width: parent.width
                    text: I18n.tr("LEAVING %1").arg(sh.cap(sh.activeName).toUpperCase())
                }
                Body {
                    width: parent.width
                    text: I18n.tr("Nothing to remove: none of %1's packages are installed to reclaim here, so both compositors stay available and switching back costs nothing.").arg(sh.cap(sh.activeName))
                }
            }
        }

        // ── foot ────────────────────────────────────────────────────────────
        Item {
            id: foot
            anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
            height: 60
            Rectangle { height: 1; color: Tokens.lineSoft; anchors { left: parent.left; right: parent.right; top: parent.top } }
            Btn {
                anchors { left: parent.left; leftMargin: Tokens.s5; verticalCenter: parent.verticalCenter }
                text: sh.switched ? I18n.tr("LATER") : I18n.tr("CANCEL")
                onAct: sh.close()
            }
            Btn {
                anchors { right: parent.right; rightMargin: Tokens.s5; verticalCenter: parent.verticalCenter }
                text: sh.switched ? I18n.tr("LOG OUT NOW")
                    : (sh.switching ? I18n.tr("SWITCHING")
                    : I18n.tr("SWITCH TO %1").arg(sh.cap(sh.targetName).toUpperCase()))
                primary: true
                armed: sh.switched || (sh.available && !sh.loading && !sh.switching)
                onAct: sh.switched ? sh.logOut() : sh.confirm()
            }
        }
    }

    // a section rule inside the sheet, in the same // vocabulary as the settings
    // sections, so the confirmation reads as part of the same surface.
    component CompositorSwitchSheetHead: Row {
        property alias text: mark.text
        spacing: Tokens.s2
        Text {
            text: "//"
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            anchors.verticalCenter: parent.verticalCenter
        }
        Text {
            id: mark
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackMark
            anchors.verticalCenter: parent.verticalCenter
        }
    }

    component Body: Text {
        property bool faint: false
        wrapMode: Text.WordWrap
        color: faint ? Tokens.inkFaint : Tokens.inkMuted
        font.family: Tokens.ui
        font.pixelSize: Tokens.fSmall
        lineHeight: 1.3
    }
}
