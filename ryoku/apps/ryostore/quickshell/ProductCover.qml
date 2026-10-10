import QtQuick
import Ryoku.Ui.Singletons
import "lib/store.js" as StoreLogic

Item {
    id: cover

    required property var item
    property string mode: "cover"
    property bool selected: false
    property bool active: true
    property string artOverride: ""
    property bool zoomed: false
    property bool reducedMotion: false

    function identityColor() {
        const raw = String(item && (item.accent || item.surface) || "").trim().toLowerCase();
        if (/^#(?:[0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/.test(raw))
            return Qt.color(raw);
        if (raw !== "") {
            var hash = 0;
            for (var i = 0; i < raw.length; i++)
                hash = ((hash << 5) - hash + raw.charCodeAt(i)) | 0;
            return Qt.hsla(Math.abs(hash % 360) / 360, 0.52, 0.42, 1);
        }
        return Tokens.paperLift;
    }

    readonly property bool tile: mode === "cover"
    readonly property string coverArt: {
        const raw = String(item && item.artRaw || "");
        return raw.length > 0 ? raw : String(item && item.art || "");
    }
    readonly property bool hasArtwork: coverArt !== ""
    readonly property bool hasIdentity: Boolean(item && (item.id || item.name))
    readonly property string coverTitle: String(item && (item.name || item.id) || I18n.tr("Untitled"))
    readonly property color coverSurface: item && item.surface ? item.surface : Tokens.paperLift
    readonly property string coverInitials: {
        const explicit = String(item && item.initials || "").trim();
        if (explicit !== "")
            return explicit.slice(0, 3).toUpperCase();
        const words = coverTitle.trim().split(/\s+/);
        return (words.length > 1 ? words[0][0] + words[words.length - 1][0]
                                 : coverTitle.slice(0, 2)).toUpperCase();
    }
    readonly property color identitySurface: identityColor()
    readonly property real identityLuma: identitySurface.r * 0.299
            + identitySurface.g * 0.587 + identitySurface.b * 0.114
    readonly property color identityInk: Qt.hsla(0, 0, identityLuma > 0.56 ? 0.08 : 0.94, 1)
    readonly property var status: StoreLogic.statusLabels(item)
    readonly property string statusTag: status.length > 0 && status[0] !== "AVAILABLE" ? status[0] : ""
    readonly property bool unavailable: StoreLogic.isUnavailable(item)
    readonly property color frameSurface: mode === "plate" || mode === "view"
            ? Tokens.paper : coverSurface

    clip: true
    Accessible.role: Accessible.Graphic
    Accessible.ignored: !hasIdentity
    Accessible.name: [
        coverTitle,
        String(item && (item.categoryName || item.category) || ""),
        status.map(s => I18n.tr(s)).join(", ")
    ].filter(Boolean).join(", ")

    Rectangle {
        anchors.fill: parent
        color: cover.frameSurface
    }


    ProductMedia {
        id: productMedia
        anchors.fill: parent
        source: cover.artOverride !== "" ? cover.artOverride : cover.coverArt
        mode: cover.mode
        surface: cover.frameSurface
        active: cover.active
        fallbackText: cover.hasIdentity ? cover.coverInitials : ""
        fallbackSurface: cover.identitySurface
        fallbackInk: cover.identityInk
        scale: cover.tile && cover.zoomed ? 1.025 : 1

        Behavior on scale {
            enabled: !cover.reducedMotion
            NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease }
        }
    }

    Rectangle {
        anchors.fill: parent
        visible: cover.unavailable
        color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.68)
    }

    Rectangle {
        visible: cover.tile && cover.statusTag !== ""
        anchors { top: parent.top; left: parent.left; margins: Tokens.s2 }
        width: statusText.implicitWidth + Tokens.s3
        height: statusText.implicitHeight + Tokens.s2
        radius: Tokens.radius
        color: cover.statusTag === "UPDATE" ? Tokens.bone : Tokens.paper
        border.width: Tokens.border
        border.color: cover.statusTag === "UPDATE" ? Tokens.bone : Tokens.lineStrong

        Text {
            id: statusText
            anchors.centerIn: parent
            text: I18n.tr(cover.statusTag)
            color: cover.statusTag === "UPDATE" ? Tokens.inkOnBone : Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
        }
    }

    Rectangle {
        anchors.fill: parent
        visible: cover.tile
        color: "transparent"
        border.width: cover.selected ? Tokens.border * 2 : Tokens.border
        border.color: cover.selected ? Tokens.bone : Tokens.line
    }
}
