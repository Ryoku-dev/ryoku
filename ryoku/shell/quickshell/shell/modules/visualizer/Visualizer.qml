//@ pragma DefaultEnv QSG_RENDER_LOOP = threaded
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import "Singletons"

// desktop audio visualiser module entry. click-through, palette-tinted cava
// spectrum across the bottom of one monitor. mode drives visibility and layer:
// "off" hides it, "desktop" draws on the wallpaper behind every window
// (WlrLayer.Bottom), "overlay" raises it over windows (WlrLayer.Top). the
// controller instantiates one per screen and binds `screen`/`mode`; cava only
// runs while enabled.
Item {
    id: root

    // monitor this instance draws on, supplied by the controller.
    property var screen

    // off | desktop | overlay. defaults from Config so a standalone instance
    // still honours the persisted enabled flag; the controller overrides it.
    property string mode: Config.enabled ? "desktop" : "off"

    // Placement: the look becomes draggable and sizable on the desktop, on its
    // own surface (Placer), and rides the top layer so it is not buried while
    // being aimed.
    property bool placing: false
    property bool suppressed: false
    signal placingDone

    readonly property bool active: root.mode !== "off"
    readonly property bool placeable: root.placing && root.active
    readonly property bool raised: root.mode === "overlay" || root.placeable

    // Keep the shared spectrum running while this look is being aimed, even under
    // Power Saver or silence, so it stays visible to place; released when done.
    onPlaceableChanged: Spectrum.placementHolds += root.placeable ? 1 : -1
    Component.onDestruction: if (root.placeable) Spectrum.placementHolds -= 1

    // cava runs whenever the visualiser is enabled: gating on "audio playing"
    // needs a probe that is either broken or costs a periodic graph dump, while
    // cava is ~1% idle and the render already freezes on silence.
    Binding {
        target: Spectrum
        property: "active"
        value: root.active || root.suppressed
    }

    // one shared cava for every instance, at the largest band count any of them
    // wants; each Motion resamples down. changing it restarts cava.
    Binding {
        target: Spectrum
        property: "bars"
        value: Config.maxBars
    }

    // cava's framerate follows the render ceiling: no point sampling faster than
    // the spectrum draws.
    Binding {
        target: Spectrum
        property: "fps"
        value: Config.fps
    }

    // the scope look draws the actual playback waveform; capture the monitor only
    // while some instance is the line look and the visualiser is on.
    Binding {
        target: Waveform
        property: "active"
        value: root.active && Config.anyLine
    }

    PanelWindow {
        id: win

        screen: root.screen
        visible: root.active && !root.suppressed
        color: "transparent"

        // The curtain hangs off the bar, so its surface honours the bar's
        // exclusive zone and starts where the bar ends. Every other look owns
        // the whole screen and ignores reservations.
        exclusionMode: Config.styleId === "curtain" ? ExclusionMode.Normal : ExclusionMode.Ignore
        WlrLayershell.layer: root.raised ? WlrLayer.Top : WlrLayer.Bottom
        WlrLayershell.namespace: "ryoku-visualizer"
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.None

        // empty input region: every click falls through to windows above, so the
        // visualiser shares the desktop without ever intercepting it. Placement
        // runs on its own surface (Placer) rather than lifting this one, since a
        // surface masked click-through does not start taking a pointer again just
        // because the region is swapped.
        mask: emptyRegion
        Region { id: emptyRegion }

        anchors { top: true; left: true; right: true; bottom: true }

        // One view per instance: the primary plus every extra, each painting its
        // own look on the shared surface.
        Item {
            anchors.fill: parent
            Repeater {
                id: rep
                model: Config.count
                delegate: VisualizerView {
                    id: vizView
                    required property int index
                    anchors.fill: parent
                    cfg: VizItem { data: Config.dataAt(vizView.index) }
                    // The first read of activeView happens while the Repeater is
                    // still empty; delegate creation changes neither count nor
                    // the active index, so this is the only signal that re-fires
                    // the binding once the views exist.
                    Component.onCompleted: root.viewsReady++
                }
            }
        }
    }

    // The active view, for the placement overlay to frame and colour-match.
    // `viewsReady` is bumped by each delegate once it exists: the first read of
    // this binding happens while the Repeater is still empty, and delegate
    // creation changes neither `rep.count` nor `Config.active`, so without that
    // signal the binding latches null and the Placer never loads.
    property int viewsReady: 0
    readonly property Item activeView: {
        root.viewsReady;   // rebuild the binding when delegates appear
        rep.count;         // rebuild the binding when the delegates change
        return rep.itemAt(Config.active);
    }

    // The placement overlay only exists while a look is being aimed; it tunes the
    // active instance.
    Loader {
        active: root.placeable && root.activeView !== null
        sourceComponent: Placer {
            screen: root.screen
            box: root.activeView.boxRect
            guide: root.activeView.guide
            onDone: root.placingDone()
        }
    }
}
