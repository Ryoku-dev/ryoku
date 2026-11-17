pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services
import "../../services" as Services
import "../../services/lib/markdown.js" as Md

Item {
    id: root

    required property real s
    required property Services.AskSession session
    property bool active: false
    property int selectedChip: 0
    property string flash: ""
    property int chipEpoch: 0
    readonly property bool motionAllowed: active && !Tokens.reduceMotion && !Motion.reduce
    readonly property var chips: {
        if (session.busy)
            return [
                { kind: "chat", value: "", label: I18n.tr("Move to the agent") },
                { kind: "app", value: "", label: I18n.tr("Open Rashin") },
                { kind: "cancel", value: "", label: I18n.tr("Cancel") }
            ];
        if (session.phase !== "done")
            return [];
        const values = [];
        if (session.needsAgent)
            values.push({ kind: "chat", value: "", label: I18n.tr("Move to the agent") });
        values.push({ kind: "copy", value: session.answerText, label: I18n.tr("Copy") });
        for (let i = 0; i < session.answerActions.length; i++)
            values.push(session.answerActions[i]);
        if (!session.needsAgent)
            values.push({ kind: "chat", value: "", label: I18n.tr("Continue with the agent") });
        values.push({ kind: "app", value: "", label: I18n.tr("Open Rashin") });
        values.push({ kind: "pin", value: "", label: I18n.tr("Pin bubble") });
        return values;
    }

    signal switchToChat()
    signal openApp()
    signal recentLoaded(string question)
    signal pinBubbleRequested()

    implicitHeight: content.implicitHeight

    function move(delta) {
        if (session.recentOpen) {
            recentList.currentIndex = Math.max(0, Math.min(recentList.count - 1, recentList.currentIndex + delta));
            return;
        }
        selectedChip = Math.max(0, Math.min(chips.length - 1, selectedChip + delta));
    }

    function activate() {
        if (session.recentOpen) {
            if (recentList.currentIndex >= 0 && session.recent[recentList.currentIndex]) {
                const entry = session.recent[recentList.currentIndex];
                session.loadRecent(entry);
                root.recentLoaded(session.askedQuestion);
            }
            return;
        }
        if (chips[selectedChip])
            fire(chips[selectedChip]);
    }

    function fire(chip) {
        const kind = String(chip.kind || "");
        if (kind === "copy" || kind === "cmd" || kind === "color") {
            Quickshell.execDetached(["sh", "-c", "printf '%s' \"$1\" | wl-copy", "_", String(chip.value || "")]);
            flash = kind + "\u0000" + String(chip.value || "");
            flashTimer.restart();
            return;
        }
        if (kind === "file")
            Spawn.runApp(["kitty", "-e", "nvim", String(chip.value || "")], { label: "kitty" });
        else if (kind === "dir" || kind === "url")
            Spawn.runApp(["xdg-open", String(chip.value || "")], { label: kind });
        else if (kind === "chat") {
            const run = session.busy || session.needsAgent;
            Needle.handoff(
                run ? "" : session.answerId,
                session.askedQuestion,
                run ? "" : session.answerText,
                run
            );
            root.switchToChat();
        }
        else if (kind === "app")
            root.openApp();
        else if (kind === "pin")
            root.pinBubbleRequested();
        else if (kind === "cancel")
            session.cancel();
    }

    function caption(chip) {
        if (flash === String(chip.kind) + "\u0000" + String(chip.value || ""))
            return I18n.tr("COPIED");
        if (chip.kind === "file") return "nvim " + chip.label;
        if (chip.kind === "dir") return I18n.tr("Open %1").arg(chip.label);
        if (chip.kind === "cmd") return "$ " + chip.label;
        return chip.label;
    }

    Timer {
        id: flashTimer
        interval: Motion.spatial * 3
        onTriggered: root.flash = ""
    }


    Connections {
        target: root.session
        function onAnswered() {
            root.selectedChip = 0;
            root.chipEpoch++;
        }
    }
    Column {
        id: content
        width: parent.width
        spacing: Tokens.s3 * root.s

        Text {
            id: workingLabel
            visible: root.session.busy
            text: root.session.working
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall * root.s
            SequentialAnimation on opacity {
                running: root.motionAllowed && root.session.busy
                loops: Animation.Infinite
                NumberAnimation { from: 1; to: 0.7; duration: Tokens.dur(900); easing.type: Easing.InOutSine }
                NumberAnimation { from: 0.7; to: 1; duration: Tokens.dur(900); easing.type: Easing.InOutSine }
                onStopped: workingLabel.opacity = 1
            }
        }

        Text {
            visible: root.session.recentOpen && root.session.recent.length === 0
            text: I18n.tr("No recent asks")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall * root.s
        }

        ListView {
            id: recentList
            width: parent.width
            visible: root.session.recentOpen
            height: visible ? Math.min(contentHeight, 260 * root.s) : 0
            clip: true
            model: root.session.recent
            currentIndex: count > 0 ? 0 : -1
            spacing: Tokens.s1 * root.s
            boundsBehavior: Flickable.StopAtBounds
            delegate: Rectangle {
                id: recentRow
                required property var modelData
                required property int index
                width: ListView.view ? ListView.view.width : 0
                height: 52 * root.s
                radius: Tokens.radius * root.s
                color: recentRow.index === recentList.currentIndex ? Tokens.bone : recentHover.hovered ? Tokens.tint10 : "transparent"
                Column {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.margins: Tokens.s3 * root.s
                    Text {
                        width: parent.width
                        text: String(recentRow.modelData.q || "")
                        color: recentRow.index === recentList.currentIndex ? Tokens.inkOnBone : Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        text: String(recentRow.modelData.a || "").replace(/\n/g, " ")
                        color: recentRow.index === recentList.currentIndex ? Tokens.inkOnBone : Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fTiny * root.s
                        elide: Text.ElideRight
                    }
                }
                HoverHandler { id: recentHover; cursorShape: Qt.PointingHandCursor; onHoveredChanged: if (hovered) recentList.currentIndex = recentRow.index }
                TapHandler {
                    onTapped: {
                        recentList.currentIndex = recentRow.index;
                        root.activate();
                    }
                }
            }
        }

        TextEdit {
            id: answerText
            readonly property bool shown: root.session.phase === "done" && !root.session.recentOpen
            width: parent.width
            visible: opacity > 0.001
            text: Md.prepare(root.session.answerText)
            textFormat: TextEdit.MarkdownText
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody * root.s
            wrapMode: TextEdit.Wrap
            readOnly: true
            selectByMouse: true
            selectionColor: Tokens.bone
            selectedTextColor: Tokens.inkOnBone
            onLinkActivated: url => {
                const safe = Md.safeUrl(url);
                if (safe.length > 0)
                    Spawn.runApp(["xdg-open", safe], { label: "browser" });
            }
            opacity: shown ? 1 : 0
            transform: Translate {
                y: answerText.shown ? 0 : Tokens.s2 * root.s
                Behavior on y {
                    enabled: root.motionAllowed
                    NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
                }
            }
            Behavior on opacity {
                enabled: root.motionAllowed
                NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
            }
        }

        Text {
            width: parent.width
            visible: root.session.needsAgent && root.session.needsAgentReason.length > 0
                && !root.session.recentOpen
            text: root.session.needsAgentReason
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall * root.s
            font.italic: true
            wrapMode: Text.WordWrap
        }

        Row {
            visible: root.session.phase === "done" && root.session.answerImages.length > 0
            spacing: Tokens.s2 * root.s
            Repeater {
                model: root.session.answerImages
                delegate: Rectangle {
                    id: imageFrame
                    required property string modelData
                    width: 92 * root.s
                    height: width
                    radius: Tokens.radius * root.s
                    color: Tokens.tint5
                    border.width: Tokens.border
                    border.color: Tokens.lineSoft
                    clip: true
                    Image { anchors.fill: parent; source: "file://" + imageFrame.modelData; fillMode: Image.PreserveAspectCrop; asynchronous: true }
                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: Spawn.runApp(["xdg-open", imageFrame.modelData], { label: "image" }) }
                }
            }
        }

        Row {
            width: parent.width
            visible: root.session.phase === "failed"
            spacing: Tokens.s2 * root.s
            Rectangle {
                width: 6 * root.s
                height: width
                radius: width / 2
                color: Tokens.alert
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                width: content.width - 6 * root.s - parent.spacing
                text: root.session.errorText
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * root.s
                wrapMode: Text.WordWrap
            }
        }

        Flow {
            width: parent.width
            visible: root.chips.length > 0 && !root.session.recentOpen
            spacing: Tokens.s2 * root.s
            Repeater {
                model: root.chips
                delegate: Rectangle {
                    id: chip
                    required property var modelData
                    required property int index
                    property real revealProgress: root.motionAllowed ? 0 : 1
                    property int revealEpoch: root.chipEpoch
                    readonly property bool current: index === root.selectedChip
                    width: chipContent.implicitWidth + Tokens.s4 * root.s
                    height: Tokens.ctlH * root.s
                    radius: Tokens.radius * root.s
                    color: chipTap.pressed ? Tokens.tint16
                        : current ? Tokens.bone
                        : chipHover.hovered ? Tokens.tint10 : "transparent"
                    border.width: Tokens.border
                    border.color: current ? Tokens.bone : Tokens.line
                    opacity: revealProgress
                    scale: chipTap.pressed && root.motionAllowed ? 0.94
                        : chipHover.hovered && root.motionAllowed ? 1.025 : 1
                    transform: Translate { y: (1 - chip.revealProgress) * Tokens.s2 * root.s }
                    function restartReveal() {
                        reveal.stop();
                        revealProgress = root.motionAllowed ? 0 : 1;
                        if (root.motionAllowed)
                            reveal.restart();
                    }
                    onRevealEpochChanged: restartReveal()
                    Component.onCompleted: restartReveal()
                    Behavior on scale {
                        enabled: root.motionAllowed
                        NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
                    }
                    SequentialAnimation {
                        id: reveal
                        PauseAnimation { duration: Math.min(chip.index * 60, 240) }
                        NumberAnimation {
                            target: chip
                            property: "revealProgress"
                            to: 1
                            duration: Tokens.move
                            easing.type: Tokens.ease
                        }
                    }
                    Row {
                        id: chipContent
                        anchors.centerIn: parent
                        spacing: Tokens.s2 * root.s
                        Rectangle {
                            visible: String(chip.modelData.kind || "") === "color"
                            width: 10 * root.s
                            height: width
                            radius: Tokens.radius * root.s
                            color: visible ? String(chip.modelData.value || Tokens.ink) : Tokens.ink
                            border.width: Tokens.border
                            border.color: chip.current ? Tokens.inkOnBone : Tokens.lineStrong
                        }
                        Text {
                            id: chipLabel
                            text: root.caption(chip.modelData)
                            color: chip.current ? Tokens.inkOnBone : Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall * root.s
                        }
                    }
                    HoverHandler { id: chipHover; cursorShape: Qt.PointingHandCursor; onHoveredChanged: if (hovered) root.selectedChip = chip.index }
                    TapHandler { id: chipTap; onTapped: root.fire(chip.modelData) }
                }
            }
        }
    }
}
