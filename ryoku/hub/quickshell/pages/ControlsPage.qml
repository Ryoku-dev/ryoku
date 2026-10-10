pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."
import "../Singletons"
import "../schema/ControlsPage.js" as ControlsSchema

Item {
    id: page
    property var hub
    property string selectedSection: "vitals"

    property var config: ControlsSchema.defaults()
    property var confirmedConfig: ControlsSchema.defaults()
    property var retryConfig: ControlsSchema.defaults()
    property bool writePending: false
    property string writeError: ""
    readonly property var selectedModel: ControlsSchema.section(selectedSection)
    readonly property string footerText: writePending
        ? "SAVING CONTROLS…"
        : writeError !== "" ? "CONTROLS NOT SAVED · RETRY IN PAGE"
        : "SAVED · LIVE ON YOUR DESKTOP"

    function sameConfig(a, b) {
        return JSON.stringify(ControlsSchema.normalize(a)) === JSON.stringify(ControlsSchema.normalize(b));
    }

    function syncConfirmed() {
        var server = ControlsSchema.normalize(Settings.get("controls"));
        page.confirmedConfig = server;
        page.config = server;
        if (page.sameConfig(server, page.retryConfig))
            page.writeError = "";
    }

    Component.onCompleted: page.syncConfirmed()
    Connections {
        target: Settings
        function onRevisionChanged() {
            if (!page.writePending)
                page.syncConfirmed();
        }
    }

    function finishWrite(ok, error, candidate) {
        if (!page.writePending)
            return;
        page.writePending = false;
        if (ok) {
            page.confirmedConfig = ControlsSchema.normalize(candidate);
            page.config = page.confirmedConfig;
            page.writeError = "";
        } else {
            page.config = page.confirmedConfig;
            page.writeError = error || I18n.tr("The shell daemon rejected the change.");
        }
    }

    function patch(next) {
        if (page.writePending)
            return;
        var candidate = ControlsSchema.normalize(next);
        if (page.sameConfig(candidate, page.confirmedConfig)) {
            page.config = page.confirmedConfig;
            page.writeError = "";
            return;
        }
        page.retryConfig = candidate;
        page.writeError = "";
        page.writePending = true;
        Settings.patch("controls", candidate, function(ok, error) {
            page.finishWrite(ok, error, candidate);
        });
    }

    function copyConfig() {
        return ControlsSchema.normalize(page.config);
    }

    function setSectionVisible(id, visible) {
        var next = page.copyConfig();
        for (var i = 0; i < next.sections.length; ++i)
            if (next.sections[i].id === id)
                next.sections[i] = { id: id, visible: visible };
        page.patch(next);
    }

    function moveSection(id, target) {
        var next = page.copyConfig();
        var from = -1;
        for (var i = 0; i < next.sections.length; ++i)
            if (next.sections[i].id === id) { from = i; break; }
        if (from < 0)
            return;
        target = Math.max(0, Math.min(next.sections.length - 1, target));
        if (from === target)
            return;
        var moved = next.sections.splice(from, 1)[0];
        next.sections.splice(target, 0, moved);
        page.patch(next);
    }

    function elementVisible(id) {
        return page.config.hidden.indexOf(id) < 0;
    }

    function setElementVisible(id, visible) {
        var next = page.copyConfig();
        var at = next.hidden.indexOf(id);
        if (visible && at >= 0)
            next.hidden.splice(at, 1);
        else if (!visible && at < 0)
            next.hidden.push(id);
        page.patch(next);
    }

    function resetLayout() {
        page.selectedSection = "vitals";
        page.patch(ControlsSchema.defaults());
    }

    component VisibilityChip: Rectangle {
        id: chip
        required property string label
        required property bool checked
        signal changed(bool checked)
        implicitWidth: chipLabel.implicitWidth + Tokens.s3 * 2
        implicitHeight: Tokens.ctlH + Tokens.s1
        radius: Tokens.radius
        color: checked ? Tokens.bone : (chipHover.hovered ? Tokens.tint10 : Tokens.paperLift)
        border.width: Tokens.border
        border.color: checked ? Tokens.bone : Tokens.line

        Text {
            id: chipLabel
            anchors.centerIn: parent
            text: I18n.tr(chip.label)
            color: chip.checked ? Tokens.inkOnBone : Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            font.weight: Font.Medium
        }
        HoverHandler { id: chipHover; cursorShape: Qt.PointingHandCursor }
        TapHandler { onTapped: chip.changed(!chip.checked) }
    }

    Column {
        id: head
        anchors.top: parent.top
        width: page.width - 14
        spacing: Tokens.s3

        Item {
            width: parent.width
            height: 14
            Row {
                id: eyebrow
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s2
                Rectangle { width: 16; height: Tokens.border; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
                Text { text: "力"; color: Tokens.ink; font.family: Tokens.jp; font.pixelSize: Tokens.fMicro; anchors.verticalCenter: parent.verticalCenter }
                Text {
                    text: I18n.tr("DESKTOP")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackMark
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
            Rectangle {
                anchors.left: eyebrow.right
                anchors.right: register.left
                anchors.verticalCenter: parent.verticalCenter
                anchors.leftMargin: Tokens.s3
                anchors.rightMargin: Tokens.s3
                height: Tokens.border
                color: Tokens.lineSoft
            }
            Text {
                id: register
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("CONTROL / LAYOUT")
                color: Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fTiny
                font.letterSpacing: Tokens.trackLabel
            }
        }
        Text {
            text: I18n.tr("Controls")
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fTitle
        }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Choose what Super+Escape shows. Drag the miniature blocks into order, then refine the selected block.")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            wrapMode: Text.WordWrap
        }
    }
    Rectangle {
        id: writeStatus
        visible: page.writePending || page.writeError !== ""
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: head.bottom
        anchors.topMargin: Tokens.s3
        anchors.rightMargin: 14
        height: visible ? 44 : 0
        radius: Tokens.radius
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: page.writeError !== "" ? Tokens.lineStrong : Tokens.line

        Rectangle {
            id: statusMark
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            width: 7
            height: 7
            radius: 4
            color: page.writeError !== "" ? Tokens.inkMuted : Tokens.bone
            SequentialAnimation on opacity {
                running: page.writePending && !Tokens.reduceMotion
                loops: Animation.Infinite
                NumberAnimation { to: 0.3; duration: 600 }
                NumberAnimation { to: 1; duration: 600 }
            }
        }

        Btn {
            id: retryButton
            visible: page.writeError !== ""
            anchors.right: parent.right
            anchors.rightMargin: Tokens.s2
            anchors.verticalCenter: parent.verticalCenter
            width: 82
            text: I18n.tr("RETRY")
            onAct: page.patch(page.retryConfig)
        }

        Text {
            anchors.left: statusMark.right
            anchors.leftMargin: Tokens.s3
            anchors.right: retryButton.visible ? retryButton.left : parent.right
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            text: page.writePending
                ? I18n.tr("Saving Controls. Waiting for the desktop service.")
                : I18n.tr("Controls were not saved.") + " " + page.writeError
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            elide: Text.ElideRight
        }
    }


    Flickable {
        id: flick
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: writeStatus.visible ? writeStatus.bottom : head.bottom
        anchors.bottom: parent.bottom
        anchors.topMargin: writeStatus.visible ? Tokens.s3 : Tokens.s5
        contentWidth: width
        contentHeight: Math.max(editor.height, height)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        QQC.ScrollBar.vertical: ScrollRail { policy: QQC.ScrollBar.AsNeeded }
        WheelScroll { }

        Row {
            id: editor
            width: flick.width - 14
            spacing: Tokens.s5
            enabled: !page.writePending
            opacity: page.writePending ? 0.62 : 1
            Behavior on opacity { NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic } }


            SettingCard {
                id: previewCardHost
                width: Math.max(360, Math.round((editor.width - editor.spacing) * 0.54))
                title: I18n.tr("PANEL PREVIEW")
                kana: I18n.tr("表示")
                collapsible: false

                Item {
                    width: parent.width
                    height: previewColumn.height + Tokens.s4 * 2

                    Column {
                        id: previewColumn
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.top: parent.top
                        anchors.margins: Tokens.s4
                        spacing: Tokens.s2

                        Repeater {
                            id: previewRepeater
                            model: page.config.sections

                            delegate: Item {
                                id: slot
                                required property var modelData
                                required property int index
                                width: previewColumn.width
                                height: 60

                                readonly property var definition: ControlsSchema.section(modelData.id)
                                readonly property bool shown: modelData.visible === true

                                Rectangle {
                                    id: block
                                    x: 0
                                    y: 0
                                    width: slot.width
                                    height: slot.height
                                    radius: Tokens.radius
                                    color: slot.shown ? Tokens.paperLift : "transparent"
                                    border.width: Tokens.border
                                    border.color: page.selectedSection === slot.modelData.id ? Tokens.lineStrong : Tokens.line
                                    opacity: slot.shown ? 1 : 0.58

                                    Rectangle {
                                        anchors.left: parent.left
                                        anchors.top: parent.top
                                        anchors.bottom: parent.bottom
                                        width: Tokens.s1
                                        color: slot.shown ? Tokens.bone : Tokens.line
                                    }
                                    Text {
                                        anchors.left: parent.left
                                        anchors.leftMargin: Tokens.s4
                                        anchors.verticalCenter: parent.verticalCenter
                                        width: parent.width - Tokens.s4 - 94
                                        text: I18n.tr(slot.definition ? slot.definition.label : slot.modelData.id)
                                        color: Tokens.ink
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fBody
                                        font.weight: Font.DemiBold
                                        elide: Text.ElideRight
                                    }
                                    Text {
                                        anchors.right: visibility.left
                                        anchors.rightMargin: Tokens.s2
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: "drag_indicator"
                                        color: Tokens.inkFaint
                                        font.family: "Material Symbols Rounded"
                                        font.pixelSize: Tokens.fBody
                                    }
                                    Rectangle {
                                        id: visibility
                                        anchors.right: parent.right
                                        anchors.rightMargin: Tokens.s3
                                        anchors.verticalCenter: parent.verticalCenter
                                        width: Tokens.ctlH
                                        height: width
                                        radius: Tokens.radius
                                        color: eyeHover.hovered ? Tokens.tint10 : "transparent"
                                        Text {
                                            anchors.centerIn: parent
                                            text: slot.shown ? "visibility" : "visibility_off"
                                            color: slot.shown ? Tokens.ink : Tokens.inkFaint
                                            font.family: "Material Symbols Rounded"
                                            font.pixelSize: Tokens.fBody
                                        }
                                        HoverHandler { id: eyeHover; cursorShape: Qt.PointingHandCursor }
                                        TapHandler {
                                            onTapped: page.setSectionVisible(slot.modelData.id, !slot.shown)
                                        }
                                    }

                                    MouseArea {
                                        anchors.left: parent.left
                                        anchors.right: visibility.left
                                        anchors.top: parent.top
                                        anchors.bottom: parent.bottom
                                        hoverEnabled: true
                                        cursorShape: pressed ? Qt.ClosedHandCursor : Qt.OpenHandCursor
                                        drag.target: block
                                        drag.axis: Drag.YAxis
                                        drag.minimumY: -slot.y
                                        drag.maximumY: previewColumn.height - slot.y - block.height
                                        onPressed: {
                                            page.selectedSection = slot.modelData.id;
                                            block.z = 10;
                                        }
                                        onReleased: {
                                            var centre = slot.y + block.y + block.height / 2;
                                            var target = 0;
                                            var best = Number.MAX_VALUE;
                                            for (var i = 0; i < previewRepeater.count; ++i) {
                                                var candidate = previewRepeater.itemAt(i);
                                                if (!candidate) continue;
                                                var distance = Math.abs(centre - (candidate.y + candidate.height / 2));
                                                if (distance < best) { best = distance; target = i; }
                                            }
                                            block.y = 0;
                                            block.z = 0;
                                            page.moveSection(slot.modelData.id, target);
                                        }
                                        onCanceled: { block.y = 0; block.z = 0; }
                                    }
                                }
                            }
                        }

                        Text {
                            width: parent.width
                            text: I18n.tr("Hidden blocks are not created by the shell, so their probes and animation stay stopped.")
                            color: Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                    }
                }
            }

            Column {
                width: Math.max(280, editor.width - previewCardHost.width - editor.spacing)
                spacing: Tokens.s4

                SettingCard {
                    width: parent.width
                    title: page.selectedModel ? I18n.tr(page.selectedModel.label.toUpperCase()) : I18n.tr("DETAILS")
                    kana: page.selectedModel ? page.selectedModel.kana : ""
                    collapsible: false

                    Item {
                        width: parent.width
                        height: detailColumn.height + Tokens.s4 * 2

                        Column {
                            id: detailColumn
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.top: parent.top
                            anchors.margins: Tokens.s4
                            spacing: Tokens.s3

                            Text {
                                width: parent.width
                                text: page.selectedModel && page.selectedModel.elements.length > 0
                                    ? I18n.tr("Tap a chip to include or remove that part.")
                                    : I18n.tr("This block has no smaller parts.")
                                color: Tokens.inkMuted
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fSmall
                                wrapMode: Text.WordWrap
                            }

                            Flow {
                                width: parent.width
                                spacing: Tokens.s2
                                Repeater {
                                    model: page.selectedModel ? page.selectedModel.elements : []
                                    delegate: VisibilityChip {
                                        required property var modelData
                                        label: modelData.label
                                        checked: page.elementVisible(modelData.id)
                                        onChanged: value => page.setElementVisible(modelData.id, value)
                                    }
                                }
                            }
                        }
                    }
                }

                SettingCard {
                    width: parent.width
                    title: I18n.tr("DEFAULT")
                    kana: I18n.tr("初期")
                    collapsible: false

                    SettingRow {
                        width: parent.width
                        label: I18n.tr("Restore panel")
                        desc: I18n.tr("Show the original blocks in their shipped order.")
                        source: "shell.json"
                        controlWidth: 100
                        Btn {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("RESET")
                            onAct: page.resetLayout()
                        }
                    }
                }
            }
        }
    }
}
