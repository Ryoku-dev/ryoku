pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

// The harbour log: every alert with its reason beside it, and every event the
// session recorded, newest first. The dashboard counts alerts; this is where
// you read them. An auth alert carries its own fix: save the password into the
// keyring (never a file) and the next probe fills the graphs.
Item {
    id: log
    property bool open: false
    signal closed()
    signal openResource(string kind, string key)

    anchors.fill: parent
    visible: opacity > 0.01
    opacity: open ? 1 : 0
    z: 110
    Behavior on opacity { NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease } }

    // the merged session log: yard receipts and harbour events on one rail.
    readonly property var session: {
        var rows = [], i;
        for (i = 0; i < Remotes.events.length; i++) {
            var e = Remotes.events[i];
            rows.push({ at: e.at || 0, time: e.time, kind: e.kind, title: e.alias || "", text: e.text });
        }
        for (i = 0; i < Vm.events.length; i++) {
            var v = Vm.events[i];
            rows.push({ at: v.at || 0, time: v.time, kind: v.kind, title: v.vm || "", text: v.text });
        }
        rows.sort(function (a, b) { return b.at - a.at; });
        return rows;
    }

    // scrim: only an outside click (or Esc) dismisses the sheet
    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0, 0, 0, 0.55)
        MouseArea { anchors.fill: parent; onClicked: log.closed() }
    }

    Keys.onEscapePressed: (e) => { if (log.open) { log.closed(); e.accepted = true; } }
    focus: open

    component SectionMark: Row {
        id: mark
        property string label: ""
        property string glyph: ""
        spacing: Tokens.s2
        Text { text: "//"; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro; anchors.verticalCenter: parent.verticalCenter }
        Text {
            text: mark.label
            color: Tokens.ink
            font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
            anchors.verticalCenter: parent.verticalCenter
        }
        Text { text: mark.glyph; color: Tokens.inkFaint; font.family: Tokens.jp; font.pixelSize: 12; anchors.verticalCenter: parent.verticalCenter }
    }

    Rectangle {
        anchors.centerIn: parent
        width: Math.min(640, log.width - Tokens.s6 * 2)
        height: Math.min(log.height - Tokens.s5 * 2, 680)
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        MouseArea { anchors.fill: parent }   // a click inside never dismisses
        Ticks { color: Tokens.line }

        Row {
            id: head
            anchors { top: parent.top; left: parent.left; right: parent.right }
            anchors.margins: Tokens.s6
            width: parent.width - Tokens.s6 * 2
            spacing: Tokens.s2
            SectionMark { label: I18n.tr("HARBOUR LOG"); glyph: "港"; anchors.verticalCenter: parent.verticalCenter }
            Item { width: parent.width - 180; height: 1 }
            IconBtn { anchors.verticalCenter: parent.verticalCenter; glyph: "\u2715"; onAct: log.closed() }
        }
        Rectangle {
            id: headRule
            anchors { top: head.bottom; topMargin: Tokens.s3; left: parent.left; right: parent.right }
            anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6
            height: 1; color: Tokens.lineSoft
        }

        // the whole body scrolls: a long alert list can never spill past the
        // card edge or bury the session log.
        Flickable {
            id: bodyFlick
            anchors { top: headRule.bottom; topMargin: Tokens.s4; left: parent.left; right: parent.right; bottom: parent.bottom }
            anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6; anchors.bottomMargin: Tokens.s6
            contentWidth: width
            contentHeight: col.implicitHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }

            Column {
                id: col
                width: bodyFlick.width
                spacing: Tokens.s4

            // ---- alerts -----------------------------------------------------
            SectionMark { label: I18n.tr("ALERTS"); glyph: "警" }
            Text {
                width: parent.width
                visible: Remotes.alertCount === 0 && (!Vm.fault || Vm.fault.length === 0)
                text: I18n.tr("All quiet. Every berth answers and nothing is flagged.")
                color: Tokens.inkMuted
                font.family: Tokens.ui; font.pixelSize: 11
            }
            Repeater {
                model: {
                    var rows = Remotes.alerts.slice();
                    if (Vm.fault && Vm.fault.length > 0)
                        rows.unshift({ kind: "fault", key: "", title: I18n.tr("The yard"),
                            text: I18n.tr("engine fault"), detail: Vm.faultDetail || Vm.fault });
                    return rows;
                }
                delegate: Rectangle {
                    id: alertRow
                    required property var modelData
                    width: parent ? parent.width : 0
                    implicitHeight: inner.implicitHeight + Tokens.s4 * 2
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.lineSoft

                    Column {
                        id: inner
                        anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter }
                        anchors.margins: Tokens.s4
                        spacing: 3
                        Row {
                            width: parent.width
                            spacing: Tokens.s2
                            Text {
                                width: inner.width - kindTag.width - Tokens.s2
                                text: (alertRow.modelData.title || "-") + I18n.tr(": ") + alertRow.modelData.text
                                elide: Text.ElideRight
                                color: Tokens.ink
                                font.family: Tokens.ui; font.pixelSize: 11
                                font.weight: Font.Medium
                            }
                            Text {
                                id: kindTag
                                text: (alertRow.modelData.kind || "").toUpperCase()
                                color: Tokens.inkFaint
                                font.family: Tokens.mono; font.pixelSize: 9
                            }
                        }
                        Text {
                            width: parent.width
                            text: alertRow.modelData.detail || ""
                            visible: text.length > 0
                            wrapMode: Text.WordWrap
                            color: Tokens.inkMuted
                            font.family: Tokens.mono; font.pixelSize: 10
                        }
                        // an auth alert offers its own repair, inline.
                        Row {
                            width: parent.width
                            spacing: Tokens.s2
                            visible: alertRow.modelData.kind === "auth"
                            Field {
                                id: pwF
                                width: parent.width - saveBtn.width - retryBtn.width - Tokens.s2 * 2
                                secret: true
                                tabular: true
                                placeholder: I18n.tr("password for %1 (keyring only)").arg(alertRow.modelData.key)
                            }
                            Btn {
                                id: saveBtn
                                text: I18n.tr("SAVE")
                                primary: true
                                armed: pwF.text.length > 0
                                onAct: {
                                    Remotes.setPass(alertRow.modelData.key, pwF.text);
                                    pwF.clear();
                                }
                            }
                            Btn {
                                id: retryBtn
                                text: I18n.tr("PROBE")
                                onAct: Remotes.probe(alertRow.modelData.key)
                            }
                        }
                        Row {
                            spacing: Tokens.s2
                            visible: alertRow.modelData.key.length > 0
                            Btn {
                                text: I18n.tr("REVIEW")
                                compact: true
                                onAct: {
                                    var k = alertRow.modelData.kind;
                                    log.openResource(k === "console" ? "console" : "remote", alertRow.modelData.key);
                                }
                            }
                            Btn {
                                visible: alertRow.modelData.kind === "console"
                                text: I18n.tr("OPEN CONSOLE")
                                compact: true
                                onAct: { Remotes.openConsole(alertRow.modelData.key); log.closed(); }
                            }
                        }
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: Tokens.lineSoft }

            // ---- session log -------------------------------------------------
            SectionMark { label: I18n.tr("SESSION"); glyph: "記録" }
            Flickable {
                width: parent.width
                height: Math.min(240, Math.max(120, log.height * 0.3))
                contentHeight: sessionList.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                Column {
                    id: sessionList
                    width: parent.width
                    spacing: 0
                    Text {
                        width: parent.width
                        visible: log.session.length === 0
                        text: I18n.tr("Nothing has happened yet this session.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui; font.pixelSize: 11
                    }
                    Repeater {
                        model: log.session
                        Item {
                            id: eventRow
                            required property var modelData
                            width: sessionList.width
                            height: 30
                            Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: Tokens.lineSoft; opacity: 0.5 }
                            Text { anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter; width: 58; text: eventRow.modelData.time; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9 }
                            Text { anchors.left: parent.left; anchors.leftMargin: 66; anchors.verticalCenter: parent.verticalCenter; width: 88; elide: Text.ElideRight; text: (eventRow.modelData.kind || "event").toUpperCase(); color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: 9 }
                            Text { anchors.left: parent.left; anchors.leftMargin: 162; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; elide: Text.ElideRight; text: (eventRow.modelData.title.length > 0 ? eventRow.modelData.title + " \u2014 " : "") + eventRow.modelData.text; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 10 }
                        }
                    }
                }
            }
        }
    }
}
}
