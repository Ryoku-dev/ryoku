pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../../../components"
import "../framebars/menus" as Menus
import Ryoku.Ui.Singletons

// Tools: the kept download / convert / install backends with a clean face.
// Paste a link to fetch it into the stash (cobalt drives a sequential queue);
// the list shows what landed; Convert compresses stashed media, Install runs a
// stashed package. No LocalSend, no file board -- those went with the old stash.
Item {
    id: root

    property real s: 1
    property bool open: false
    // The Cobalt first-run modal. Opened by the engine switch when the host is
    // not ready, so the setup is where the user already is rather than a
    // terminal they were told to open.
    property bool setupOpen: false
    signal pick(string mode)

    property string urlText: ""
    implicitHeight: col.implicitHeight + 24 * root.s

    // Fetch reads the link's formats. Nothing here probes on a keystroke: a
    // format list costs a network round trip, so it waits for a deliberate press
    // of the button or Enter in the field. The download that follows is the
    // row's own action, or the fallback pill when the link could not be read.
    readonly property bool probing: Stash.probeState === "probing"

    // Whether the field holds something Fetch could ask about: an http(s) scheme
    // and a non-empty, whitespace-free authority. Same family as the probe gate
    // in Stash, so a green border never promises something Fetch would refuse.
    // Advisory only: it colours the border and nothing else, the button's own
    // enabled state is unchanged.
    readonly property bool urlValid:
        /^https?:\/\/[^\s\/?#]+(?:[\/?#][^\s]*)?$/i.test(root.urlText.trim())

    function fetch() {
        if (root.urlText.trim().length === 0 || root.probing)
            return;
        // The mode travels with the chip, not the ask: one probe answers all
        // three, so switching chips only re-renders the cached lists.
        Stash.probe(root.urlText);
    }

    // An answer about a link that is no longer in the field is worse than no
    // answer: it would offer formats for something else. Dropped rather than
    // re-asked, since re-asking is the very request cost this removed.
    onUrlTextChanged: {
        if (Stash.probeState !== "idle" && root.urlText.trim() !== Stash.probedUrl)
            Stash.resetProbe();
    }

    function fileGlyph(name) {
        const e = ("" + name).toLowerCase().split(".").pop();
        if (/^(png|jpe?g|webp|gif|bmp|tiff?|avif)$/.test(e)) return "image";
        if (/^(mp4|mkv|webm|mov|avi|m4v)$/.test(e)) return "movie";
        if (/^(mp3|flac|wav|ogg|opus|m4a|aac)$/.test(e)) return "music_note";
        if (/^(zip|tar|gz|xz|zst|bz2|7z|rar|tgz)$/.test(e)) return "folder_zip";
        if (/^(appimage|deb|rpm|flatpak|pkg)$/.test(e)) return "deployed_code";
        return "draft";
    }

    // The id is what the script speaks (auto | audio | mute); only the word on the
    // chip is ours. "Video" says what the mode actually fetches, where "Auto"
    // named a policy the user never set.
    readonly property var modes: [
        { id: "auto", label: I18n.tr("Video") },
        { id: "audio", label: I18n.tr("Audio") },
        { id: "mute", label: I18n.tr("Mute") }
    ]

    // Sub-label under the engine switch, driven by docker + cobalt state. The
    // two dead-end strings this used to show ("Install Docker", "Start
    // docker.service or add yourself to the docker group") named a chore and did
    // nothing about it; the setup wizard does them, so the label now says a
    // switch flip is all that is needed.
    readonly property bool engineNeedsSetup: Stash.dockerState === "setup" || Stash.dockerState === "missing"
    readonly property string engineSub: {
        if (Stash.setupState === "running") return I18n.tr("Setting up…");
        if (Stash.dockerState === "missing") return I18n.tr("Needs Docker. `ryoku update` installs it");
        if (Stash.dockerState === "setup") return I18n.tr("One-time setup: flip the switch to run it");
        if (Stash.dockerState === "denied") return I18n.tr("Docker is unreachable and the ryoku-docker helper is missing");
        switch (Stash.cobaltState) {
        case "starting": return Stash.cobaltMsg === "pulling"
            ? I18n.tr("Downloading cobalt image — first launch takes a minute and uses some memory & CPU")
            : I18n.tr("Starting cobalt…");
        case "running": return I18n.tr("On — downloads run through your local cobalt");
        case "error": return Stash.cobaltMsg.length > 0 ? Stash.cobaltMsg : I18n.tr("Failed to start");
        default: return I18n.tr("Off — using yt-dlp");
        }
    }

    Flickable {
        anchors.fill: parent
        anchors.leftMargin: 18 * root.s
        anchors.rightMargin: 18 * root.s
        anchors.topMargin: 16 * root.s
        anchors.bottomMargin: 8 * root.s
        contentWidth: width
        contentHeight: col.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height

        Column {
            id: col
            width: parent.width
            spacing: 10 * root.s

            Menus.QsSection { width: parent.width; label: I18n.tr("Download") }

            // Engine switch: local cobalt (Docker) vs the yt-dlp fallback.
            Rectangle {
                width: parent.width
                radius: Theme.radiusWidget
                color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.05)
                implicitHeight: engineRow.implicitHeight + 16 * root.s

                Item {
                    id: engineRow
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.leftMargin: 9 * root.s
                    anchors.rightMargin: 9 * root.s
                    implicitHeight: Math.max(engineText.implicitHeight, 18 * root.s)

                    Column {
                        id: engineText
                        anchors.left: parent.left
                        anchors.right: engineRight.left
                        anchors.rightMargin: 8 * root.s
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 1 * root.s

                        Text {
                            text: I18n.tr("Cobalt engine")
                            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                            font.family: Theme.fontPrimary
                            font.pixelSize: 10 * root.s
                            font.weight: Font.DemiBold
                        }
                        Text {
                            width: parent.width
                            text: root.engineSub
                            wrapMode: Text.WordWrap
                            color: Stash.cobaltState === "error" ? Theme.vermLit
                                : Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                            font.family: Theme.fontPrimary
                            font.pixelSize: 8 * root.s
                        }
                    }

                    Row {
                        id: engineRight
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        spacing: 7 * root.s

                        MaterialIcon {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: Stash.cobaltState === "starting"
                            text: "progress_activity"
                            font.pixelSize: 13 * root.s
                            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                            RotationAnimation on rotation {
                                running: Stash.cobaltState === "starting"
                                loops: Animation.Infinite
                                from: 0; to: 360; duration: 900
                            }
                        }
                        LinkToggle {
                            id: engineSwitch
                            anchors.verticalCenter: parent.verticalCenter
                            s: root.s
                            readonly property bool engineOn: Stash.cobaltState === "running" || Stash.cobaltState === "starting"
                            on: engineOn
                            enabled: Stash.setupState !== "running"
                            onToggled: {
                                if (engineSwitch.engineOn) {
                                    Stash.setEngine(false);
                                    return;
                                }
                                // Turning on: anything short of a ready host goes
                                // through the wizard, which is the only thing that
                                // can start the service or grant access. A missing
                                // runtime opens it too, so the reason is on screen
                                // instead of hidden in a disabled switch.
                                if (root.engineNeedsSetup || Stash.dockerState === "unknown") {
                                    root.setupOpen = true;
                                    Stash.startSetup();
                                    return;
                                }
                                Stash.setEngine(true);
                            }
                        }
                    }
                }
            }

            Rectangle {
                width: parent.width
                height: 26 * root.s
                radius: Theme.radiusWidget
                color: Theme.surface
                border.width: Theme.borderWidth
                // Three states, one border. Empty is the field at rest (focus
                // still wins, so a caret never hides), a typed link reads its
                // verdict as you type.
                border.color: urlInput.activeFocus && root.urlText.trim().length === 0
                    ? Theme.primary
                    : root.urlText.trim().length === 0 ? Theme.outline
                    : root.urlValid
                        ? Theme.inkOn(Theme.effectiveSurface, Scheme.tertiary, 3.0)
                        : Theme.inkOn(Theme.effectiveSurface, Theme.error, 3.0)
                Behavior on border.color { ColorAnimation { duration: Motion.fast } }
                SumiEdge {}

                Row {
                    anchors.fill: parent
                    anchors.leftMargin: 12 * root.s
                    anchors.rightMargin: 12 * root.s
                    spacing: 9 * root.s

                    MaterialIcon {
                        anchors.verticalCenter: parent.verticalCenter
                        font.pixelSize: 13 * root.s
                        text: "link"
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                    }
                    TextInput {
                        id: urlInput
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - 27 * root.s - parent.spacing
                        text: root.urlText
                        onTextChanged: root.urlText = text
                        color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                        font.family: Theme.fontPrimary
                        font.pixelSize: 10.5 * root.s
                        clip: true
                        selectByMouse: true
                        onAccepted: root.fetch()

                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: urlInput.text.length === 0
                            text: I18n.tr("Paste a link to download")
                            color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                            font: urlInput.font
                        }
                    }
                }
            }

            Row {
                width: parent.width
                spacing: 6 * root.s
                readonly property real segW: (width - 2 * spacing) / 3

                Repeater {
                    model: root.modes
                    delegate: Rectangle {
                        required property var modelData
                        readonly property bool on: Stash.dlMode === modelData.id
                        width: parent.segW
                        height: 22 * root.s
                        radius: Theme.radiusWidget
                        color: on ? Theme.primary : Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.05)
                        border.width: 1
                        border.color: on ? "transparent" : Qt.rgba(Theme.outline.r, Theme.outline.g, Theme.outline.b, 0.3)
                        Behavior on color { ColorAnimation { duration: Motion.fast } }

                        Text {
                            anchors.centerIn: parent
                            text: I18n.tr(modelData.label)
                            color: parent.on ? Theme.inkOn(Theme.primary, Theme.onPrimary)
                                : Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                            font.family: Theme.fontPrimary
                            font.pixelSize: 9 * root.s
                            font.weight: parent.on ? Font.DemiBold : Font.Normal
                        }
                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: Stash.dlMode = modelData.id
                        }
                    }
                }
            }

            // "Fetch", not "Download": what this button does now is read the
            // link's formats. The download is a row's own action, and naming
            // this one after it invited tapping the wrong thing. It dims while
            // the read is in flight, because a second press would ask twice.
            ActionButton {
                width: parent.width
                label: I18n.tr("Fetch")
                icon: "search"
                busy: root.probing
                primary: root.urlText.trim().length > 0
                enabled: root.urlText.trim().length > 0 && !root.probing
                onTapped: root.fetch()
            }

            // One slot, four states: the sites bubble when nothing is asked,
            // a quiet line while reading, the tiers once they land, and the
            // fallback when the link cannot be read. It owns the tier rows, so
            // this file never has to know what a probe returned.
            QualityPicker {
                width: parent.width
                s: root.s
                url: root.urlText
                mode: Stash.dlMode
                onAccepted: root.urlText = ""
            }

            Column {
                width: parent.width
                spacing: 6 * root.s
                visible: Stash.queueModel.count > 0

                Repeater {
                    model: Stash.queueModel
                    delegate: Rectangle {
                        id: qRow
                        required property var model
                        required property int index
                        width: parent.width
                        height: 24 * root.s
                        radius: Theme.radiusWidget
                        color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.05)

                        Column {
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.leftMargin: 12 * root.s
                            anchors.rightMargin: 12 * root.s
                            spacing: 5 * root.s

                            Item {
                                width: parent.width
                                height: qName.implicitHeight

                                // The row's name is a video title as often as a file path now, and a
                                // title is not a single word: it is elided, never
                                // trimmed or re-cased, because the file's own
                                // words are the only thing that says what is
                                // being fetched.
                                Text {
                                    id: qName
                                    anchors.left: parent.left
                                    anchors.right: qActions.left
                                    anchors.rightMargin: 8 * root.s
                                    text: qRow.model.name && qRow.model.name.length > 0 ? qRow.model.name : qRow.model.arg
                                    elide: Text.ElideRight
                                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                                    font.family: Theme.fontPrimary
                                    font.pixelSize: 9 * root.s
                                }
                                Row {
                                    id: qActions
                                    anchors.right: parent.right
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: 7 * root.s

                                    // A row that was stopped is not a row that failed: same quiet ink as
                                        // queued, no red, and the same two
                                        // affordances a finished row gets.
                                        Text {
                                            anchors.verticalCenter: parent.verticalCenter
                                            text: qRow.model.state === "running" ? qRow.model.pct + "%"
                                                : qRow.model.state === "error" ? (qRow.model.msg && qRow.model.msg.length > 0 ? qRow.model.msg : I18n.tr("failed"))
                                                : qRow.model.state === "done" ? I18n.tr("done")
                                                : qRow.model.state === "queued" ? I18n.tr("queued")
                                                : qRow.model.state === "cancelled" ? I18n.tr("cancelled")
                                                : qRow.model.state
                                            color: qRow.model.state === "error" ? Theme.vermLit
                                                : Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                                            font.family: Theme.mono
                                            font.pixelSize: 8 * root.s
                                        }
                                        // Retry re-queues whatever the row was,
                                        // stopped included, so a cancel costs
                                        // the position in the queue and nothing
                                        // else.
                                        MaterialIcon {
                                            visible: qRow.model.state === "error" || qRow.model.state === "cancelled"
                                            anchors.verticalCenter: parent.verticalCenter
                                            font.pixelSize: 12 * root.s
                                            text: "refresh"
                                            color: Theme.inkOn(Theme.effectiveSurface, retryArea.containsMouse ? Theme.onSurface : Theme.onSurfaceVariant, 3.0)
                                            MouseArea {
                                                id: retryArea
                                                anchors.fill: parent
                                                anchors.margins: -6 * root.s
                                                hoverEnabled: true
                                                cursorShape: Qt.PointingHandCursor
                                                onClicked: Stash.retryJob(qRow.index)
                                            }
                                        }
                                        MaterialIcon {
                                            visible: qRow.model.state === "done" || qRow.model.state === "error" || qRow.model.state === "cancelled"
                                            anchors.verticalCenter: parent.verticalCenter
                                            font.pixelSize: 12 * root.s
                                            text: "close"
                                            color: Theme.inkOn(Theme.effectiveSurface, dropArea.containsMouse ? Theme.onSurface : Theme.onSurfaceVariant, 3.0)
                                            MouseArea {
                                                id: dropArea
                                                anchors.fill: parent
                                                anchors.margins: -6 * root.s
                                                hoverEnabled: true
                                                cursorShape: Qt.PointingHandCursor
                                                onClicked: Stash.dismissJob(qRow.index)
                                            }
                                        }
                                        // Cancel wears the dismiss icon on
                                        // purpose: it is the same shape as
                                        // "this row goes away", which is what
                                        // pressing it means, rather than a new
                                        // glyph the user has to learn.
                                        MaterialIcon {
                                            visible: qRow.model.state === "running"
                                            anchors.verticalCenter: parent.verticalCenter
                                            font.pixelSize: 12 * root.s
                                            text: "close"
                                            color: Theme.inkOn(Theme.effectiveSurface, cancelArea.containsMouse ? Theme.vermLit : Theme.onSurfaceVariant, 3.0)
                                            Behavior on color { ColorAnimation { duration: Motion.fast } }
                                            MouseArea {
                                                id: cancelArea
                                                anchors.fill: parent
                                                anchors.margins: -6 * root.s
                                                hoverEnabled: true
                                                cursorShape: Qt.PointingHandCursor
                                                onClicked: Stash.cancelJob(qRow.index)
                                            }
                                        }
                                }
                            }
                            Rectangle {
                                visible: qRow.model.state === "running" || qRow.model.state === "queued"
                                width: parent.width
                                height: 2
                                radius: 1
                                color: Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.1)
                                Rectangle {
                                    height: parent.height
                                    radius: 1
                                    width: parent.width * Math.max(0, Math.min(1, (qRow.model.pct || 0) / 100))
                                    color: Theme.primary
                                    Behavior on width { NumberAnimation { duration: Motion.fast } }
                                }
                            }
                        }
                    }
                }
            }

            Menus.QsSection { width: parent.width; label: I18n.tr("Recently downloaded") }

            Text {
                width: parent.width
                visible: Stash.count === 0
                text: I18n.tr("Nothing downloaded yet. Links you grab land here.")
                wrapMode: Text.WordWrap
                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                font.family: Theme.fontPrimary
                font.pixelSize: 9 * root.s
            }

            Column {
                width: parent.width
                spacing: 6 * root.s

                Repeater {
                    model: Stash.recentFiles
                    delegate: Rectangle {
                        id: fileRow
                        required property var modelData
                        width: parent.width
                        height: 24 * root.s
                        radius: Theme.radiusWidget
                        color: rowArea.containsMouse
                            ? Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.08)
                            : Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.04)
                        Behavior on color { ColorAnimation { duration: Motion.fast } }

                        MouseArea {
                            id: rowArea
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: Stash.openFile(fileRow.modelData.path)
                        }

                        Row {
                            anchors.left: parent.left
                            anchors.right: rmBtn.left
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.leftMargin: 12 * root.s
                            anchors.rightMargin: 6 * root.s
                            spacing: 10 * root.s

                            MaterialIcon {
                                anchors.verticalCenter: parent.verticalCenter
                                font.pixelSize: 13 * root.s
                                text: root.fileGlyph(fileRow.modelData.name)
                                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                width: parent.width - 28 * root.s - parent.spacing
                                text: fileRow.modelData.name
                                elide: Text.ElideMiddle
                                color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                                font.family: Theme.fontPrimary
                                font.pixelSize: 9 * root.s
                            }
                        }

                        MaterialIcon {
                            id: rmBtn
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.rightMargin: 10 * root.s
                            font.pixelSize: 13 * root.s
                            text: "close"
                            color: Theme.inkOn(Theme.effectiveSurface, rmArea.containsMouse ? Theme.onSurface : Theme.onSurfaceVariant, 3.0)
                            MouseArea {
                                id: rmArea
                                anchors.fill: parent
                                anchors.margins: -8 * root.s
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: Stash.removeFile(fileRow.modelData.path)
                            }
                        }
                    }
                }
            }

            Menus.QsSection { width: parent.width; label: I18n.tr("Convert & install") }

            ActionButton {
                width: parent.width
                label: I18n.tr("Compress video…")
                icon: "compress"
                onTapped: root.pick("compress")
            }
            ActionButton {
                width: parent.width
                label: I18n.tr("Install app…")
                icon: "install_desktop"
                onTapped: root.pick("install")
            }

            Item { width: 1; height: 6 * root.s }
        }
    }

    // Sits above the scrolling body so the modal covers the whole Tools face.
    CobaltSetupWizard {
        anchors.fill: parent
        s: root.s
        open: root.setupOpen
        onClosed: root.setupOpen = false
    }

    // primary = bone plate + dark ink; otherwise a quiet outlined tile.
    component ActionButton: Item {
        id: btn
        property string label: ""
        property string icon: ""
        property bool primary: false
        // A work in flight replaces the icon rather than stacking beside it, so
        // the button stays one object the same size whether or not it is busy.
        property bool busy: false
        signal tapped()

        implicitHeight: 24 * root.s
        opacity: btn.enabled ? (btn.busy ? 0.55 : 1) : 0.4
        Behavior on opacity { NumberAnimation { duration: Motion.fast } }

        Rectangle {
            anchors.fill: parent
            radius: Theme.radiusWidget
            color: btn.primary ? Theme.primary
                : ba.containsMouse ? Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.08)
                : Qt.rgba(Theme.onSurface.r, Theme.onSurface.g, Theme.onSurface.b, 0.04)
            border.width: btn.primary ? 0 : Theme.borderWidth
            border.color: Theme.outline
            Behavior on color { ColorAnimation { duration: Motion.fast } }
            SumiEdge { visible: btn.primary }

            Row {
                anchors.centerIn: parent
                spacing: 8 * root.s

                MaterialIcon {
                    anchors.verticalCenter: parent.verticalCenter
                    font.pixelSize: 13 * root.s
                    visible: !btn.busy && btn.icon.length > 0
                    text: btn.icon
                    color: btn.primary ? Theme.inkOn(Theme.primary, Theme.onPrimary)
                        : Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                }
                MaterialIcon {
                    anchors.verticalCenter: parent.verticalCenter
                    font.pixelSize: 13 * root.s
                    visible: btn.busy
                    text: "progress_activity"
                    color: btn.primary ? Theme.inkOn(Theme.primary, Theme.onPrimary)
                        : Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                    RotationAnimation on rotation {
                        running: btn.busy
                        loops: Animation.Infinite
                        from: 0; to: 360; duration: 900
                    }
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr(btn.label)
                    color: btn.primary ? Theme.inkOn(Theme.primary, Theme.onPrimary)
                        : Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                    font.family: Theme.fontPrimary
                    font.pixelSize: 9 * root.s
                    font.weight: Font.DemiBold
                }
            }
        }

        scale: ba.pressed && btn.enabled ? 0.97 : 1
        Behavior on scale { NumberAnimation { duration: Motion.fast } }

        MouseArea {
            id: ba
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onClicked: btn.tapped()
        }
    }
}
