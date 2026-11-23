import QtQuick
import "../kit" as K
import "lib/rows.js" as Rows

// A file or a recent file: an image shows itself on a plate, anything else its
// file symbol; then the name, the path, and Open / Show in folder.
Column {
    id: root

    property var lead: null
    readonly property var row: root.lead ? root.lead.row : null
    readonly property var engine: root.lead ? root.lead.engine : null

    // find rows carry their preview url, recent rows keep their uri in the id
    readonly property string imageUrl: {
        const row = root.row;
        if (!row)
            return "";
        if (String(row.preview || "").length > 0)
            return String(row.preview);
        const id = String(row.id || "");
        if (row.providerId === "recent" && id.indexOf("recent:file://") === 0
                && /\.(png|jpe?g|gif|webp|bmp|svg|avif|tiff?)$/i.test(id))
            return id.slice("recent:".length);
        return "";
    }
    readonly property bool showImage: root.imageUrl.length > 0 && picture.status !== Image.Error

    spacing: K.Style.s3

    K.Shape {
        visible: root.showImage
        width: root.width
        height: Math.round(Math.min(root.width * 0.62, 220))
        cutSize: K.Look.cutLg
        fill: K.Style.tint5
        strokeWidth: K.Look.lithic || K.Look.fluent ? K.Style.border : 0
        strokeColor: K.Look.lithic ? K.Style.line : K.Style.lineSoft

        Image {
            id: picture
            anchors.fill: parent
            anchors.margins: K.Style.s2
            anchors.topMargin: K.Style.s2 + (root.lead ? root.lead.rise : 0)
            source: root.imageUrl
            sourceSize: Qt.size(480, 480)
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            smooth: true
            mipmap: true
        }
    }

    IconPlate {
        visible: !root.showImage
        glyph: root.row ? Rows.glyphFor(root.row) : "description"
        numeral: !root.engine || root.engine.selectedIndex < 0 ? ""
            : Rows.indexLabel(root.engine.selectedIndex, true,
                Rows.indexDigits(root.engine.resultCount, true))
        rise: root.lead ? root.lead.rise : 0
    }

    Column {
        width: root.width
        spacing: K.Style.s1

        K.Label {
            width: parent.width
            text: root.row ? String(root.row.title || "") : ""
            role: "headline"
            wrapMode: Text.Wrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }

        K.Label {
            width: parent.width
            visible: text.length > 0
            text: root.row ? String(root.row.subtitle || "") : ""
            role: "caption"
            tone: "muted"
            elide: Text.ElideMiddle
        }
    }

    ActionList {
        width: root.width
        items: root.lead ? root.lead.actionItems : []
        focusIndex: root.lead ? root.lead.actionFocus : -1
        scroller: root.lead ? root.lead.scroller : null
        onActivated: index => root.lead.runAt(index)
    }
}
