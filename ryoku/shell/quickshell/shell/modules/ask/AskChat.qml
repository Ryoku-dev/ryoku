pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: root

    required property real s
    required property RashinSession session
    property bool active: false
    property real maximumHeight: 520 * s
    property bool modelMenuOpen: false
    readonly property var sessionData: session.state

    signal inputFocusRequested()

    implicitHeight: Math.min(maximumHeight, 430 * s)

    function shortModel(value) {
        const text = String(value || "");
        const slash = Math.max(text.lastIndexOf("/"), text.lastIndexOf(":"));
        return slash >= 0 ? text.slice(slash + 1) : text;
    }
    function statusLabel(value) {
        const status = String(value || "running").toLowerCase();
        if (status === "done" || status === "completed")
            return I18n.tr("Done");
        if (status === "failed" || status === "error")
            return I18n.tr("Failed");
        if (status === "cancelled" || status === "canceled")
            return I18n.tr("Cancelled");
        if (status === "waiting" || status === "pending")
            return I18n.tr("Waiting");
        return I18n.tr("Running");
    }


    function permissionOption(permission, kind) {
        const options = permission && permission.options ? permission.options : [];
        for (var i = 0; i < options.length; i++) {
            if (String(options[i].kind || "") === kind)
                return String(options[i].id || "");
        }
        return "";
    }

    function answer(permission, kind) {
        const option = permissionOption(permission, kind);
        if (option.length > 0)
            session.answerPermission(String(permission.request || ""), option);
    }

    Column {
        anchors.fill: parent
        spacing: Tokens.s2 * root.s

        Row {
            width: parent.width
            height: 32 * root.s
            spacing: Tokens.s2 * root.s

            Rectangle {
                width: parent.width - actionRow.width - parent.spacing
                height: parent.height
                radius: Tokens.radius * root.s
                color: modelTap.pressed ? Tokens.tint16 : modelHover.hovered ? Tokens.tint10 : Tokens.tint5
                border.width: Tokens.border
                border.color: root.modelMenuOpen ? Tokens.bone : Tokens.lineStrong
                Text {
                    anchors.centerIn: parent
                    width: parent.width - Tokens.s5 * root.s
                    text: root.sessionData.model ? root.shortModel(root.sessionData.model) : I18n.tr("Agent model")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall * root.s
                    elide: Text.ElideMiddle
                    horizontalAlignment: Text.AlignHCenter
                }
                HoverHandler { id: modelHover; cursorShape: Qt.PointingHandCursor }
                TapHandler { id: modelTap; onTapped: root.modelMenuOpen = !root.modelMenuOpen }
            }

            Row {
                id: actionRow
                height: parent.height
                spacing: Tokens.s1 * root.s
                Rectangle {
                    width: openLabel.implicitWidth + Tokens.s4 * root.s
                    height: parent.height
                    radius: Tokens.radius * root.s
                    color: openTap.pressed ? Tokens.tint16 : Tokens.tint5
                    border.width: Tokens.border
                    border.color: Tokens.lineStrong
                    opacity: root.session.id.length > 0 ? 1 : 0.45
                    Text { id: openLabel; anchors.centerIn: parent; text: I18n.tr("Open in Rashin"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall * root.s }
                    HoverHandler { enabled: root.session.id.length > 0; cursorShape: Qt.PointingHandCursor }
                    TapHandler { id: openTap; enabled: root.session.id.length > 0; onTapped: Quickshell.execDetached(["rashin", "open", "--session", root.session.id]) }
                }
                Rectangle {
                    width: newLabel.implicitWidth + Tokens.s4 * root.s
                    height: parent.height
                    radius: Tokens.radius * root.s
                    color: newTap.pressed ? Tokens.tint16 : Tokens.tint5
                    border.width: Tokens.border
                    border.color: Tokens.lineStrong
                    Text { id: newLabel; anchors.centerIn: parent; text: I18n.tr("New"); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall * root.s }
                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                    TapHandler { id: newTap; onTapped: { root.session.reset(); root.inputFocusRequested(); } }
                }
            }
        }

        Rectangle {
            width: parent.width
            visible: root.modelMenuOpen
            height: visible ? Math.min(modelColumn.implicitHeight + Tokens.s2 * root.s, 160 * root.s) : 0
            radius: Tokens.radius * root.s
            color: Tokens.paperLift
            border.width: Tokens.border
            border.color: Tokens.lineStrong
            clip: true
            Flickable {
                anchors.fill: parent
                anchors.margins: Tokens.s1 * root.s
                contentHeight: modelColumn.implicitHeight
                boundsBehavior: Flickable.StopAtBounds
                Column {
                    id: modelColumn
                    width: parent.width
                    Repeater {
                        model: root.sessionData.models || []
                        delegate: Rectangle {
                            id: modelRow
                            required property var modelData
                            width: modelColumn.width
                            height: 32 * root.s
                            radius: Tokens.radius * root.s
                            color: String(modelData.id || "") === String(root.sessionData.model || "") ? Tokens.bone : modelRowHover.hovered ? Tokens.tint10 : "transparent"
                            Text {
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.leftMargin: Tokens.s3 * root.s
                                anchors.rightMargin: Tokens.s3 * root.s
                                anchors.verticalCenter: parent.verticalCenter
                                text: String(modelRow.modelData.name || modelRow.modelData.id || "")
                                color: String(modelRow.modelData.id || "") === String(root.sessionData.model || "") ? Tokens.inkOnBone : Tokens.ink
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall * root.s
                                elide: Text.ElideMiddle
                            }
                            HoverHandler { id: modelRowHover; cursorShape: Qt.PointingHandCursor }
                            TapHandler { onTapped: { root.session.setModel(String(modelRow.modelData.id || "")); root.modelMenuOpen = false; } }
                        }
                    }
                    Text {
                        width: parent.width
                        visible: (root.sessionData.models || []).length === 0
                        text: I18n.tr("Model choices appear when the harness reports them")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny * root.s
                        wrapMode: Text.WordWrap
                    }
                }
            }
        }

        Flickable {
            id: transcript
            width: parent.width
            height: Math.max(120 * root.s, parent.height - y)
            contentHeight: transcriptColumn.implicitHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded; visible: transcript.contentHeight > transcript.height + 1 }
            onContentHeightChanged: contentY = Math.max(0, contentHeight - height)

            Column {
                id: transcriptColumn
                width: transcript.width
                spacing: Tokens.s3 * root.s

                Repeater {
                    model: root.sessionData.items || []
                    delegate: Item {
                        id: row
                        required property var modelData
                        width: transcriptColumn.width
                        implicitHeight: rowColumn.implicitHeight
                        Column {
                            id: rowColumn
                            width: parent.width
                            spacing: Tokens.s1 * root.s
                            Text {
                                text: row.modelData.kind === "user" ? I18n.tr("You")
                                    : row.modelData.kind === "tool" ? root.statusLabel(row.modelData.status)
                                    : row.modelData.kind === "thought" ? I18n.tr("Thought") : I18n.tr("Rashin")
                                color: Tokens.inkMuted
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fTiny * root.s
                            }
                            TextEdit {
                                width: parent.width
                                visible: row.modelData.kind !== "tool" && text.length > 0
                                text: row.modelData.kind === "user" && row.modelData.handoff
                                    ? I18n.tr("From Ask: %1").arg(row.modelData.handoff.question || row.modelData.text || "")
                                    : String(row.modelData.text || "")
                                textFormat: row.modelData.kind === "text" ? TextEdit.MarkdownText : TextEdit.PlainText
                                color: row.modelData.kind === "thought" ? Tokens.inkMuted : Tokens.inkDim
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall * root.s
                                font.italic: row.modelData.kind === "thought"
                                wrapMode: TextEdit.Wrap
                                readOnly: true
                                selectByMouse: true
                                onLinkActivated: url => Spawn.runApp(["xdg-open", url], { label: "browser" })
                            }
                            Rectangle {
                                visible: row.modelData.kind === "tool"
                                width: parent.width
                                implicitHeight: toolText.implicitHeight + Tokens.s3 * root.s * 2
                                radius: Tokens.radius * root.s
                                color: Tokens.tint5
                                border.width: Tokens.border
                                border.color: Tokens.lineSoft
                                Text {
                                    id: toolText
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.top: parent.top
                                    anchors.margins: Tokens.s3 * root.s
                                    text: String(row.modelData.title || row.modelData.toolKind || I18n.tr("Tool"))
                                    color: Tokens.inkDim
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fSmall * root.s
                                    elide: Text.ElideRight
                                }
                            }
                        }
                    }
                }

                Rectangle {
                    width: parent.width
                    visible: (root.sessionData.plan || []).length > 0
                    implicitHeight: planColumn.implicitHeight + Tokens.s3 * root.s * 2
                    radius: Tokens.radius * root.s
                    color: Tokens.tint5
                    border.width: Tokens.border
                    border.color: Tokens.lineSoft
                    Column {
                        id: planColumn
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.top: parent.top
                        anchors.margins: Tokens.s3 * root.s
                        spacing: Tokens.s1 * root.s
                        Text { text: I18n.tr("Plan"); color: Tokens.inkMuted; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny * root.s }
                        Repeater {
                            model: root.sessionData.plan || []
                            delegate: Text {
                                required property var modelData
                                width: planColumn.width
                                text: root.statusLabel(modelData.status) + "  " + String(modelData.content || "")
                                color: Tokens.inkDim
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall * root.s
                                wrapMode: Text.WordWrap
                            }
                        }
                    }
                }

                Repeater {
                    model: root.sessionData.permissions || []
                    delegate: Rectangle {
                        id: permission
                        required property var modelData
                        width: transcriptColumn.width
                        implicitHeight: permissionColumn.implicitHeight + Tokens.s3 * root.s * 2
                        radius: Tokens.radius * root.s
                        color: Tokens.tint10
                        border.width: Tokens.border
                        border.color: Tokens.lineStrong
                        Column {
                            id: permissionColumn
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.top: parent.top
                            anchors.margins: Tokens.s3 * root.s
                            spacing: Tokens.s2 * root.s
                            Text { width: parent.width; text: String(permission.modelData.title || permission.modelData.input || I18n.tr("Permission required")); color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall * root.s; wrapMode: Text.WordWrap }
                            Flow {
                                width: parent.width
                                spacing: Tokens.s2 * root.s
                                Repeater {
                                    model: [
                                        { kind: "allow_once", label: I18n.tr("Allow once") },
                                        { kind: "allow_always", label: I18n.tr("Always allow") },
                                        { kind: "reject_once", label: I18n.tr("Deny") }
                                    ]
                                    delegate: Rectangle {
                                        id: permissionButton
                                        required property var modelData
                                        readonly property string option: root.permissionOption(permission.modelData, modelData.kind)
                                        width: permissionLabel.implicitWidth + Tokens.s4 * root.s
                                        height: Tokens.ctlH * root.s
                                        radius: Tokens.radius * root.s
                                        color: permissionTap.pressed ? Tokens.tint16 : Tokens.tint5
                                        opacity: option.length > 0 ? 1 : 0.4
                                        border.width: Tokens.border
                                        border.color: Tokens.lineStrong
                                        Text { id: permissionLabel; anchors.centerIn: parent; text: permissionButton.modelData.label; color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny * root.s }
                                        HoverHandler { enabled: permissionButton.option.length > 0; cursorShape: Qt.PointingHandCursor }
                                        TapHandler { id: permissionTap; enabled: permissionButton.option.length > 0; onTapped: root.answer(permission.modelData, permissionButton.modelData.kind) }
                                    }
                                }
                            }
                        }
                    }
                }

                Text {
                    width: parent.width
                    visible: String(root.sessionData.error || "").length > 0
                    text: String(root.sessionData.error || "")
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall * root.s
                    wrapMode: Text.WordWrap
                }
            }
        }
    }
}
