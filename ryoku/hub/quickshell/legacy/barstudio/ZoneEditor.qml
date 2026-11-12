pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "BarStudioModel.js" as Model

// The selected rail's three zones, each a clear titled section: its widgets as
// a numbered, reorderable list (move up, move down, remove), and a per-zone
// drawer that opens the palette of widgets that fit this rail and are not
// already on it. A widget lands in the zone whose drawer you opened; there is no
// cross-zone move (a misplaced widget is a remove and an add). Controls use a
// MouseArea with preventStealing, so a tap always lands even inside the page's
// scroll view (the old compact TapHandler buttons lost taps to the Flickable).
Column {
    id: root
    required property var config
    required property string edge
    required property var catalog
    signal staged(var next)

    readonly property bool horizontal: root.edge === "top" || root.edge === "bottom"
    readonly property var zoneIds: root.horizontal ? ["start", "center", "end"] : ["top", "center", "bottom"]

    // the zone whose add-drawer is open ("" = none); a rail change closes it
    property string openZone: ""
    onEdgeChanged: root.openZone = ""

    // widgets that fit this rail's axis and are not already anywhere on the rail
    function available() {
        const axis = root.horizontal ? "horizontal" : "vertical";
        const onRail = Model.railWidgets(root.config, root.edge);
        const out = [];
        for (const id of root.catalog.ids()) {
            const entry = root.catalog.entry(id);
            if (entry && entry.axes.includes(axis) && onRail.indexOf(id) < 0) out.push(id);
        }
        return out;
    }

    spacing: Tokens.s4
    CatalogLabels { id: labels }

    // A clickable whose tap always lands inside the page scroll: a plain
    // MouseArea with preventStealing, not a TapHandler the Flickable can grab.
    // Carries a glyph (square icon) or a text label (a chip), and an armed and
    // active state.
    component Tap: Rectangle {
        id: tp
        property string glyph: ""
        property string label: ""
        property bool armed: true
        property bool active: false
        signal act()

        implicitWidth: tp.label !== "" ? tlab.implicitWidth + 20 : 28
        implicitHeight: 28
        radius: Tokens.radius
        opacity: tp.armed ? 1 : 0.3
        color: tp.active ? Tokens.bone : (ma.containsMouse && tp.armed ? Tokens.tint10 : "transparent")
        border.width: Tokens.border
        border.color: tp.active ? Tokens.bone : (ma.containsMouse && tp.armed ? Tokens.lineStrong : Tokens.line)
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

        Text {
            id: tlab
            anchors.centerIn: parent
            text: tp.glyph !== "" ? tp.glyph : I18n.tr(tp.label)
            color: tp.active ? Tokens.inkOnBone : Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: tp.glyph !== "" ? 14 : 10
            font.weight: Font.Medium
            font.letterSpacing: tp.label !== "" ? Tokens.trackLabel : 0
        }
        MouseArea {
            id: ma
            anchors.fill: parent
            enabled: tp.armed
            hoverEnabled: true
            preventStealing: true
            cursorShape: Qt.PointingHandCursor
            onClicked: tp.act()
        }
    }

    Repeater {
        model: root.zoneIds
        delegate: Column {
            id: zone
            required property string modelData
            readonly property var items: root.config.rails[root.edge][zone.modelData]
            readonly property bool open: root.openZone === zone.modelData
            width: root.width
            spacing: Tokens.s1

            // header: // ZONE_  count ........ [+ ADD]
            Item {
                width: parent.width
                height: 28
                Row {
                    id: zhead
                    spacing: Tokens.s2
                    anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                    Text { text: "//"; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; anchors.verticalCenter: parent.verticalCenter }
                    Text { text: labels.zone(zone.modelData).toUpperCase(); color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel; anchors.verticalCenter: parent.verticalCenter }
                    Text { visible: zone.items.length > 0; text: String(zone.items.length); color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; anchors.verticalCenter: parent.verticalCenter }
                }
                Rectangle {
                    anchors { left: zhead.right; right: addBtn.left; leftMargin: Tokens.s3; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                    height: 1
                    color: Tokens.lineSoft
                }
                Tap {
                    id: addBtn
                    objectName: "zone-add-" + zone.modelData
                    anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                    label: zone.open ? I18n.tr("CLOSE") : I18n.tr("+ ADD")
                    active: zone.open
                    armed: zone.open || root.available().length > 0
                    onAct: root.openZone = zone.open ? "" : zone.modelData
                }
            }

            // the widgets in this zone, numbered, each with move and remove
            Repeater {
                model: zone.items
                delegate: Rectangle {
                    id: wrow
                    required property string modelData
                    required property int index
                    objectName: "widget-" + root.edge + "-" + zone.modelData + "-" + wrow.index
                    width: zone.width
                    height: 42
                    radius: Tokens.radius
                    color: rh.hovered ? Tokens.tint5 : "transparent"
                    border.width: Tokens.border
                    border.color: rh.hovered ? Tokens.line : Tokens.lineSoft
                    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
                    HoverHandler { id: rh }

                    Row {
                        anchors { left: parent.left; leftMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                        spacing: Tokens.s3
                        Text {
                            text: String(wrow.index + 1)
                            color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Text {
                            text: labels.item(wrow.modelData)
                            color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                    Row {
                        anchors { right: parent.right; rightMargin: Tokens.s2; verticalCenter: parent.verticalCenter }
                        spacing: Tokens.s1
                        Tap {
                            objectName: "widget-up-" + wrow.index
                            glyph: "\u2191"
                            armed: wrow.index > 0
                            onAct: root.staged(Model.reorderZoneItem(root.config, root.edge, zone.modelData, wrow.index, wrow.index - 1))
                        }
                        Tap {
                            objectName: "widget-down-" + wrow.index
                            glyph: "\u2193"
                            armed: wrow.index < zone.items.length - 1
                            onAct: root.staged(Model.reorderZoneItem(root.config, root.edge, zone.modelData, wrow.index, wrow.index + 1))
                        }
                        Tap {
                            objectName: "widget-remove-" + wrow.index
                            glyph: "\u00d7"
                            onAct: root.staged(Model.removeZoneItem(root.config, root.edge, zone.modelData, wrow.index))
                        }
                    }
                }
            }

            // an empty zone reads as empty until you open its drawer
            Rectangle {
                visible: zone.items.length === 0 && !zone.open
                width: parent.width
                height: 32
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: Tokens.lineSoft
                Text {
                    anchors { left: parent.left; leftMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                    text: I18n.tr("// EMPTY")
                    color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny; font.letterSpacing: Tokens.trackLabel
                }
            }

            // the drawer: tap a widget to add it to this zone
            Rectangle {
                id: drawer
                visible: zone.open
                width: parent.width
                height: visible ? dcol.implicitHeight + Tokens.s3 * 2 : 0
                radius: Tokens.radius
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.line
                Column {
                    id: dcol
                    anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s3 }
                    spacing: Tokens.s2
                    Text {
                        width: parent.width
                        text: root.available().length > 0
                            ? I18n.tr("ADD TO %1").arg(labels.zone(zone.modelData).toUpperCase())
                            : I18n.tr("EVERY WIDGET THAT FITS IS ALREADY ON THIS RAIL")
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                        font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                        wrapMode: Text.WordWrap
                    }
                    Flow {
                        width: parent.width
                        spacing: Tokens.s1
                        visible: root.available().length > 0
                        Repeater {
                            model: root.available()
                            delegate: Tap {
                                required property string modelData
                                label: labels.item(modelData)
                                onAct: root.staged(Model.addZoneItem(root.config, root.edge, zone.modelData, modelData, root.catalog))
                            }
                        }
                    }
                }
            }
        }
    }
}
