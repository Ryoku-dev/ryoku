import QtQuick
import QtQuick.Layouts
import Quickshell
import stage
import stage.services
import stage.modules.common
import stage.modules.common.widgets
import stage.modules.common.functions

/**
 * Edit Mode's chrome: the toolbar above the shrunk desktop.
 *
 * Everything is placed off `card`, the rectangle the desktop is drawn at -
 * the same arithmetic the desktop's own transform is built out of - so the
 * chrome cannot be a pixel off the thing it frames. That also gives the motion
 * for free and gives it the right shape: `card` is a function of the mode's
 * progress, so the toolbar rises out of the top band exactly as fast as the
 * desktop shrinks away from it. Nothing here carries a Behavior of its own, and
 * must not: a Behavior whose target moves every frame restarts every frame
 * and never ticks.
 *
 * The toolbar owns the display target and catalogue controls. The catalogue
 * opens on the right, into room the geometry has already taken out of the
 * desktop's width.
 */
Item {
    id: root

    // The desktop's rectangle on screen. Defaults to the whole item, so an
    // unconnected instance parks its chrome off the edge rather than somewhere
    // arbitrary.
    property rect card: Qt.rect(0, 0, root.width, root.height)
    // The screen minus what the bar and the dock occupy, interpolated on the
    // same progress as `card`. The toolbar clamps itself to this area and to
    // the card's top edge.
    property rect area: Qt.rect(0, 0, root.width, root.height)
    // The toolbar belongs to the card, not to the empty band above it.  Keep a
    // small clearance from the usable edge and from the drawer when a narrow
    // monitor leaves very little room for the chrome.
    readonly property real toolbarGap: Math.max(4, Math.min(
        Appearance.sizes.editModeEdgeMargin,
        Appearance.rounding.small))
    readonly property real toolbarLeftLimit: root.area.x + root.toolbarGap
    readonly property real toolbarRightLimit: Math.max(root.toolbarLeftLimit,
        (root.drawer.width > 0 ? root.drawer.x : root.area.x + root.area.width)
        - root.toolbarGap)
    readonly property real toolbarAvailableWidth: Math.max(1,
        root.toolbarRightLimit - root.toolbarLeftLimit)
    readonly property var visibleExtraSections: Config.extraSections.filter(
        section => section.hidden !== true)
    // Keep labels at their real size. When the drawer narrows a 1280px screen,
    // low-priority labels fold to icons and stay named by tooltips.
    readonly property bool compactToolbar: root.toolbarAvailableWidth < 1040
        || root.visibleExtraSections.length > 2
    readonly property real toolbarScale: toolbar.implicitWidth > 0
        ? Math.min(1, root.toolbarAvailableWidth / toolbar.implicitWidth) : 1
    readonly property real toolbarVisualWidth: toolbar.implicitWidth * root.toolbarScale
    readonly property real toolbarVisualHeight: toolbar.implicitHeight * root.toolbarScale
    readonly property real toolbarX: Math.max(root.toolbarLeftLimit,
        Math.min(root.toolbarRightLimit - root.toolbarVisualWidth,
            root.card.x + (root.card.width - root.toolbarVisualWidth) / 2))
    readonly property real toolbarTopLimit: root.area.y + root.toolbarGap
    readonly property real toolbarBottomLimit: Math.max(root.toolbarTopLimit,
        root.area.y + root.area.height - root.toolbarVisualHeight - root.toolbarGap)
    readonly property real toolbarYFromCard: root.card.y
        - root.toolbarVisualHeight - root.toolbarGap
    readonly property real toolbarY: Math.max(root.toolbarTopLimit,
        Math.min(root.toolbarBottomLimit, root.toolbarYFromCard))

    signal doneRequested()
    signal undoRequested()
    signal redoRequested()
    signal tabRequested(string tab)
    signal snapToggleRequested()
    signal drawerToggleRequested()
    // The toolbar's second way into the panel: the bar's own quick settings,
    // which are a page of the catalogue rather than a panel of their own.
    signal drawerPageRequested(string section, string page)
    // The drawer's gestures, relayed: the surface owns the geometry and every write.
    signal drawerAddRequested(string widgetId, real dropX, real dropY)
    signal drawerAddWidgetRequested(string widgetId)
    signal drawerBarPlaceRequested(string componentId, string bucket)
    signal drawerBarRemoveRequested(string componentId)
    signal drawerBarDragMoved(string componentId, real x, real y)
    signal drawerBarDropRequested(string componentId, real x, real y)
    signal drawerBarDragCancelled()
    signal drawerAddAppRequested(string appId, real dropX, real dropY)
    signal drawerToggleAppRequested(string appId)
    signal drawerAddAppPairRequested(string firstAppId, string secondAppId, string name)
    signal drawerAddFolderRequested(string folderName, var appsList)
    signal drawerClearHomeScreenAppsRequested()
    signal drawerLockLayoutResetRequested()
    // "widgets", "bar", "dock" or "lockIslands": that surface back to the
    // shell's defaults, as one history entry.
    signal drawerResetRequested(string what)

    // The drawer's reveal, from the same geometry the card is: its width is
    // the drawer's on the drawer's scalar, so the panel slides out of the
    // card's right edge exactly as fast as the desktop makes room for it.
    property rect drawer: Qt.rect(root.width, 0, 0, 0)
    readonly property alias drawerItem: drawerReveal
    // Whether the catalogue's search field holds the keyboard: the surface
    // takes focus for it and gives it straight back.
    readonly property bool drawerSearchFocused: drawerLoader.item ? drawerLoader.item.searchFocused : false
    property string drawerScreenName: ""

    // Published for the surface's input mask: the only pixels of a
    // screen-sized layer surface that may take a click.
    // The layer-shell Region must receive an untransformed item.  The visual
    // toolbar may be scaled on a narrow monitor, but handing that transformed
    // item to Region can make the overlay claim a much larger area and swallow
    // the bar's drag surface.
    readonly property alias toolbarItem: toolbarFrame
    // The guide's card is the other one, while a guided session is on.
    readonly property alias guideItem: guide.cardItem

    // The toolbar's rectangle, for a surface that is not this one. The
    // Welcome's collapsed pill parks beside the toolbar and has no other way
    // to know where it ended up.
    readonly property rect toolbarRect: Qt.rect(root.toolbarX, root.toolbarY,
        root.toolbarVisualWidth, root.toolbarVisualHeight)
    // The chrome only exists on the screen the mode is on, so there is never a
    // second one racing this write.
    onToolbarRectChanged: GlobalStates.editToolbarRect = root.toolbarRect

    // ── The toolbar's cascade ────────────────────────────────────────────────
    // The pill rises out of the band as one piece (its `y` is a function of
    // `card`), and its contents used to arrive with it in a single flat frame:
    // eight controls appearing at once, which reads as a screenshot rather than
    // as a toolbar being handed to you.
    //
    // Derived from `editProgress` rather than animated here, and that is the
    // whole trick. That scalar already carries the mode's clock - one Behavior,
    // in GlobalStates - so a piece's own reveal is arithmetic on it: the piece
    // at slot N waits `N * staggerStep` of the entry and then takes
    // `revealSpan` to arrive. Nothing here holds a timer, nothing restarts, and
    // the cascade plays BACKWARDS on the way out for free, because the same
    // scalar runs back down to zero. An animation of its own could do none of
    // that, and a declarative one whose target moves every frame restarts
    // every frame and never ticks at all (b710ef731).
    readonly property real staggerStep: 0.06
    readonly property real revealSpan: 0.5

    function slotReveal(slot) {
        const t = (GlobalStates.editProgress - slot * root.staggerStep) / root.revealSpan;
        return Math.max(0, Math.min(1, t));
    }

    // Scale rather than an offset: a `y` translate would take the control
    // outside the pill it is arriving into, and the pill does not clip.
    function slotScale(slot) {
        return 0.72 + 0.28 * root.slotReveal(slot);
    }

    // The toolbar's own body, claiming the cursor for the whole of it. Without
    // it the gaps between the buttons - the toolbar's padding and the rules -
    // set no cursor at all, so whatever the last surface asked for stays up
    // and the hand the buttons DO ask for reads as intermittent rather than as
    // the pointer answering the button. `Qt.NoButton` keeps it out of the way
    // of every real click, and it sits under the toolbar so each button's own
    // hand still wins over it.
    MouseArea {
        x: root.toolbarX
        y: root.toolbarY
        width: root.toolbarVisualWidth
        height: root.toolbarVisualHeight
        z: -1
        acceptedButtons: Qt.NoButton
        hoverEnabled: true
        cursorShape: Qt.ArrowCursor
    }

    Item {
        id: toolbarFrame
        x: root.toolbarX
        y: root.toolbarY
        width: root.toolbarVisualWidth
        height: root.toolbarVisualHeight

        Toolbar {
            id: toolbar
            // Stay attached to the card's top edge.  The old placement used
            // a fraction of the whole top band, which left a large dead gap
            // on tall monitors and made the toolbar appear centred between
            // the bar and the wallpaper instead of belonging to the wallpaper.
            x: 0
            y: 0
            width: implicitWidth
            height: implicitHeight
            scale: root.toolbarScale
            transformOrigin: Item.TopLeft
            spacing: Appearance.sizes.space2


            // The output name replaces the old one-item Desktop tab. With
            // several outputs it cycles the edit target; with one it remains
            // a stable label for the canvas being framed.
            IconAndTextToolbarButton {
                id: monitorButton
                readonly property var screens: Quickshell.screens
                opacity: root.slotReveal(0)
                scale: (monitorButton.down ? 0.96 : 1) * root.slotScale(0)
                Layout.alignment: Qt.AlignVCenter
                iconText: "monitor"
                text: GlobalStates.editModeMonitor !== ""
                    ? GlobalStates.editModeMonitor
                    : (monitorButton.screens.length > 0 ? monitorButton.screens[0].name : Translation.tr("Display"))
                compact: root.compactToolbar && monitorButton.screens.length > 1
                maximumLabelWidth: 120
                pointingHandCursor: monitorButton.screens.length > 1
                onClicked: {
                    const names = Array.from(monitorButton.screens).map(screen => screen.name);
                    if (names.length < 2)
                        return;
                    const at = names.indexOf(GlobalStates.editModeMonitor);
                    GlobalStates.switchEditMonitor(names[(at + 1) % names.length]);
                }

                StyledToolTip {
                    requireOverlay: false
                    text: monitorButton.screens.length > 1
                        ? Translation.tr("Edit the next display")
                        : Translation.tr("Editing %1").arg(monitorButton.text)
                }
            }

            Rectangle {
            opacity: root.slotReveal(0)
            scale: root.slotScale(0)
            Layout.alignment: Qt.AlignVCenter
            Layout.leftMargin: Appearance.sizes.space1
            Layout.rightMargin: Appearance.sizes.space1
            implicitWidth: 1
            implicitHeight: Appearance.sizes.space5
            color: Appearance.colors.colOutline
        }

        // The catalogue choices stay one semantic group. On a constrained
        // screen the same controls fold to icons instead of scaling their type
        // below the legibility floor; every folded chip remains named on hover.
            Rectangle {
            id: sectionGroup
            opacity: root.slotReveal(1)
            scale: root.slotScale(1)
            Layout.alignment: Qt.AlignVCenter
            implicitWidth: sectionRow.implicitWidth
            implicitHeight: Appearance.sizes.controlHeight
            radius: Appearance.rounding.small
            color: "transparent"

            component SectionChip: IconAndTextToolbarButton {
                id: chip
                required property string section
                property string page: ""
                property string tooltip: ""
                readonly property bool open: GlobalStates.editDrawerOpen && GlobalStates.editDrawerSection === chip.section

                Layout.fillHeight: false
                compact: root.compactToolbar
                maximumLabelWidth: 96
                toggled: chip.open
                onClicked: {
                    if (chip.open) {
                        root.drawerToggleRequested();
                        return;
                    }
                    root.drawerPageRequested(chip.section, chip.page);
                }

                StyledToolTip {
                    requireOverlay: false
                    text: chip.tooltip !== "" ? chip.tooltip : chip.text
                }
            }

            Row {
                id: sectionRow
                anchors.centerIn: parent
                spacing: Appearance.sizes.space1

                SectionChip {
                    section: "widgets"
                    iconText: "widgets"
                    text: Translation.tr("Widgets")
                    tooltip: Translation.tr("Desktop widgets")
                }
                // The wallpaper on its own: which picture each screen shows,
                // which one the colours come from, and how the desktop card
                // sits while framing.
                SectionChip {
                    section: "wallpaper"
                    iconText: "wallpaper"
                    text: Translation.tr("Wallpaper")
                    tooltip: GlobalStates.editLockPreview
                        ? Translation.tr("Lock screen wallpaper")
                        : Translation.tr("Wallpaper, position and screens")
                }
                SectionChip {
                    section: "style"
                    iconText: "palette"
                    text: Translation.tr("Style")
                    tooltip: Translation.tr("Presets, theme and colours")
                }
                SectionChip {
                    section: "dock"
                    iconText: "dock"
                    text: Translation.tr("Dock")
                    tooltip: Translation.tr("Dock design, apps and behaviour")
                }
                // Whatever the provider folds in beside them (Config.extraSections).
                Repeater {
                    model: root.visibleExtraSections
                    delegate: SectionChip {
                        required property var modelData
                        section: modelData.section
                        iconText: modelData.icon
                        text: modelData.label
                        tooltip: modelData.tooltip
                    }
                }
            }
        }

        // Edge snapping, drawn as state. It reads and toggles the key Settings
        // already offers rather than a switch of its own. Icon-only: the
        // toolbar's width is the card's inset and the labels beside it already
        // spend the words.
            IconToolbarButton {
            id: snapButton
            opacity: root.slotReveal(2)
            scale: (snapButton.down ? 0.96 : 1) * root.slotScale(2)
            Layout.alignment: Qt.AlignVCenter
            // The guides ARE the feature - the dot lattice and the alignment
            // lines a dragged widget latches onto. The alignment glyph this
            // used to carry says "align these to the left", which is a
            // different thing and the wrong promise.
            text: Config.widgetSnapEnabled ? "grid_guides" : "grid_off"
            toggled: Config.widgetSnapEnabled
            onClicked: root.snapToggleRequested()

            StyledToolTip {
                requireOverlay: false
                text: Config.widgetSnapEnabled
                    ? Translation.tr("Edge snapping on")
                    : Translation.tr("Edge snapping off")
            }
        }

            Rectangle {
            opacity: root.slotReveal(3)
            scale: root.slotScale(3)
            Layout.alignment: Qt.AlignVCenter
            Layout.leftMargin: Appearance.sizes.space1
            Layout.rightMargin: Appearance.sizes.space1
            implicitWidth: 1
            implicitHeight: Appearance.sizes.space5
            color: Appearance.colors.colOutline
        }

        // The two the keyboard already offers, for a pointer that never
        // reaches for it. Disabled rather than hidden when their stack is
        // empty: a button that comes and goes moves every other button on the
        // toolbar with it, and the toolbar is centred on the card, so the whole
        // row would slide under the pointer on the first edit.
            IconToolbarButton {
            id: undoButton
            // RippleButton dims a disabled button through this same property,
            // and an outer binding replaces its rule rather than joining it -
            // so the dimming is multiplied back in by hand.
            opacity: root.slotReveal(4) * (undoButton.enabled ? 1 : 0.4)
            scale: (undoButton.down ? 0.96 : 1) * root.slotScale(4)
            Layout.alignment: Qt.AlignVCenter
            text: "undo"
            enabled: GlobalStates.editCanUndo
            onClicked: root.undoRequested()

            StyledToolTip {
                requireOverlay: false
                text: Translation.tr("Undo (Ctrl+Z)")
            }
        }

            IconToolbarButton {
            id: redoButton
            opacity: root.slotReveal(5) * (redoButton.enabled ? 1 : 0.4)
            scale: (redoButton.down ? 0.96 : 1) * root.slotScale(5)
            Layout.alignment: Qt.AlignVCenter
            text: "redo"
            enabled: GlobalStates.editCanRedo
            onClicked: root.redoRequested()

            StyledToolTip {
                requireOverlay: false
                text: Translation.tr("Redo (Ctrl+Shift+Z)")
            }
        }

        // The labelled exit is the permanent primary action.
            IconAndTextToolbarButton {
            id: doneButton
            opacity: root.slotReveal(6)
            scale: (doneButton.down ? 0.96 : 1) * root.slotScale(6)
            Layout.alignment: Qt.AlignVCenter
            iconText: "done"
            text: Translation.tr("Done")
            colBackground: Appearance.colors.colSecondary
            colBackgroundHover: Appearance.colors.colSecondaryHover
            colBackgroundActive: Appearance.colors.colSecondaryActive
            colRipple: Appearance.colors.colOnSecondaryContainer
            colText: Appearance.colors.colOnSecondary
            borderColor: Appearance.colors.colSecondary
            onClicked: root.doneRequested()
        }
        }
    }

    // The tour of the toolbar, shown only while a guide is hosting the session.
    // Above the toolbar in declaration order so its card is never behind the
    //control it points at.
    EditModeGuide {
        id: guide
        anchors.fill: parent
        z: 150
        tabsTarget: monitorButton
        sectionsTarget: sectionGroup
        historyTarget: undoButton
        doneTarget: doneButton
        drawerTarget: drawerReveal
    }

    // Clicking anywhere but the field lets the keyboard go. Only this surface
    // needs the catcher: a click on the desktop lands on another surface, which
    // takes the keyboard with it and deactivates this window on its own. The
    // press is declined rather than consumed, so whatever was actually clicked
    // still gets it.
    MouseArea {
        anchors.fill: parent
        z: 200
        enabled: root.drawerSearchFocused
        acceptedButtons: Qt.AllButtons
        onPressed: mouse => {
            if (drawerLoader.item && !drawerLoader.item.pointInSearchField(mouse.x, mouse.y))
                drawerLoader.item.releaseSearchFocus();
            mouse.accepted = false;
        }
    }

    // The reveal: a clip the width of the drawer's animated scalar, with the
    // full-width panel anchored to its left edge, so the panel slides in from
    // the card's right edge and its contents never reflow.
    Item {
        id: drawerReveal
        x: root.drawer.x
        y: root.drawer.y
        width: root.drawer.width
        height: root.drawer.height
        clip: true
        visible: width > 0

        Loader {
            id: drawerLoader
            anchors.fill: parent
            active: GlobalStates.editDrawerOpen || GlobalStates.editDrawerProgress > 0
            sourceComponent: EditModeDrawer {
                anchors.fill: parent
                ghostParent: root
                screenName: root.drawerScreenName
                onAddRequested: (widgetId, dropX, dropY) => root.drawerAddRequested(widgetId, dropX, dropY)
                onLockLayoutResetRequested: root.drawerLockLayoutResetRequested()
                onResetRequested: what => root.drawerResetRequested(what)
                onAddInstanceRequested: widgetId => root.drawerAddWidgetRequested(widgetId)
                onBarPlaceRequested: (componentId, bucket) => root.drawerBarPlaceRequested(componentId, bucket)
                onBarRemoveRequested: componentId => root.drawerBarRemoveRequested(componentId)
                onBarDragMoved: (componentId, x, y) => root.drawerBarDragMoved(componentId, x, y)
                onBarDropRequested: (componentId, x, y) => root.drawerBarDropRequested(componentId, x, y)
                onBarDragCancelled: root.drawerBarDragCancelled()
                onAddAppRequested: (appId, dropX, dropY) => root.drawerAddAppRequested(appId, dropX, dropY)
                onToggleAppOnHomeScreenRequested: appId => root.drawerToggleAppRequested(appId)
                onAddAppPairRequested: (firstAppId, secondAppId, name) => root.drawerAddAppPairRequested(firstAppId, secondAppId, name)
                onAddFolderRequested: (folderName, appsList) => root.drawerAddFolderRequested(folderName, appsList)
                onClearHomeScreenAppsRequested: root.drawerClearHomeScreenAppsRequested()
            }
        }
    }
}
