pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services
import ".."
import "." as Cards

Item {
    id: root

    required property real s
    required property bool open
    required property real reveal
    required property bool tabActive
    property int index: 0
    property string page: ""
    signal requestClose()
    signal pick(string mode)

    property string urlText: ""
    property bool setupOpen: false
    property bool pickerOpen: false
    property string pickerMode: "compress"

    implicitHeight: card.implicitHeight

    readonly property bool engineNeedsSetup: Stash.dockerState === "setup" || Stash.dockerState === "missing"
    readonly property string engineSub: {
        if (Stash.setupState === "running") return I18n.tr("Setting up…");
        if (Stash.dockerState === "missing") return I18n.tr("Needs Docker. `ryoku update` installs it");
        if (Stash.dockerState === "setup") return I18n.tr("One-time setup — turn it on to begin");
        if (Stash.dockerState === "denied") return I18n.tr("Docker is unreachable and the ryoku-docker helper is missing");
        switch (Stash.cobaltState) {
        case "starting": return Stash.cobaltMsg === "pulling"
            ? I18n.tr("Downloading the cobalt image. First launch may take a minute.")
            : I18n.tr("Starting cobalt…");
        case "running": return I18n.tr("On — downloads use your local cobalt");
        case "error": return Stash.cobaltMsg.length > 0 ? Stash.cobaltMsg : I18n.tr("Failed to start");
        default: return I18n.tr("Off — using yt-dlp");
        }
    }

    function fileGlyph(name) {
        const ext = String(name).toLowerCase().split(".").pop();
        if (/^(png|jpe?g|webp|gif|bmp|tiff?|avif)$/.test(ext)) return "image";
        if (/^(mp4|mkv|webm|mov|avi|m4v)$/.test(ext)) return "movie";
        if (/^(mp3|flac|wav|ogg|opus|m4a|aac)$/.test(ext)) return "music_note";
        if (/^(zip|tar|gz|xz|zst|bz2|7z|rar|tgz)$/.test(ext)) return "folder_zip";
        if (/^(appimage|deb|rpm|flatpak|pkg)$/.test(ext)) return "deployed_code";
        return "draft";
    }

    function startDownload() {
        if (root.urlText.trim().length === 0)
            return;
        Stash.enqueueDownload(root.urlText, Stash.dlMode);
        root.urlText = "";
    }

    function openPicker(mode) {
        root.pickerMode = mode;
        root.pickerOpen = true;
        root.pick(mode);
    }

    property string lastPage: ""
    function applyPage() {
        if (root.page === root.lastPage)
            return;
        root.lastPage = root.page;
        if (root.page === "compress" || root.page === "install")
            root.openPicker(root.page);
    }

    onPageChanged: root.applyPage()
    onTabActiveChanged: {
        if (!root.tabActive)
            root.lastPage = "";
        else
            root.applyPage();
    }
    Component.onCompleted: root.applyPage()

    SidebarCardShell {
        id: card
        width: root.width
        index: root.index
        open: root.open
        reveal: root.reveal
        tabActive: root.tabActive
        title: I18n.tr("Stash tools")
        glyph: "download"
        eyebrow: I18n.tr("DOWNLOADS & PACKAGES")

        Column {
            width: parent.width
            spacing: Tokens.s5 * root.s

            Section {
                width: parent.width
                title: I18n.tr("DOWNLOAD")

                Column {
                    width: parent.width
                    spacing: Tokens.s3 * root.s

                    Rectangle {
                        width: parent.width
                        implicitHeight: engineBody.implicitHeight + Tokens.s4 * root.s * 2
                        radius: Tokens.radius * root.s
                        color: Tokens.tint5
                        border.width: Tokens.border
                        border.color: Stash.cobaltState === "error" ? Tokens.alert : Tokens.lineSoft

                        Row {
                            id: engineBody
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.top: parent.top
                            anchors.margins: Tokens.s4 * root.s
                            spacing: Tokens.s3 * root.s

                            Rectangle {
                                width: 38 * root.s
                                height: width
                                radius: Tokens.radius * root.s
                                color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.13)
                                Text {
                                    font.family: "Material Symbols Rounded"
                                    anchors.centerIn: parent
                                    text: Stash.cobaltState === "starting" ? "progress_activity" : "bolt"
                                    color: Tokens.sun
                                    font.pixelSize: 20 * root.s
                                    RotationAnimation on rotation {
                                        running: Stash.cobaltState === "starting" && !Tokens.reduceMotion
                                        loops: Animation.Infinite
                                        from: 0
                                        to: 360
                                        duration: 900
                                    }
                                }
                            }

                            Column {
                                width: parent.width - 38 * root.s - engineSwitch.width - parent.spacing * 2
                                spacing: Tokens.s1 * root.s
                                Text {
                                    width: parent.width
                                    text: I18n.tr("Cobalt engine")
                                    color: Tokens.ink
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fRow * root.s
                                    font.weight: Font.DemiBold
                                }
                                Text {
                                    width: parent.width
                                    text: root.engineSub
                                    color: Stash.cobaltState === "error" ? Tokens.alert : Tokens.inkMuted
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fSmall * root.s
                                    wrapMode: Text.WordWrap
                                }
                            }

                            Sw {
                                id: engineSwitch
                                anchors.verticalCenter: parent.verticalCenter
                                readonly property bool engineOn: Stash.cobaltState === "running" || Stash.cobaltState === "starting"
                                on: engineOn
                                enabled: Stash.setupState !== "running"
                                onToggled: {
                                    if (engineSwitch.engineOn) {
                                        Stash.setEngine(false);
                                        return;
                                    }
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

                    Row {
                        width: parent.width
                        spacing: Tokens.s2 * root.s

                        Field {
                            id: urlField
                            width: parent.width - downloadButton.width - parent.spacing
                            placeholder: I18n.tr("Paste a link to download")
                            tabular: true
                            text: root.urlText
                            onEdited: value => root.urlText = value
                            onAccepted: root.startDownload()
                        }
                        Btn {
                            id: downloadButton
                            text: I18n.tr("Download")
                            primary: true
                            armed: root.urlText.trim().length > 0
                            onAct: root.startDownload()
                        }
                    }

                    Seg {
                        width: parent.width
                        options: ["auto", "audio", "mute"]
                        labels: ({ auto: I18n.tr("Auto"), audio: I18n.tr("Audio"), mute: I18n.tr("Mute") })
                        current: Stash.dlMode
                        onChose: key => Stash.dlMode = key
                    }

                    Rectangle {
                        width: parent.width
                        implicitHeight: siteBody.implicitHeight + Tokens.s3 * root.s * 2
                        radius: Tokens.radius * root.s
                        color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.07)
                        border.width: Tokens.border
                        border.color: Tokens.lineSoft

                        Column {
                            id: siteBody
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.top: parent.top
                            anchors.margins: Tokens.s3 * root.s
                            spacing: Tokens.s1 * root.s
                            Text {
                                text: Stash.cobaltState === "running"
                                    ? I18n.tr("WORKS WITH %1 SITES").arg(Stash.supportedSites.length)
                                    : I18n.tr("POWERED BY YT-DLP")
                                color: Tokens.sun
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fTiny * root.s
                                font.weight: Font.DemiBold
                                font.letterSpacing: Tokens.trackMark
                            }
                            Text {
                                width: parent.width
                                text: Stash.cobaltState === "running"
                                    ? Stash.supportedSites.map(site => site === "twitter" ? "x" : site).join("  ·  ")
                                    : I18n.tr("Works with 1000+ sites, including YouTube, Twitter/X, Reddit, TikTok, and more.")
                                wrapMode: Text.WordWrap
                                color: Tokens.inkDim
                                font.family: Tokens.mono
                                font.pixelSize: Tokens.fTiny * root.s
                                lineHeight: 1.3
                            }
                        }
                    }
                }
            }

            Section {
                width: parent.width
                visible: Stash.queueModel.count > 0
                title: I18n.tr("QUEUE")

                Column {
                    width: parent.width
                    spacing: Tokens.s2 * root.s

                    Repeater {
                        model: Stash.queueModel
                        delegate: Rectangle {
                            id: queueRow
                            required property var model
                            required property int index
                            width: parent.width
                            implicitHeight: queueBody.implicitHeight + Tokens.s3 * root.s * 2
                            radius: Tokens.radius * root.s
                            color: Tokens.tint5
                            border.width: Tokens.border
                            border.color: queueRow.model.state === "error" ? Tokens.alert : Tokens.lineSoft

                            Column {
                                id: queueBody
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.top: parent.top
                                anchors.margins: Tokens.s3 * root.s
                                spacing: Tokens.s2 * root.s

                                Item {
                                    width: parent.width
                                    height: Math.max(queueName.implicitHeight, queueActions.implicitHeight)

                                    Text {
                                        id: queueName
                                        anchors.left: parent.left
                                        anchors.right: queueActions.left
                                        anchors.rightMargin: Tokens.s2 * root.s
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: queueRow.model.name && queueRow.model.name.length > 0 ? queueRow.model.name : queueRow.model.arg
                                        elide: Text.ElideMiddle
                                        color: Tokens.ink
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fSmall * root.s
                                        font.weight: Font.Medium
                                    }

                                    Row {
                                        id: queueActions
                                        anchors.right: parent.right
                                        anchors.verticalCenter: parent.verticalCenter
                                        spacing: Tokens.s1 * root.s

                                        Text {
                                            anchors.verticalCenter: parent.verticalCenter
                                            text: queueRow.model.state === "running" ? queueRow.model.pct + "%"
                                                : queueRow.model.state === "error" ? (queueRow.model.msg && queueRow.model.msg.length > 0 ? queueRow.model.msg : I18n.tr("failed"))
                                                : queueRow.model.state === "done" ? I18n.tr("done")
                                                : queueRow.model.state === "queued" ? I18n.tr("queued")
                                                : queueRow.model.state
                                            color: queueRow.model.state === "error" ? Tokens.alert : Tokens.inkMuted
                                            font.family: Tokens.mono
                                            font.pixelSize: Tokens.fTiny * root.s
                                        }
                                        IconBtn {
                                            visible: queueRow.model.state === "error"
                                            glyph: "↻"
                                            onAct: Stash.retryJob(queueRow.index)
                                        }
                                        IconBtn {
                                            visible: queueRow.model.state === "done" || queueRow.model.state === "error"
                                            glyph: "×"
                                            onAct: Stash.dismissJob(queueRow.index)
                                        }
                                    }
                                }

                                Rectangle {
                                    visible: queueRow.model.state === "running" || queueRow.model.state === "queued"
                                    width: parent.width
                                    height: 3 * root.s
                                    radius: height / 2
                                    color: Tokens.tint10
                                    Rectangle {
                                        width: parent.width * Math.max(0, Math.min(1, (queueRow.model.pct || 0) / 100))
                                        height: parent.height
                                        radius: parent.radius
                                        color: Tokens.sun
                                        Behavior on width { NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease } }
                                    }
                                }
                            }
                        }
                    }
                }
            }

            Section {
                width: parent.width
                title: I18n.tr("RECENTLY DOWNLOADED")

                Column {
                    width: parent.width
                    spacing: Tokens.s3 * root.s

                    Text {
                        width: parent.width
                        visible: Stash.count === 0
                        text: I18n.tr("Nothing downloaded yet. Links you grab land here.")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        wrapMode: Text.WordWrap
                    }

                    Flow {
                        id: recentGrid
                        width: parent.width
                        spacing: Tokens.s2 * root.s

                        Repeater {
                            model: Stash.recentFiles
                            delegate: Rectangle {
                                id: fileTile
                                required property var modelData
                                width: (recentGrid.width - recentGrid.spacing) / 2
                                height: 82 * root.s
                                radius: Tokens.radius * root.s
                                color: fileHover.hovered ? Tokens.tint10 : Tokens.tint5
                                border.width: Tokens.border
                                border.color: fileHover.hovered ? Tokens.lineStrong : Tokens.lineSoft
                                y: fileHover.hovered && !Tokens.reduceMotion ? -2 * root.s : 0
                                Behavior on y { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }
                                Behavior on color { ColorAnimation { duration: Tokens.snap } }

                                Text {
                                    font.family: "Material Symbols Rounded"
                                    anchors.left: parent.left
                                    anchors.top: parent.top
                                    anchors.leftMargin: Tokens.s3 * root.s
                                    anchors.topMargin: Tokens.s3 * root.s
                                    text: root.fileGlyph(fileTile.modelData.name)
                                    color: Tokens.sun
                                    font.pixelSize: 20 * root.s
                                }
                                IconBtn {
                                    anchors.right: parent.right
                                    anchors.top: parent.top
                                    anchors.rightMargin: Tokens.s1 * root.s
                                    anchors.topMargin: Tokens.s1 * root.s
                                    glyph: "×"
                                    onAct: Stash.removeFile(fileTile.modelData.path)
                                }
                                Text {
                                    anchors.left: parent.left
                                    anchors.right: parent.right
                                    anchors.bottom: parent.bottom
                                    anchors.margins: Tokens.s3 * root.s
                                    text: fileTile.modelData.name
                                    elide: Text.ElideMiddle
                                    color: Tokens.ink
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fTiny * root.s
                                    font.weight: Font.Medium
                                }

                                HoverHandler { id: fileHover; cursorShape: Qt.PointingHandCursor }
                                TapHandler { onTapped: Stash.openFile(fileTile.modelData.path) }
                            }
                        }
                    }
                }
            }

            Section {
                width: parent.width
                title: I18n.tr("CONVERT & INSTALL")

                Row {
                    width: parent.width
                    spacing: Tokens.s2 * root.s
                    Btn {
                        width: (parent.width - parent.spacing) / 2
                        text: I18n.tr("Compress video…")
                        onAct: root.openPicker("compress")
                    }
                    Btn {
                        width: (parent.width - parent.spacing) / 2
                        text: I18n.tr("Install app…")
                        primary: true
                        onAct: root.openPicker("install")
                    }
                }
            }
        }
    }

    Cards.FilePickerOverlay {
        anchors.fill: parent
        s: root.s
        open: root.pickerOpen
        mode: root.pickerMode
        onCancelled: root.pickerOpen = false
        onConfirmed: paths => {
            if (root.pickerMode === "install")
                Stash.install(paths);
            else
                Stash.compress(paths);
            root.pickerOpen = false;
        }
    }

    Cards.CobaltSetupOverlay {
        anchors.fill: parent
        s: root.s
        open: root.setupOpen
        onClosed: root.setupOpen = false
    }
}
