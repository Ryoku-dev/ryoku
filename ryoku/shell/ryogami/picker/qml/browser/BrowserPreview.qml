import QtQuick
import QtMultimedia
import Ryoku.Ui.Singletons

Rectangle {
    id: preview

    property var item: null
    property string provider: ""
    property string fullPath: ""
    property bool showApply: false
    property bool applying: false
    property int maxMinutes: 3
    property var sources: null
    property bool cancellable: false
    property string clipStart: I18n.tr("0:00")
    property string clipLen: I18n.tr("3:00")
    property bool shown: false
    property real reveal: shown ? 1 : 0

    signal closeRequested()
    signal save()
    signal apply()
    signal cancel()
    signal copyId(string id)

    visible: reveal > 0.001
    opacity: reveal
    focus: shown
    color: Theme.withAlpha(Theme.surface, 0.99)
    border.width: 1
    border.color: Theme.withAlpha(Theme.outline, 0.54)
    Behavior on reveal { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutCubic } }

    readonly property bool _isYoutube: provider === "youtube"
    readonly property real _duration: item && item.durationSecs ? Number(item.durationSecs) : 0
    readonly property bool _downloaded: !!item && item.downloaded === true
    readonly property bool _downloading: !!sources && sources.isDownloading(item)
    readonly property bool _hasClip: _isYoutube && _duration > 0
    readonly property string _artSource: Library.fileUrl(fullPath.length > 0 ? fullPath
        : (item && item.thumb ? String(item.thumb) : ""))
    readonly property string _clipUrl: {
        var url = item && item.fullUrl ? String(item.fullUrl) : ""
        var path = url.split(/[?#]/)[0].toLowerCase()
        return /\.(webm|mp4|mkv|mov)$/.test(path) ? url : ""
    }
    function _aspectFor(value) {
        if (!value)
            return 16 / 9
        var resolution = value.resolution ? String(value.resolution) : ""
        var match = resolution.match(/(\d+)\s*[x\u00d7]\s*(\d+)/i)
        if (match && Number(match[1]) > 0 && Number(match[2]) > 0)
            return Number(match[1]) / Number(match[2])
        var imageW = Number(value.width || value.imageWidth || 0)
        var imageH = Number(value.height || value.imageHeight || 0)
        return imageW > 0 && imageH > 0 ? imageW / imageH : 16 / 9
    }
    readonly property real _aspect: _aspectFor(item)
    readonly property real _sourceAspect: heroImage.status === Image.Ready && heroImage.sourceSize.height > 0
        ? heroImage.sourceSize.width / heroImage.sourceSize.height : _aspect

    property string _clipFor: ""
    onItemChanged: {
        var id = preview.item ? String(preview.item.id) : ""
        if (id === preview._clipFor)
            return
        preview._clipFor = id
        if (preview._hasClip) {
            preview.clipStart = I18n.tr("0:00")
            preview.clipLen = preview._fmtClock(Math.min(preview._duration, preview.maxMinutes * 60))
        }
    }

    function _fmtClock(secs) {
        secs = Math.max(0, Math.floor(secs))
        var hours = Math.floor(secs / 3600)
        var minutes = Math.floor((secs % 3600) / 60)
        var seconds = secs % 60
        var ss = seconds < 10 ? "0" + seconds : "" + seconds
        if (hours > 0)
            return hours + ":" + (minutes < 10 ? "0" + minutes : "" + minutes) + ":" + ss
        return minutes + ":" + ss
    }
    function _parseClock(text) {
        var parts = String(text).split(":")
        var value = 0
        for (var i = 0; i < parts.length; ++i) {
            var n = parseInt(parts[i], 10)
            value = value * 60 + (isNaN(n) ? 0 : n)
        }
        return value
    }
    function _fmtSize(value) {
        if (value === undefined || value === null || value === "") return ""
        if (typeof value === "string") return I18n.tr(value)
        var bytes = Number(value)
        if (isNaN(bytes) || bytes < 1024) return ""
        if (bytes >= 1024 * 1024) return I18n.tr("%1 MB").arg((bytes / (1024 * 1024)).toFixed(1))
        return I18n.tr("%1 KB").arg(Math.round(bytes / 1024))
    }
    readonly property int clipStartSecs: _parseClock(clipStart)
    readonly property int clipLenSecs: _parseClock(clipLen)

    Column {
        anchors.fill: parent
        spacing: 0

        Item {
            width: parent.width
            height: 54 * Theme.scale

            Column {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                anchors.leftMargin: 16 * Theme.scale
                width: parent.width - 66 * Theme.scale
                spacing: 1 * Theme.scale
                Text {
                    width: parent.width
                    text: preview.item && preview.item.title
                        ? I18n.tr(String(preview.item.title))
                        : (preview.item ? I18n.tr(String(preview.item.id)) : "")
                    elide: Text.ElideRight
                    color: Theme.surfaceText
                    font.family: Theme.display
                    font.pixelSize: Theme.fontHead
                    renderType: Text.NativeRendering
                }
                Text {
                    text: I18n.tr("DETAIL")
                    color: Theme.withAlpha(Theme.surfaceText, 0.44)
                    font.family: Theme.sans
                    font.pixelSize: Theme.fontTiny
                    font.letterSpacing: 1.4
                    renderType: Text.NativeRendering
                }
            }

            Rectangle {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                anchors.rightMargin: 11 * Theme.scale
                width: 32 * Theme.scale
                height: 32 * Theme.scale
                radius: Theme.radius
                color: closeHover.hovered ? Theme.withAlpha(Theme.surfaceText, 0.1) : "transparent"
                Text {
                    anchors.centerIn: parent
                    text: "\u{f0156}"
                    color: Theme.surfaceText
                    font.family: Theme.icon
                    font.pixelSize: Theme.fs(18)
                    renderType: Text.NativeRendering
                }
                HoverHandler { id: closeHover; cursorShape: Qt.PointingHandCursor }
                TapHandler { onTapped: preview.closeRequested() }
            }
            Rectangle {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                height: 1
                color: Theme.withAlpha(Theme.outline, 0.3)
            }
        }

        Flickable {
            id: details
            width: parent.width
            height: parent.height - y
            clip: true
            contentWidth: width
            contentHeight: detailColumn.implicitHeight + 28 * Theme.scale
            boundsBehavior: Flickable.StopAtBounds

            Column {
                id: detailColumn
                width: details.width - 28 * Theme.scale
                x: 14 * Theme.scale
                y: 14 * Theme.scale
                spacing: 12 * Theme.scale

                Rectangle {
                    width: parent.width
                    height: Math.min(width / Math.max(preview._sourceAspect, 0.1), details.height * 0.55)
                    radius: Theme.radius
                    color: Theme.withAlpha(Theme.background, 0.76)
                    border.width: 1
                    border.color: Theme.withAlpha(Theme.outline, 0.34)
                    clip: true

                    Image {
                        id: heroImage
                        anchors.fill: parent
                        visible: preview._artSource.length > 0
                        source: preview._artSource
                        fillMode: Image.PreserveAspectFit
                        asynchronous: true
                        cache: false
                    }
                    Loader {
                        anchors.fill: parent
                        active: preview.shown && preview._clipUrl.length > 0
                        sourceComponent: Video {
                            source: preview._clipUrl
                            fillMode: VideoOutput.PreserveAspectFit
                            loops: MediaPlayer.Infinite
                            muted: true
                            Component.onCompleted: play()
                        }
                    }
                    BrowserSpinner {
                        anchors.centerIn: parent
                        visible: preview._artSource.length === 0 && preview._clipUrl.length === 0
                        size: 48
                        color: Theme.surfaceText
                    }
                    Rectangle {
                        anchors.right: parent.right
                        anchors.top: parent.top
                        anchors.margins: 7 * Theme.scale
                        visible: preview._clipUrl.length > 0
                        width: liveText.implicitWidth + 12 * Theme.scale
                        height: 19 * Theme.scale
                        radius: Theme.radius
                        color: Theme.withAlpha(Theme.surface, 0.88)
                        Text {
                            id: liveText
                            anchors.centerIn: parent
                            text: I18n.tr("Live")
                            color: Theme.surfaceText
                            font.family: Theme.sans
                            font.weight: Font.DemiBold
                            font.pixelSize: Theme.fontTiny
                            renderType: Text.NativeRendering
                        }
                    }
                }

                Flow {
                    width: parent.width
                    spacing: 7 * Theme.scale
                    Repeater {
                        model: [
                            preview.item && preview.item.resolution ? I18n.tr(String(preview.item.resolution)) : "",
                            preview.item && preview.item.fileSize ? preview._fmtSize(preview.item.fileSize) : "",
                            preview.item && preview.item.purity ? I18n.tr(String(preview.item.purity)) : "",
                            preview.item && preview.item.category ? I18n.tr(String(preview.item.category)) : ""
                        ].filter(function(value) { return value.length > 0 })
                        delegate: Rectangle {
                            required property string modelData
                            width: tagText.implicitWidth + 16 * Theme.scale
                            height: 25 * Theme.scale
                            radius: Theme.radius
                            color: Theme.withAlpha(Theme.surfaceText, 0.05)
                            border.width: 1
                            border.color: Theme.withAlpha(Theme.outline, 0.32)
                            Text {
                                id: tagText
                                anchors.centerIn: parent
                                text: I18n.tr(modelData)
                                color: Theme.withAlpha(Theme.surfaceText, 0.7)
                                font.family: Theme.sans
                                font.pixelSize: Theme.fontSmall
                                renderType: Text.NativeRendering
                            }
                        }
                    }
                }


                Rectangle {
                    width: parent.width
                    height: 36 * Theme.scale
                    radius: Theme.radius
                    color: Theme.withAlpha(Theme.surfaceText, 0.04)
                    border.width: 1
                    border.color: Theme.withAlpha(Theme.outline, 0.32)
                    Row {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.leftMargin: 10 * Theme.scale
                        anchors.rightMargin: 10 * Theme.scale
                        spacing: 8 * Theme.scale
                        Text {
                            text: I18n.tr("ID")
                            color: Theme.withAlpha(Theme.surfaceText, 0.38)
                            font.family: Theme.sans
                            font.weight: Font.DemiBold
                            font.pixelSize: Theme.fontTiny
                            renderType: Text.NativeRendering
                        }
                        Text {
                            width: parent.width - 28 * Theme.scale
                            text: preview.item ? I18n.tr(String(preview.item.id)) : ""
                            elide: Text.ElideMiddle
                            color: Theme.withAlpha(Theme.surfaceText, 0.68)
                            font.family: Theme.sans
                            font.pixelSize: Theme.fontSmall
                            renderType: Text.NativeRendering
                        }
                    }
                }

                Flow {
                    width: parent.width
                    spacing: 7 * Theme.scale

                    FolioAction {
                        label: preview._downloaded
                            ? I18n.tr("Saved")
                            : (preview._downloading
                               ? (preview.cancellable ? I18n.tr("Cancel") : I18n.tr("Saving"))
                               : I18n.tr("Save"))
                        active: !preview._downloaded
                        enabled: !preview._downloaded && (!preview._downloading || preview.cancellable)
                        onTriggered: preview._downloading ? preview.cancel() : preview.save()
                    }
                    FolioAction {
                        visible: preview.showApply
                        label: preview.applying ? I18n.tr("Applying") : I18n.tr("Apply")
                        active: true
                        enabled: !preview.applying
                        onTriggered: preview.apply()
                    }
                    FolioAction {
                        label: I18n.tr("Copy id")
                        enabled: !!preview.item
                        onTriggered: if (preview.item) preview.copyId(String(preview.item.id))
                    }
                }

                Rectangle {
                    visible: preview._hasClip
                    width: parent.width
                    height: clipColumn.implicitHeight + 20 * Theme.scale
                    radius: Theme.radius
                    color: Theme.withAlpha(Theme.surfaceText, 0.04)
                    border.width: 1
                    border.color: Theme.withAlpha(Theme.outline, 0.32)
                    Column {
                        id: clipColumn
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.leftMargin: 11 * Theme.scale
                        anchors.rightMargin: 11 * Theme.scale
                        spacing: 7 * Theme.scale
                        Text {
                            text: I18n.tr("Clip range")
                            color: Theme.surfaceText
                            font.family: Theme.sans
                            font.weight: Font.Medium
                            font.pixelSize: Theme.fontBody
                            renderType: Text.NativeRendering
                        }
                        Row {
                            spacing: 6 * Theme.scale
                            BrowserClockField {
                                placeholder: I18n.tr("0:00")
                                text: preview.clipStart
                                onEdited: preview.clipStart = value
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("for")
                                color: Theme.withAlpha(Theme.surfaceText, 0.54)
                                font.family: Theme.sans
                                font.pixelSize: Theme.fontSmall
                                renderType: Text.NativeRendering
                            }
                            BrowserClockField {
                                placeholder: I18n.tr("3:00")
                                text: preview.clipLen
                                onEdited: preview.clipLen = value
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("of %1").arg(preview._fmtClock(preview._duration))
                                color: Theme.withAlpha(Theme.surfaceText, 0.54)
                                font.family: Theme.sans
                                font.pixelSize: Theme.fontSmall
                                renderType: Text.NativeRendering
                            }
                        }
                    }
                }
                Text {
                    width: parent.width
                    visible: !!preview.item && !!preview.item.attribution
                    text: preview.item ? I18n.tr(String(preview.item.attribution)) : ""
                    wrapMode: Text.WordWrap
                    color: Theme.withAlpha(Theme.surfaceText, 0.62)
                    font.family: Theme.sans
                    font.pixelSize: Theme.fontBody
                    renderType: Text.NativeRendering
                }
            }
        }

    }

    Keys.onEscapePressed: function(event) {
        preview.closeRequested()
        event.accepted = true
    }
}
