pragma ComponentBehavior: Bound
import QtQuick
import inir.modules.background.widgets
import inir.modules.background.widgets.imageConverter
import inir.modules.background.widgets.japaneseTypography
import inir.modules.background.widgets.visualizer

// Visible host for one vendored iRiS CANVAS widget (the ones with no irisFace:
// customImage, editorial, imageConverter, japaneseTypography,
// visualizer). Unlike the faces, the whole upstream widget is rendered here,
// unmodified, at the slot's origin -- Ryoku's WidgetSlot owns placement, lock,
// persistence and (in Ryoku style) the backing and geometry around it. The
// widget's own x/y are pinned to 0 when ryokuHosted so it draws where the slot
// puts it, while ryokuScreenX/Y keep its real on-screen position for any
// wallpaper-crop surface. Ink/accent overrides only bite where the widget reads
// the iRiS accent/ink tokens; otherwise the Ryoku skin is just the slot backing.
Item {
    id: prov

    property string faceId: "editorial"
    property int screenW: 1920
    property int screenH: 1080
    property string outputName: ""
    property real hostX: 0
    property real hostY: 0
    property bool ryokuStyle: false
    property color inkOverride: "transparent"
    property color accentOverride: "transparent"
    property real scaleOverride: -1
    property real radiusOverride: -1
    // Resolved slot corner radius (the WidgetSlot's card rounding), so the Ryoku
    // custom-image surface can clip its media to the same rounded square.
    property real slotRadius: -1
    property var optionOverrides: ({})

    readonly property var widget: host.item
    implicitWidth: host.item ? host.item.implicitWidth : 240
    implicitHeight: host.item ? host.item.implicitHeight : 160

    function _componentFor(id: string): Component {
        switch (id) {
        case "customImage": return cCustom;
        case "editorial": return cEditorial;
        case "imageConverter": return cImage;
        case "japaneseTypography": return cJp;
        case "visualizer": return cViz;
        }
        return null;
    }

    Loader {
        id: host
        anchors.fill: parent
        sourceComponent: prov._componentFor(prov.faceId)
    }

    Component { id: cCustom; CustomImageWidget { anchors.fill: parent; configEntryName: "customImage"; visible: true; ryokuHosted: true; ryokuStyle: prov.ryokuStyle; outputName: prov.outputName; screenWidth: prov.screenW; screenHeight: prov.screenH; scaledScreenWidth: prov.screenW; scaledScreenHeight: prov.screenH; wallpaperScale: 1; ryokuHostX: prov.hostX; ryokuHostY: prov.hostY; ryokuInkOverride: prov.inkOverride; ryokuAccentOverride: prov.accentOverride; ryokuScaleOverride: prov.scaleOverride; ryokuRadiusOverride: prov.radiusOverride; ryokuSlotRadius: prov.slotRadius; ryokuOptionOverrides: prov.optionOverrides } }
    Component { id: cEditorial; EditorialWidget { anchors.fill: parent; configEntryName: "editorial"; visible: true; ryokuHosted: true; ryokuStyle: prov.ryokuStyle; outputName: prov.outputName; screenWidth: prov.screenW; screenHeight: prov.screenH; scaledScreenWidth: prov.screenW; scaledScreenHeight: prov.screenH; wallpaperScale: 1; ryokuHostX: prov.hostX; ryokuHostY: prov.hostY; ryokuInkOverride: prov.inkOverride; ryokuAccentOverride: prov.accentOverride; ryokuScaleOverride: prov.scaleOverride; ryokuRadiusOverride: prov.radiusOverride; ryokuOptionOverrides: prov.optionOverrides } }
    Component { id: cImage; ImageConverterWidget { anchors.fill: parent; configEntryName: "imageConverter"; visible: true; ryokuHosted: true; ryokuStyle: prov.ryokuStyle; outputName: prov.outputName; screenWidth: prov.screenW; screenHeight: prov.screenH; scaledScreenWidth: prov.screenW; scaledScreenHeight: prov.screenH; wallpaperScale: 1; ryokuHostX: prov.hostX; ryokuHostY: prov.hostY; ryokuInkOverride: prov.inkOverride; ryokuAccentOverride: prov.accentOverride; ryokuScaleOverride: prov.scaleOverride; ryokuRadiusOverride: prov.radiusOverride; ryokuOptionOverrides: prov.optionOverrides } }
    Component { id: cJp; JapaneseTypographyWidget { anchors.fill: parent; configEntryName: "japaneseTypography"; visible: true; ryokuHosted: true; ryokuStyle: prov.ryokuStyle; outputName: prov.outputName; screenWidth: prov.screenW; screenHeight: prov.screenH; scaledScreenWidth: prov.screenW; scaledScreenHeight: prov.screenH; wallpaperScale: 1; ryokuHostX: prov.hostX; ryokuHostY: prov.hostY; ryokuInkOverride: prov.inkOverride; ryokuAccentOverride: prov.accentOverride; ryokuScaleOverride: prov.scaleOverride; ryokuRadiusOverride: prov.radiusOverride; ryokuOptionOverrides: prov.optionOverrides } }
    Component { id: cViz; VisualizerWidget { anchors.fill: parent; configEntryName: "visualizer"; visible: true; ryokuHosted: true; ryokuStyle: prov.ryokuStyle; outputName: prov.outputName; screenWidth: prov.screenW; screenHeight: prov.screenH; scaledScreenWidth: prov.screenW; scaledScreenHeight: prov.screenH; wallpaperScale: 1; ryokuHostX: prov.hostX; ryokuHostY: prov.hostY; ryokuInkOverride: prov.inkOverride; ryokuAccentOverride: prov.accentOverride; ryokuScaleOverride: prov.scaleOverride; ryokuRadiusOverride: prov.radiusOverride; ryokuSlotRadius: prov.slotRadius; ryokuOptionOverrides: prov.optionOverrides } }
}
