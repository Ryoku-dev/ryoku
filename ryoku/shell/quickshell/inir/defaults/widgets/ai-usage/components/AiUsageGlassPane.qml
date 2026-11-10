pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Widgets
import inir
import inir.services
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.common.functions
import inir.modules.iris.style
import inir.modules.iris.components

ClippingRectangle {
    required property var page
    id: pane
    radius: page.radius
    // A transparent ClippingRectangle stops painting its children (see
    // IrisIsland's chassis); this is the same near-transparent body clip.
    color: IrisStyle.bodyClip
    readonly property var screen: pane.QsWindow.window?.screen ?? page.targetScreen
    readonly property point at: {
        // mapToItem does not track its ancestors as binding dependencies, so
        // a layout that moves the page (opening, a card growing, a new
        // workspace) left this point at the one it had before the move: the
        // glass sampled the wallpaper near the screen's top edge and read as
        // a dark card until some unrelated resize re-evaluated it. Read every
        // ancestor's position to keep the binding honest.
        let walk = pane
        while (walk) {
            void (walk.x + walk.y)
            walk = walk.parent
        }
        void (pane.width + pane.height)
        return pane.mapToItem(null, 0, 0)
    }

    Image {
        id: wallpaper
        x: -pane.at.x
        y: -pane.at.y
        width: pane.screen?.width ?? 1920
        height: pane.screen?.height ?? 1080
        source: WallpaperListener.wallpaperUrlForScreen(pane.screen)
        fillMode: Image.PreserveAspectCrop
        cache: true
        asynchronous: true
        sourceSize.width: width
        sourceSize.height: height
        visible: status === Image.Ready
        layer.enabled: visible
        layer.effect: MultiEffect {
            source: wallpaper
            anchors.fill: source
            blurEnabled: true
            blurMax: 64
            blur: 1
            saturation: 0.08
        }
    }

    Rectangle {
        anchors.fill: parent
        color: ColorUtils.applyAlpha(IrisStyle.surfaceOpaque, 0.38)
    }
}
