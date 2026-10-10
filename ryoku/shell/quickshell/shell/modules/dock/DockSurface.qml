pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import shell.services
import stage

// The Ryoku dock: a first-class shell surface (namespace ryoku-dock), one per
// monitor and style-agnostic -- it no longer belongs to any bar. A frosted
// island row (DockBand) on the screen edge OPPOSITE the bar, revealed by a peek
// strip when autohide is on.
//
// Reveal is the point of this surface. The band shows when the dock is not
// autohiding, or the pointer is on it, or nothing is focused, or a pin is being
// dragged -- and is forced away under a fullscreen window unless the pointer is
// in the peek strip. Hiding slides the band out by its own depth, leaving a 3 px
// peek inside the input mask; the empty margins stay click-through because the
// mask is only the band rect unioned with that strip.
//
// While the Stage Editor frames this monitor the dock is the thing being
// edited, so it never steps aside: it holds revealed, rises above the lifted
// desktop, and publishes the edge it takes so the mode's viewport shrinks
// around it (stage's EditModeInsets).
PanelWindow {
    id: dock

    // ── edge: explicit, or auto (opposite the bar) ───────────────────────────
    readonly property string edge: Dock.resolvedEdge()
    readonly property bool horizontal: dock.edge === "top" || dock.edge === "bottom"

    // ── sizing ────────────────────────────────────────────────────────────────
    readonly property bool autohide: Dock.cfg("autohide", true)
    readonly property real edgeGap: 8
    readonly property real peek: 3
    readonly property real depth: band.baseSize
    // Inner clearance for a magnified icon to rise into, plus room for the hover
    // name label above it; both grow into the desktop, never off the screen edge.
    // Zero when neither is on, so a plain dock reserves nothing extra.
    readonly property real headroom: ((Dock.cfg("magnify", true) && !Perf.reduceMotion) ? 22 : 0)
        + (Dock.cfg("labels", true) ? 30 : 0)

    // The dock band's along-edge size and centre in screen coordinates (0 size
    // while the dock is off), so a neighbour -- the record island -- can park flush
    // beside the band instead of drifting into a corner. Uses the band's resting
    // geometry, independent of reveal, so an autohiding dock still defines a stable
    // lane. The band is centred on the edge, so the centre is the surface midpoint.
    readonly property real bandSize: dock.visible ? (dock.horizontal ? band.implicitWidth : band.implicitHeight) : 0
    readonly property real bandCenter: dock.horizontal ? (dock.width / 2) : (dock.height / 2)

    // Set by the host: the Stage Editor is framing this monitor.
    property bool stageEditing: false

    color: "transparent"
    visible: Dock.cfg("enabled", false)
    // `screen` is PanelWindow's own property, set per monitor from shell.qml.
    WlrLayershell.namespace: "ryoku-dock"
    // The editing desktop lifts to Top; the dock rides Overlay so its strip
    // stays clickable (the chrome's own mask never covers the dock's edge).
    WlrLayershell.layer: stageEditing ? WlrLayer.Overlay : WlrLayer.Bottom
    exclusionMode: ExclusionMode.Normal
    // A pinned (non-autohide) dock reserves its depth; an autohiding one floats
    // over the desktop and reserves nothing.
    exclusiveZone: (!dock.autohide && dock.visible) ? (dock.depth + dock.edgeGap) : 0

    anchors {
        left: dock.horizontal || dock.edge === "left"
        right: dock.horizontal || dock.edge === "right"
        top: !dock.horizontal || dock.edge === "top"
        bottom: !dock.horizontal || dock.edge === "bottom"
    }
    implicitHeight: dock.horizontal ? (dock.depth + dock.edgeGap + dock.headroom) : 0
    implicitWidth: dock.horizontal ? 0 : (dock.depth + dock.edgeGap + dock.headroom)

    // ── reveal state machine ──────────────────────────────────────────────────
    readonly property bool monFullscreen: Wm.outputHasFullscreen(dock.screen ? dock.screen.name : "")
    readonly property bool pointerInside: band.hovered || peekHover.hovered
    readonly property string screenName: dock.screen ? dock.screen.name : ""
    readonly property bool menuHere: Dock.menuOpen && Dock.menuScreen === dock.screenName
    readonly property bool revealed: {
        // The editor is showing this dock's settings page: it holds open.
        if (dock.stageEditing)
            return true;
        // A fullscreen window on this monitor forces the dock away; only a hover
        // into the peek strip brings it back over the fullscreen content.
        if (dock.monFullscreen)
            return dock.pointerInside;
        return !dock.autohide
            || dock.pointerInside
            || !Dock.anyFocused
            || band.dragging
            || dock.menuHere;
    }

    // The edge the mode's viewport must clear: the band's resting depth plus
    // its gap, never reveal or hover state (a viewport that moves mid-edit
    // rescales every widget under the cursor).
    readonly property real insetDepth: dock.depth + dock.edgeGap
    function publishInset() {
        GlobalStates.setDockInset(dock.screenName,
            dock.stageEditing && dock.visible ? dock.edge : "", dock.insetDepth);
    }
    onStageEditingChanged: dock.publishInset()
    onVisibleChanged: dock.publishInset()
    onEdgeChanged: dock.publishInset()
    onInsetDepthChanged: dock.publishInset()
    Component.onCompleted: dock.publishInset()
    Component.onDestruction: GlobalStates.setDockInset(dock.screenName, "", 0)

    // Perp offset (the axis pointing away from the screen edge) of the band's
    // resting outer corner, and the direction it slides to hide. The far edges
    // (bottom, right) keep headroom before the band; the near edges (top, left)
    // keep the gap.
    readonly property real slideDist: dock.depth + dock.edgeGap - dock.peek
    readonly property real perpRest: (dock.edge === "bottom" || dock.edge === "right") ? dock.headroom : dock.edgeGap
    readonly property int perpSign: (dock.edge === "bottom" || dock.edge === "right") ? 1 : -1
    readonly property real perpPos: dock.perpRest + (dock.revealed ? 0 : dock.perpSign * dock.slideDist)

        DockBand {
            id: band
            edge: dock.edge
            screenName: dock.screenName
            reservedDepth: dock.depth + dock.edgeGap

            // Centre along the band's own axis; slide along the perpendicular one.
            x: dock.horizontal ? Math.round((dock.width - band.implicitWidth) / 2) : dock.perpPos
            y: dock.horizontal ? dock.perpPos : Math.round((dock.height - band.implicitHeight) / 2)
            Behavior on x { enabled: !Perf.reduceMotion; NumberAnimation { duration: Motion.menuSlide; easing.type: Motion.menuSlideCurve } }
            Behavior on y { enabled: !Perf.reduceMotion; NumberAnimation { duration: Motion.menuSlide; easing.type: Motion.menuSlideCurve } }
        }

        // The peek strip: a thin always-masked band at the very screen edge, so a
        // hidden dock can still be revealed by pointer proximity. It tracks the band
        // along the axis but stays pinned to the edge across it.
        Item {
            id: peekStrip
            x: dock.horizontal ? band.x : (dock.edge === "left" ? 0 : dock.width - dock.edgeGap)
            y: dock.horizontal ? (dock.edge === "top" ? 0 : dock.height - dock.edgeGap) : band.y
            width: dock.horizontal ? band.width : dock.edgeGap
            height: dock.horizontal ? dock.edgeGap : band.height
            HoverHandler { id: peekHover }
        }
    // Input mask = band rect ∪ peek strip, so the empty margins and the magnify
    // headroom stay click-through.
    mask: Region {
        Region {
            x: Math.round(band.x)
            y: Math.round(band.y)
            width: Math.round(band.width)
            height: Math.round(band.height)
        }
        Region {
            x: Math.round(peekStrip.x)
            y: Math.round(peekStrip.y)
            width: Math.round(peekStrip.width)
            height: Math.round(peekStrip.height)
        }
    }
}
