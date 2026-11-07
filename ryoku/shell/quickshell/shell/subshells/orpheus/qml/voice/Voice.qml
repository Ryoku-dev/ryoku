pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K
import "../settings" as S
import "." as V

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: true

    property double nowMs: Date.now()
    property double mediaUntil: 0
    readonly property bool unread: Notifs.hasUnread
    property string displayState: ""
    property var displayNotice: null
    property real contentPhase: 1
    property real squashY: 1

    readonly property var newestNotice: {
        const tracked = Notifs.tracked || [];
        let newest = null;
        let newestAt = 0;
        for (let index = 0; index < tracked.length; ++index) {
            const notification = tracked[index];
            const arrived = Number(Notifs.arrivalMs[notification.id] || 0);
            if (arrived > newestAt) {
                newest = notification;
                newestAt = arrived;
            }
        }
        return newest;
    }
    readonly property double newestArrival: newestNotice
        ? Number(Notifs.arrivalMs[newestNotice.id] || 0) : 0
    readonly property string noticeVersion: newestNotice
        ? String(newestNotice.id) + ":" + newestArrival : ""
    readonly property bool noticeFresh: !Notifs.dnd && newestNotice !== null
        && newestArrival > 0 && nowMs - newestArrival < 4000
    readonly property bool mediaRecent: Media.playing
        || (Media.present && nowMs < mediaUntil)
    readonly property string voiceState: Recorder.active ? "recording"
        : noticeFresh ? "notice"
        : mediaRecent && !S.Prefs.hasModule("media") ? "media"
        : "idle"
    readonly property string page: voiceState === "recording" ? "recording"
        : voiceState === "notice" ? "notifications"
        : voiceState === "media" ? "media" : "today"
    readonly property var bubbles: ({
        left: voiceState !== "idle" && !S.Prefs.hasModule("clock") ? ["time"] : [],
        right: voiceState === "recording" && Media.playing
            && !S.Prefs.hasModule("media") ? ["media"] : []
    })
    readonly property bool inlineBubbles: S.Prefs.span !== "islands"
    readonly property var voicePages: {
        const pages = ["today"];
        if (mediaRecent)
            pages.push("media");
        if ((Notifs.history || []).length > 0)
            pages.push("notifications");
        if (Recorder.active)
            pages.push("recording");
        return pages;
    }
    readonly property string hintText: voiceState === "recording" ? "Recording details"
        : voiceState === "notice" && newestNotice
            ? String(newestNotice.summary || "Notifications")
        : voiceState === "media" && Media.player
            ? String(Media.player.trackTitle || "Now playing")
        : "Today"

    state: voiceState
    states: [
        State { name: "idle" },
        State { name: "media" },
        State { name: "notice" },
        State { name: "recording" }
    ]
    implicitWidth: content.implicitWidth + 2 * K.Style.hitPad
    implicitHeight: K.Style.barH
    height: K.Style.barH

    transform: Scale {
        origin.x: root.width / 2
        origin.y: 0
        yScale: root.squashY
    }

    function armMedia() {
        if (Media.playing)
            mediaUntil = 0;
        else if (Media.present)
            mediaUntil = Date.now() + 60000;
        else
            mediaUntil = 0;
        scheduleExpiry();
    }

    function scheduleExpiry() {
        const current = Date.now();
        root.nowMs = current;
        let next = 0;
        const noticeEnd = root.newestArrival + 4000;
        if (!Notifs.dnd && root.newestArrival > 0 && noticeEnd > current)
            next = noticeEnd;
        if (!Media.playing && Media.present && root.mediaUntil > current
                && (next === 0 || root.mediaUntil < next))
            next = root.mediaUntil;
        expiryTimer.stop();
        if (next > 0) {
            expiryTimer.interval = Math.max(1, Math.ceil(next - current));
            expiryTimer.start();
        }
    }


    function recompose() {
        if (displayState.length === 0 || K.Style.still) {
            displayState = voiceState;
            displayNotice = newestNotice;
            contentPhase = 1;
            return;
        }
        recomposeAnimation.restart();
    }

    function togglePage() {
        if (scene.cardKey !== page)
            squashAnimation.restart();
        scene.toggleCard(page, root);
    }

    function cyclePage(steps) {
        const pages = voicePages;
        const current = pages.indexOf(scene.cardKey);
        if (current < 0 || steps === 0)
            return;
        const direction = steps > 0 ? -1 : 1;
        const next = (current + direction + pages.length) % pages.length;
        scene.openCard(pages[next], root, {});
    }

    function syncAudioClaim() {
        AudioBars.setActive(root, root.visible && root.voiceState === "media" && Media.playing);
    }

    onVoiceStateChanged: {
        recompose();
        syncAudioClaim();
    }
    onVisibleChanged: syncAudioClaim()
    onNoticeVersionChanged: {
        scheduleExpiry();
        if (newestArrival > 0 && !Notifs.dnd && displayState === "notice")
            recompose();
    }


    Component.onCompleted: {
        armMedia();
        displayState = voiceState;
        displayNotice = newestNotice;
        syncAudioClaim();
    }
    Component.onDestruction: AudioBars.setActive(root, false)

    Connections {
        target: Media

        function onPlayingChanged() {
            root.armMedia();
            root.syncAudioClaim();
        }

        function onPresentChanged() {
            root.armMedia();
            root.syncAudioClaim();
        }

        function onPlayerChanged() {
            root.armMedia();
        }
    }
    Connections {
        target: Notifs
        function onDndChanged() { root.scheduleExpiry(); }
    }


    Timer {
        id: expiryTimer
        repeat: false
        onTriggered: root.scheduleExpiry()
    }


    SequentialAnimation {
        id: recomposeAnimation

        NumberAnimation {
            target: root
            property: "contentPhase"
            to: 0
            duration: K.Style.ms(90)
            easing.type: Easing.OutQuad
        }
        ScriptAction {
            script: {
                root.displayState = root.voiceState;
                root.displayNotice = root.newestNotice;
            }
        }
        PauseAnimation { duration: K.Style.ms(60) }
        NumberAnimation {
            target: root
            property: "contentPhase"
            to: 1
            duration: K.Style.ms(200)
            easing.bezierCurve: K.Style.effectsCurve
        }
    }

    SequentialAnimation {
        id: squashAnimation

        NumberAnimation {
            target: root
            property: "squashY"
            to: 1.04
            duration: K.Style.ms(110)
            easing.type: Easing.OutCubic
        }
        SpringAnimation {
            target: root
            property: "squashY"
            to: 1
            spring: 3
            damping: 0.2
            mass: 1
            epsilon: 0.001
        }
    }

    Component {
        id: idleFace
        V.VoiceIdle { scene: root.scene }
    }

    Component {
        id: mediaFace
        V.VoiceMedia { scene: root.scene }
    }

    Component {
        id: recordingFace
        V.VoiceRecording { scene: root.scene }
    }

    Component {
        id: noticeFace
        V.VoiceNotice {
            scene: root.scene
            notification: root.displayNotice
        }
    }

    Component {
        id: timeBubble
        V.TimeBubble { scene: root.scene }
    }

    Component {
        id: mediaBubble
        V.MediaBubble { scene: root.scene }
    }

    Row {
        id: content
        anchors.centerIn: parent
        spacing: K.Style.barGap

        Loader {
            active: root.inlineBubbles && root.bubbles.left.indexOf("time") >= 0
            visible: active
            sourceComponent: timeBubble
        }

        Item {
            anchors.verticalCenter: parent.verticalCenter
            implicitWidth: faceLoader.item ? faceLoader.item.implicitWidth : 0
            implicitHeight: K.Style.barH
            width: implicitWidth
            height: implicitHeight
            opacity: root.contentPhase
            y: 6 * (1 - root.contentPhase)

            Loader {
                id: faceLoader
                anchors.centerIn: parent
                sourceComponent: root.displayState === "recording" ? recordingFace
                    : root.displayState === "notice" ? noticeFace
                    : root.displayState === "media" ? mediaFace : idleFace
            }
        }

        K.Badge {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.unread && root.displayState !== "notice"
            dot: true
        }

        Loader {
            active: root.inlineBubbles && root.bubbles.right.indexOf("media") >= 0
            visible: active
            sourceComponent: mediaBubble
        }
    }

    K.Press {
        id: press
        radius: K.Style.radius
        onClicked: root.togglePage()
        onRightClicked: root.scene.openCard("controls", root, {})
        onMiddleClicked: if (Media.present) Media.toggle()
        wheelSteps: true
        onWheel: steps => root.cyclePage(steps)
        onHoveredChanged: if (S.Prefs.hoverOpen)
            root.scene.hoverCard(root.page, root, hovered)
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: press.hovered
    }
}
