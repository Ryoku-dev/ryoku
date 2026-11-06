pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../../Singletons"

Popup {
    id: sheet

    property var status: null
    property string profileName: ""
    property string mode: "archive"
    property var plan: null
    property var progress: []
    property string failure: ""
    property bool busy: false
    property bool done: false

    signal completed()

    parent: Overlay.overlay
    anchors.centerIn: parent
    modal: true
    width: Math.min(700, parent ? parent.width - Tokens.s6 * 2 : 700)
    height: Math.min(720, parent ? parent.height - Tokens.s6 * 2 : 720)
    padding: 0
    closePolicy: sheet.busy ? Popup.NoAutoClose : Popup.CloseOnEscape | Popup.CloseOnPressOutside

    background: Rectangle {
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        radius: Tokens.radius
    }
    Overlay.modal: Rectangle { color: Qt.rgba(0, 0, 0, 0.6) }

    function list(value) { return value instanceof Array ? value : []; }

    function humanSize(value) {
        var bytes = Number(value || 0);
        if (bytes < 1024)
            return Math.round(bytes) + " B";
        var units = ["KiB", "MiB", "GiB", "TiB"];
        var index = -1;
        do { bytes /= 1024; index++; } while (bytes >= 1024 && index < units.length - 1);
        return bytes.toFixed(bytes >= 10 ? 1 : 2).replace(/\.0+$/, "") + " " + units[index];
    }

    function openFor(profileStatus, name) {
        sheet.status = profileStatus;
        sheet.profileName = name;
        sheet.mode = "archive";
        sheet.plan = null;
        sheet.progress = [];
        sheet.failure = "";
        sheet.busy = false;
        sheet.done = false;
        open();
        requestPlan();
    }

    function optionArgs() {
        if (sheet.mode === "keep")
            return ["--keep-account"];
        if (sheet.mode === "erase")
            return ["--erase-files"];
        return [];
    }

    function requestPlan() {
        if (!sheet.status || sheet.busy)
            return;
        sheet.failure = "";
        sheet.plan = null;
        sheet.done = false;
        sheet.busy = true;
        planProc.command = ["ryoku-host", "profile", "plan-remove", "--account", String(sheet.status.account || "")].concat(sheet.optionArgs());
        planProc.running = true;
    }

    function removeProfile() {
        if (!sheet.plan || sheet.busy)
            return;
        sheet.failure = "";
        sheet.progress = [];
        sheet.busy = true;
        removeProc.command = ["pkexec", "ryoku-host", "profile", "remove", "--account", String(sheet.status.account || "")]
            .concat(sheet.optionArgs()).concat(["--digest", String(sheet.plan.digest || "")]);
        removeProc.running = true;
    }

    function parseLine(raw) {
        var line = String(raw || "").trim();
        if (line === "")
            return;
        try {
            var value = JSON.parse(line);
            if (value.error)
                sheet.failure = I18n.tr(String(value.error));
            else if (value.ok === true)
                sheet.done = true;
            else if (value.stage)
                sheet.progress = sheet.progress.concat([value]);
        } catch (e) {
            sheet.failure = I18n.tr(line);
        }
    }

    function parseError(stdoutText, stderrText, fallback) {
        var text = String(stdoutText || "").trim();
        if (text !== "") {
            try {
                var value = JSON.parse(text);
                if (value && value.error)
                    return I18n.tr(String(value.error));
            } catch (e) {
            }
        }
        text = String(stderrText || "").trim();
        return text !== "" ? I18n.tr(text) : fallback;
    }

    Process {
        id: planProc
        environment: Spawn.env
        stdout: StdioCollector { id: planOut }
        stderr: StdioCollector { id: planErr }
        onExited: function(code) {
            sheet.busy = false;
            if (code !== 0) {
                sheet.failure = sheet.parseError(planOut.text, planErr.text, I18n.tr("The removal plan could not be prepared."));
                return;
            }
            try {
                var value = JSON.parse(String(planOut.text || "{}"));
                if (value.error)
                    throw new Error(String(value.error));
                sheet.plan = value;
            } catch (e) {
                sheet.failure = I18n.tr(String(e.message || e));
            }
        }
    }

    Process {
        id: removeProc
        environment: Spawn.env
        stdout: SplitParser { splitMarker: "\n"; onRead: line => sheet.parseLine(line) }
        stderr: StdioCollector { id: removeErr }
        onExited: function(code) {
            sheet.busy = false;
            if (code === 0 && sheet.done) {
                sheet.completed();
                return;
            }
            if (sheet.failure === "")
                sheet.failure = sheet.parseError("", removeErr.text, I18n.tr("The profile account could not be removed."));
        }
    }

    component SectionHead: Text {
        color: Tokens.inkDim
        font.family: Tokens.ui
        font.pixelSize: Tokens.fMicro
        font.weight: Font.Medium
        font.letterSpacing: Tokens.trackLabel
    }

    component RemoveRow: Item {
        property string label: ""
        property string detail: ""
        property string badge: ""
        width: parent ? parent.width : 0
        implicitHeight: rowCopy.implicitHeight + Tokens.s3 * 2
        Column {
            id: rowCopy
            anchors { left: parent.left; right: rowBadge.visible ? rowBadge.left : parent.right; rightMargin: rowBadge.visible ? Tokens.s3 : 0; verticalCenter: parent.verticalCenter }
            spacing: Tokens.s1
            Text { width: parent.width; text: parent.parent.label; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium; wrapMode: Text.WordWrap }
            Text { visible: text !== ""; width: parent.width; text: parent.parent.detail; color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; wrapMode: Text.WordWrap }
        }
        Rectangle {
            id: rowBadge
            visible: parent.badge !== ""
            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
            width: badgeText.implicitWidth + Tokens.s3
            height: 22
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            Text { id: badgeText; anchors.centerIn: parent; text: rowBadge.parent.badge; color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny }
        }
        Rectangle {
            anchors {
                left: parent.left
                right: parent.right
                bottom: parent.bottom
            }
            height: 1
            color: Tokens.lineSoft
        }
    }

    Item {
        id: header
        anchors { left: parent.left; right: parent.right; top: parent.top }
        height: 88
        Column {
            anchors { left: parent.left; right: closeButton.left; leftMargin: Tokens.s5; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
            spacing: Tokens.s1
            Text { text: I18n.tr("REMOVE PROFILE"); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark }
            Text { width: parent.width; text: I18n.tr("Remove %1 from %2?").arg(sheet.profileName).arg(sheet.status ? String(sheet.status.account || "") : ""); color: Tokens.ink; font.family: Tokens.display; font.pixelSize: Tokens.fValue; elide: Text.ElideRight }
        }
        Btn {
            id: closeButton
            anchors {
                right: parent.right
                rightMargin: Tokens.s4
                verticalCenter: parent.verticalCenter
            }
            compact: true
            text: I18n.tr("CLOSE")
            armed: !sheet.busy
            onAct: sheet.close()
        }
        Rectangle {
            anchors {
                left: parent.left
                right: parent.right
                bottom: parent.bottom
            }
            height: 1
            color: Tokens.lineSoft
        }
    }

    Flickable {
        id: body
        anchors { left: parent.left; right: parent.right; top: header.bottom; bottom: footer.top; margins: Tokens.s5; rightMargin: Tokens.s4 }
        contentWidth: width
        contentHeight: content.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: content
            width: body.width - Tokens.s3
            spacing: Tokens.s5

            Column {
                width: parent.width
                spacing: Tokens.s2
                SectionHead { text: I18n.tr("ACCOUNT AND FILES") }
                Seg {
                    width: parent.width
                    options: ["archive", "keep", "erase"]
                    labels: ({ "archive": I18n.tr("Archive home"), "keep": I18n.tr("Keep account"), "erase": I18n.tr("Erase home") })
                    current: sheet.mode
                    enabled: !sheet.busy && !sheet.done
                    onChose: value => {
                        sheet.mode = value;
                        sheet.requestPlan();
                    }
                }
                RemoveRow {
                    visible: !!sheet.plan
                    readonly property var accountPlan: sheet.plan ? (sheet.plan.account || ({})) : ({})
                    label: I18n.tr("Account action: %1").arg(I18n.tr(String(accountPlan.action || "")))
                    detail: accountPlan.archive ? I18n.tr("Archive: %1").arg(String(accountPlan.archive)) : String(accountPlan.home || "")
                    badge: I18n.tr(String(accountPlan.action || "").toUpperCase())
                }
                RemoveRow {
                    visible: !!sheet.plan
                    readonly property var filePlan: sheet.plan ? (sheet.plan.files || ({})) : ({})
                    label: I18n.tr("%1 profile-managed files").arg(Number(filePlan.managed || 0))
                    detail: I18n.tr("Only files recorded by this profile are part of the removal transaction.")
                    badge: I18n.tr("FILES")
                }
                Text {
                    visible: !!sheet.plan && sheet.list(sheet.plan.userData).length > 0
                    width: parent.width
                    text: sheet.mode === "erase"
                        ? I18n.tr("User data erased: %1").arg(sheet.list(sheet.plan.userData).join(", "))
                        : (sheet.mode === "archive"
                            ? I18n.tr("User data archived with the home: %1").arg(sheet.list(sheet.plan.userData).join(", "))
                            : I18n.tr("User data kept in the account: %1").arg(sheet.list(sheet.plan.userData).join(", ")))
                    color: sheet.mode === "erase" ? Tokens.alert : Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall
                    wrapMode: Text.WordWrap
                }
            }

            Column {
                visible: !!sheet.plan && sheet.list(sheet.plan.packages).length > 0
                width: parent.width
                spacing: Tokens.s2
                SectionHead { text: I18n.tr("PACKAGES REMOVED") }
                Repeater {
                    model: sheet.plan ? sheet.list(sheet.plan.packages) : []
                    delegate: RemoveRow {
                        required property var modelData
                        label: String(modelData.name || "")
                        detail: sheet.humanSize(modelData.installed || 0)
                        badge: String(modelData.repo || "")
                    }
                }
            }

            Column {
                visible: !!sheet.plan && sheet.list(sheet.plan.keptPackages).length > 0
                width: parent.width
                spacing: Tokens.s2
                SectionHead { text: I18n.tr("PACKAGES KEPT") }
                Repeater {
                    model: sheet.plan ? sheet.list(sheet.plan.keptPackages) : []
                    delegate: RemoveRow {
                        required property var modelData
                        label: String(modelData.name || "")
                        detail: I18n.tr(String(modelData.reason || ""))
                        badge: I18n.tr("KEEP")
                    }
                }
            }

            Column {
                visible: !!sheet.plan && sheet.list(sheet.plan.repos).length > 0
                width: parent.width
                spacing: Tokens.s2
                SectionHead { text: I18n.tr("REPOSITORIES") }
                Repeater {
                    model: sheet.plan ? sheet.list(sheet.plan.repos) : []
                    delegate: RemoveRow {
                        required property var modelData
                        label: String(modelData.id || "")
                        detail: I18n.tr(String(modelData.reason || ""))
                        badge: I18n.tr(String(modelData.action || "").toUpperCase())
                    }
                }
            }

            Column {
                visible: !!sheet.plan && sheet.list(sheet.plan.system).length > 0
                width: parent.width
                spacing: Tokens.s2
                SectionHead { text: I18n.tr("SYSTEM PIECES") }
                Repeater {
                    model: sheet.plan ? sheet.list(sheet.plan.system) : []
                    delegate: RemoveRow {
                        required property var modelData
                        label: String(modelData.path || modelData.kind || "")
                        detail: I18n.tr(String(modelData.kind || ""))
                        badge: I18n.tr(String(modelData.action || "").toUpperCase())
                    }
                }
            }

            Column {
                visible: sheet.progress.length > 0
                width: parent.width
                spacing: Tokens.s2
                SectionHead { text: I18n.tr("PROGRESS") }
                Repeater {
                    model: sheet.progress
                    delegate: RemoveRow {
                        required property var modelData
                        label: I18n.tr(String(modelData.message || modelData.stage || ""))
                        detail: String(modelData.done || 0) + " / " + String(modelData.total || 0)
                        badge: I18n.tr(String(modelData.stage || "").toUpperCase())
                    }
                }
            }

            Text {
                visible: sheet.busy && !sheet.plan
                text: I18n.tr("Preparing the removal plan…")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
            Text {
                visible: sheet.done
                width: parent.width
                text: I18n.tr("The profile account was removed according to the reviewed plan.")
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.WordWrap
            }
            Text {
                visible: sheet.failure !== ""
                width: parent.width
                text: sheet.failure
                color: Tokens.alert
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                font.weight: Font.Medium
                wrapMode: Text.WordWrap
            }
        }
    }

    Item {
        id: footer
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: 62
        Rectangle {
            anchors {
                left: parent.left
                right: parent.right
                top: parent.top
            }
            height: 1
            color: Tokens.lineSoft
        }
        Btn {
            anchors {
                left: parent.left
                leftMargin: Tokens.s5
                verticalCenter: parent.verticalCenter
            }
            text: sheet.done ? I18n.tr("DONE") : I18n.tr("CANCEL")
            armed: !sheet.busy
            onAct: sheet.close()
        }
        Btn {
            visible: !sheet.done
            anchors { right: parent.right; rightMargin: Tokens.s5; verticalCenter: parent.verticalCenter }
            text: sheet.busy ? I18n.tr("WORKING") : I18n.tr("REMOVE PROFILE")
            primary: true
            armed: !sheet.busy && !!sheet.plan
            onAct: sheet.removeProfile()
        }
    }
}
