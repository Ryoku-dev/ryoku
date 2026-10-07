import QtQuick
import Quickshell
import Quickshell.Wayland
import "../modules"
import "kit"
import "kit/Routes.js" as Routes
import Ryoku.Ui
import Ryoku.Ui.Singletons

// QS Bar Settings: the panel the bar's 力 logo opens. It is about one thing, the
// bar, in four routes down one rail: Bar (where it sits and how its surface
// reads), Layout (the three lanes you arrange the widgets in), Widgets (every
// widget as a row you show, size, colour and tune) and Dock (the app dock beside
// it). What this panel used to also carry -- logo, spaces, pickers, desktop
// widgets, the mid-work switches, the session -- already has a home, so it left.
//
// State reads and writes straight off `root` (the qsbar Theme) and the shell's
// own services, so persistence is untouched. The chrome is paper and ink from
// Ryoku.Ui; the bar's retinted colours appear only where they are data (the
// silhouette, the accent swatches, the lane chips, the marker preview).
PanelWindow {
    id: cc
    required property var root

    property var tokens: tk
    property string route: "bars"

    // `open(target)` keeps its old contract for the bar-logo click and the qsbar
    // IPC (`ipc call qsbar settings <route>`): a route id shows that route, an
    // empty/legacy target shows the first, and a retired route id (logo, spaces,
    // pickers, desktop, system, session, appearance) maps to its nearest new home
    // so an old caller never lands on nothing.
    function open(target) {
        var raw = (target === undefined || target === null) ? "" : String(target);
        cc.route = (raw === "" || raw === "quick" || raw === "configure")
            ? "bars" : Routes.resolve(raw);
        cc.root.controlVisible = true;
    }
    function close() { cc.root.controlVisible = false }

    // A route can ask to open another and carry an argument (Layout's SETTINGS
    // link lands Widgets on a chosen widget). The pending arg is read once by the
    // target page on load, then cleared.
    property var routeArg: null
    function go(id, arg) {
        cc.routeArg = (arg === undefined) ? null : arg;
        cc.route = Routes.byId(id) ? id : "bars";
    }

    readonly property var routeDef: Routes.byId(cc.route)
    function pageUrl() {
        var f = Routes.fileFor(cc.route);
        return f === "" ? Qt.resolvedUrl("routes/BarsRoute.qml") : Qt.resolvedUrl("routes/" + f + ".qml");
    }

    // Search index: one entry per route from the registry, plus the controls
    // worth naming, scoped to the four routes. Accepting an entry navigates to
    // its route.
    readonly property var searchEntries: cc.buildSearchIndex()
    function buildSearchIndex() {
        var out = [];
        for (var i = 0; i < Routes.ROUTES.length; i++) {
            var r = Routes.ROUTES[i];
            out.push({ id: r.id, name: I18n.tr(r.label), route: r.id, category: I18n.tr(r.label),
                       searchTags: String(r.keywords || "").split(/\s+/), description: I18n.tr(r.desc) });
        }
        return out.concat([
            { id: "bars.position", name: I18n.tr("Bar position"), route: "bars", category: I18n.tr("Bar"),
              searchTags: ["top", "bottom", "edge"], description: I18n.tr("Which edge the bar docks to.") },
            { id: "bars.form", name: I18n.tr("Bar form"), route: "bars", category: I18n.tr("Bar"),
              searchTags: ["full", "fit", "dock", "notch", "islands", "shape"], description: I18n.tr("The shell shape the bar takes.") },
            { id: "bars.surface", name: I18n.tr("Bar surface"), route: "bars", category: I18n.tr("Bar"),
              searchTags: ["border", "corners", "frost", "shadow", "depth", "tooltip"], description: I18n.tr("Border, corners, frost, shadow and tooltip border.") },
            { id: "bars.gaps", name: I18n.tr("Bar gaps"), route: "bars", category: I18n.tr("Bar"),
              searchTags: ["gap", "margin", "edge", "top", "bottom", "left", "right"], description: I18n.tr("How far the bar stays off each output edge.") },
            { id: "bars.accent", name: I18n.tr("Accent colour"), route: "bars", category: I18n.tr("Bar"),
              searchTags: ["colour", "color", "seal", "palette", "slot"], description: I18n.tr("Which palette slot the bar draws its accent from.") },
            { id: "bars.motion", name: I18n.tr("Gap animation"), route: "bars", category: I18n.tr("Bar"),
              searchTags: ["motion", "stream", "reactor", "animation"], description: I18n.tr("The stream that flows in the gaps between widgets.") },
            { id: "bars.scale", name: I18n.tr("Bar size"), route: "bars", category: I18n.tr("Bar"),
              searchTags: ["scale", "size", "height", "bigger"], description: I18n.tr("Scale the bar without changing display scaling.") },
            { id: "layout.arrange", name: I18n.tr("Arrange widgets"), route: "layout", category: I18n.tr("Layout"),
              searchTags: ["move", "reorder", "order", "left", "center", "right", "lane"], description: I18n.tr("Move widgets across the three lanes.") },
            { id: "layout.add", name: I18n.tr("Add a widget"), route: "layout", category: I18n.tr("Layout"),
              searchTags: ["add", "hidden", "plugin", "ryostore", "more"], description: I18n.tr("Add a hidden built-in or an installed plugin to the bar.") },
            { id: "layout.unlock", name: I18n.tr("Unlock the bar"), route: "layout", category: I18n.tr("Layout"),
              searchTags: ["unlock", "drag", "rearrange", "in place"], description: I18n.tr("Drag the widgets around on the bar itself.") },
            { id: "layout.reset", name: I18n.tr("Reset layout"), route: "layout", category: I18n.tr("Layout"),
              searchTags: ["reset", "restore", "default"], description: I18n.tr("Restore the shipped order and visibility.") },
            { id: "widgets.visibility", name: I18n.tr("Widget visibility"), route: "widgets", category: I18n.tr("Widgets"),
              searchTags: ["show", "hide", "on", "off"], description: I18n.tr("Which widgets the bar carries.") },
            { id: "widgets.density", name: I18n.tr("Widget density"), route: "widgets", category: I18n.tr("Widgets"),
              searchTags: ["density", "icon", "compact", "full"], description: I18n.tr("Draw a widget icon-only or in full.") },
            { id: "widgets.colour", name: I18n.tr("Per-widget colour"), route: "widgets", category: I18n.tr("Widgets"),
              searchTags: ["colour", "color", "tint", "fill", "frame"], description: I18n.tr("Give one widget its own accent.") },
            { id: "identity.launcher", name: I18n.tr("Launcher mark"), route: "identity", category: I18n.tr("Identity"),
              searchTags: ["launcher", "logo", "wordmark", "kanji", "glyph", "brand"], description: I18n.tr("The mark in the launcher pill.") },
            { id: "identity.workspaces", name: I18n.tr("Workspace marker"), route: "identity", category: I18n.tr("Identity"),
              searchTags: ["workspace", "spaces", "marker", "dots", "numbers", "kanji", "pacman", "aurora", "count"], description: I18n.tr("How many workspaces the bar shows and the marker each wears.") },
            { id: "widgets.ai", name: I18n.tr("AI usage tools"), route: "widgets", category: I18n.tr("Widgets"),
              searchTags: ["ai", "claude", "codex", "opencode", "usage"], description: I18n.tr("Which coding-agent meters the AI pill shows.") },
            { id: "dock.enabled", name: I18n.tr("Dock"), route: "dock", category: I18n.tr("Dock"),
              searchTags: ["dock", "apps", "pinned"], description: I18n.tr("The app dock on the opposite edge.") },
            { id: "dock.edge", name: I18n.tr("Dock edge"), route: "dock", category: I18n.tr("Dock"),
              searchTags: ["edge", "top", "bottom", "left", "right", "auto"], description: I18n.tr("Which edge the dock sits on.") },
            { id: "dock.autohide", name: I18n.tr("Dock auto-hide"), route: "dock", category: I18n.tr("Dock"),
              searchTags: ["hide", "peek", "reveal"], description: I18n.tr("Keep the dock as a peek strip until hovered.") },
            { id: "dock.pinned", name: I18n.tr("Pinned apps"), route: "dock", category: I18n.tr("Dock"),
              searchTags: ["pin", "pinned", "app", "add", "remove"], description: I18n.tr("The apps the dock always shows.") },
            { id: "community.installed", name: I18n.tr("Community widgets"), route: "community", category: I18n.tr("Community"),
              searchTags: ["plugin", "plugins", "installed", "third", "party", "remove", "uninstall"], description: I18n.tr("Bar widgets installed from outside Ryoku.") },
            { id: "community.add", name: I18n.tr("Add from git or Ryostore"), route: "community", category: I18n.tr("Community"),
              searchTags: ["git", "url", "ryostore", "store", "install", "add"], description: I18n.tr("Install a community bar widget.") }
        ]);
    }

    screen: cc.root.activePopupScreen
    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-control"
    WlrLayershell.keyboardFocus: cc.root.controlVisible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    property real reveal: cc.root.controlVisible ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: cc.root.controlVisible ? tk.revealOpen : tk.revealClose
            easing.type: Easing.OutCubic
        }
    }
    visible: reveal > 0.001
    onRevealChanged: if (reveal < 0.01) { cc.route = "bars"; cc.routeArg = null; searchOverlay.shown = false }

    CcTokens { id: tk; root: cc.root }

    readonly property int barH: cc.root.v2BarHeight
    readonly property int plateW: Math.min(tk.plateW, cc.width - 2 * tk.screenMargin)
    // The plate is as tall as it needs to be: the rail's natural height or the
    // page's, whichever is taller, capped by the screen and by tk.plateH. A short
    // route therefore yields a short panel rather than a screenful of empty
    // paper, and a long one scrolls inside the cap.
    readonly property int plateCap: Math.min(tk.plateH, cc.height - cc.barH - 2 * tk.screenMargin - tk.gap)
    readonly property int plateH: Math.max(Math.min(rail.implicitHeight, cc.plateCap),
        Math.min(cc.plateCap, tk.headH + tk.pad * 2 + tk.sectionGap + stage.pageHeight))

    // A click anywhere off the plate dismisses, the way every other popout on this
    // bar behaves. It sits below the plate, so the plate's own eater still wins.
    MouseArea {
        anchors.fill: parent
        enabled: cc.root.controlVisible
        acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
        onPressed: cc.close()
    }

    Rectangle {
        id: plate
        width: cc.plateW
        height: cc.plateH
        // Routes have different natural heights, so the plate resizes on every
        // switch. Snapping made the panel feel like a slideshow of dialogs; on the
        // house spatial curve it reads as one surface changing its mind. It only
        // animates once open, so revealing the panel never plays two motions.
        Behavior on height {
            enabled: cc.reveal > 0.99
            NumberAnimation {
                duration: Tokens.durDefaultSpatial
                easing.type: Easing.Bezier
                easing.bezierCurve: Tokens.curveDefaultSpatial
            }
        }
        radius: tk.corner
        color: Tokens.paper
        border.width: 1
        border.color: Tokens.line
        clip: true

        // it floats over the desktop, so it is the one surface here allowed a
        // shadow (docs/ui-ux.md: depth is a hairline, except when something
        // genuinely floats).
        PillShadow { theme: cc.root }

        x: Math.round(Math.max(tk.screenMargin,
            Math.min(cc.root.launcherBarX - tk.gap, cc.width - width - tk.screenMargin)))
        y: (cc.root.barPosition === "bottom" ? (cc.height - cc.barH - tk.gap - height) : (cc.barH + tk.gap))
           + (cc.root.barPosition === "bottom" ? tk.gap : -tk.gap) * (1 - cc.reveal)
        opacity: cc.reveal
        transformOrigin: cc.root.barPosition === "bottom" ? Item.Bottom : Item.Top
        focus: cc.root.controlVisible
        Keys.onPressed: function (e) {
            if (e.key === Qt.Key_Escape) {
                if (searchOverlay.shown) searchOverlay.shown = false;
                else cc.close();
                e.accepted = true;
            }
        }
        MouseArea { anchors.fill: parent; onClicked: {} }   // eat clicks so they don't dismiss

        Shortcut {
            sequence: "Ctrl+K"
            context: Qt.WindowShortcut
            enabled: cc.root.controlVisible
            onActivated: searchOverlay.shown = true
        }

        CcRail {
            id: rail
            anchors { top: parent.top; bottom: parent.bottom; left: parent.left }
            width: tk.railW
            root: cc.root
            tk: tk
            current: cc.route
            onChose: (id) => cc.route = id
        }

        Item {
            id: body
            anchors {
                top: parent.top; bottom: parent.bottom
                left: rail.right; right: parent.right
                topMargin: tk.pad; bottomMargin: tk.pad
                leftMargin: tk.pad; rightMargin: tk.pad
            }

            CcHead {
                id: head
                anchors { top: parent.top; left: parent.left; right: parent.right }
                root: cc.root
                tk: tk
                title: cc.routeDef ? I18n.tr(cc.routeDef.label) : ""
                gloss: cc.routeDef ? cc.routeDef.gloss : ""
                desc: cc.routeDef ? I18n.tr(cc.routeDef.desc) : ""
                index: Routes.indexOf(cc.route)
                onClosed: cc.close()
            }

            PageMotionStage {
                id: stage
                anchors {
                    top: head.bottom; topMargin: tk.sectionGap
                    left: parent.left; right: parent.right; bottom: parent.bottom
                }
                root: cc.root
                cc: cc
                outMs: tk.pageOut
                inMs: tk.pageIn
                pageUrl: cc.pageUrl()
            }
        }

        // A page longer than the plate is cut by the plate's clip, and a row sliced
        // in half reads as a broken layout rather than "there is more below". This
        // dissolves the cut into the paper, and only while there IS more below:
        // a page that fits, or one scrolled to its end, shows every row at full
        // ink. `clip` is rectangular and ignores the plate's radius, so the fade
        // carries the corner itself or it squares it.
        Rectangle {
            anchors { left: rail.right; right: parent.right; bottom: parent.bottom }
            height: tk.pad + tk.tailPad
            bottomRightRadius: tk.corner
            visible: opacity > 0.001
            opacity: stage.overflowBelow ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: tk.fade } }
            gradient: Gradient {
                GradientStop { position: 0.0; color: "transparent" }
                GradientStop { position: 0.6; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.8) }
                GradientStop { position: 1.0; color: Tokens.paper }
            }
        }

        // Search is a feature, not a permanent band across the top: it opens over
        // the body on Ctrl K, and closes on Escape.
        Item {
            id: searchOverlay
            property bool shown: false
            anchors.fill: body
            visible: opacity > 0.001
            opacity: searchOverlay.shown ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: tk.fade } }
            onShownChanged: if (searchOverlay.shown) search.focusInput()

            Rectangle {
                anchors.fill: parent
                color: Tokens.paper
                opacity: 0.96
            }
            CcSearch {
                id: search
                anchors { top: parent.top; left: parent.left; right: parent.right }
                root: cc.root
                tk: tk
                entries: cc.searchEntries
                onAccepted: (entry) => {
                    searchOverlay.shown = false;
                    cc.route = entry.route;
                }
                onDismissed: searchOverlay.shown = false
            }
        }

        // Instrument-panel corner ticks: the panel's frame chrome, the same
        // L-bracket vocabulary Decor's art panel and the reference sheet use. It
        // marks the plate as a registered surface. Anchored to the plate and inset
        // past its corner radius, so the frame reframes as the plate resizes on a
        // route change rather than sitting outside the motion.
        Item {
            anchors.fill: parent
            anchors.margins: tk.corner
            z: 10
            Ticks { color: Tokens.line; arm: 10 }
        }

    }
}
