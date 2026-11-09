import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property string sizeClass: "expanded"
    property int currentIndex: 0
    property var destinations: [
        { label: "Fleet", glyph: "dns" },
        { label: "Activity", glyph: "history" },
        { label: "Settings", glyph: "tune" }
    ]
    property var newMenuItems: [
        { label: "Server", glyph: "dns" },
        { label: "Console", glyph: "terminal" },
        { label: "Instant machine", glyph: "bolt" }
    ]
    property Item overlayParent: null
    property string previewState: ""
    property bool previewNewOpen: false
    readonly property bool compact: sizeClass === "compact"
    readonly property bool expanded: sizeClass === "expanded"
    readonly property real railWidth: expanded ? LithicTokens.size.railExpanded
                                                : LithicTokens.size.railMedium
    readonly property real tileSize: expanded ? LithicTokens.size.railDestinationExpanded
                                               : LithicTokens.size.railDestinationMedium
    readonly property real startY: expanded ? LithicTokens.size.railStartExpanded
                                             : LithicTokens.size.railStartMedium
    readonly property real newY: expanded ? LithicTokens.size.railNewYExpanded
                                           : LithicTokens.size.railNewYMedium

    signal homeTriggered()
    signal destinationTriggered(int index, string label)
    signal newItemTriggered(int index, string label)

    implicitWidth: compact ? LithicTokens.size.windowMinWidth : railWidth
    implicitHeight: compact ? LithicTokens.size.railBottomHeight
                            : LithicTokens.size.windowMinHeight
    padding: 0
    Accessible.role: Accessible.Pane

    function destinationY(index) {
        if (index === 0)
            return startY
        if (index === 1)
            return newY + tileSize + LithicTokens.size.railTileGap
        return height - LithicTokens.size.railItemBottomInset - tileSize
    }

    function compactColumn(index) {
        return index === 0 ? 1 : index + 2
    }

    function toggleNewMenu() {
        if (newPopover.opened)
            newPopover.close()
        else
            newPopover.open()
    }

    component DestinationButton: T.AbstractButton {
        required property string destinationLabel
        required property string glyphName
        required property bool selected
        property string previewState: ""
        property bool showLabel: true

        hoverEnabled: true
        activeFocusOnTab: true
        Accessible.name: destinationLabel
        Accessible.role: Accessible.Button

        background: CutRect {
            fill: selected ? "transparent"
                           : parent.hovered || parent.previewState === "hover"
                             ? Lithic.stone3 : "transparent"
            cut: LithicTokens.cut.mid
            ChiselColorBehavior on fill {
                duration: parent.hovered || parent.previewState === "hover"
                          ? LithicTokens.motion.hoverIn
                          : LithicTokens.motion.hoverOut
            }

            FocusRing {
                x: -outset
                y: -outset
                width: parent.width + outset * 2
                height: parent.height + outset * 2
                keyboardFocusVisible: parent.parent.visualFocus
                                      || parent.parent.previewState === "focus"
                cut: LithicTokens.cut.mid
            }
        }

        contentItem: Column {
            spacing: showLabel ? LithicTokens.size.railItemLabelGap : 0

            Glyph {
                anchors.horizontalCenter: parent.horizontalCenter
                name: glyphName
                size: LithicTokens.size.railDestinationIcon
                ink: selected ? Lithic.onSel : Lithic.fg2
            }
            Label {
                visible: showLabel
                anchors.horizontalCenter: parent.horizontalCenter
                text: destinationLabel
                typeRole: "caption"
                ink: selected ? Lithic.onSel : Lithic.fg2
            }
        }
    }

    background: Rectangle { color: Lithic.stone1 }

    contentItem: Item {
        Item {
            id: verticalRail
            anchors.fill: parent
            visible: !root.compact

            CutRect {
                id: verticalSelection
                x: (parent.width - root.tileSize) / 2
                y: root.destinationY(Math.max(0, Math.min(root.currentIndex,
                                                          root.destinations.length - 1)))
                width: root.tileSize
                height: root.tileSize
                fill: Lithic.sel
                cut: LithicTokens.cut.mid
                ChiselNumberBehavior on y { duration: LithicTokens.motion.select }
            }

            BrandMark {
                id: homeMark
                x: (parent.width - width) / 2
                y: LithicTokens.size.railHomeTop
                   + ((root.expanded ? LithicTokens.size.railHomeExpanded
                                     : LithicTokens.size.railHomeMedium)
                      - LithicTokens.size.brandMark) / 2
                width: LithicTokens.size.brandMark
                height: width
                markSize: width
                previewState: root.previewState
                onClicked: root.homeTriggered()
            }

            Repeater {
                model: root.destinations
                delegate: DestinationButton {
                    required property var modelData
                    required property int index
                    x: (verticalRail.width - width) / 2
                    y: root.destinationY(index)
                    width: root.tileSize
                    height: root.tileSize
                    destinationLabel: modelData.label
                    glyphName: modelData.glyph
                    selected: root.currentIndex === index
                    showLabel: root.expanded
                    previewState: root.previewState
                    onClicked: {
                        root.currentIndex = index
                        root.destinationTriggered(index, destinationLabel)
                        newPopover.close()
                    }
                }
            }

            NewPlaque {
                id: verticalNew
                x: (parent.width - width) / 2
                y: root.newY
                visible: !root.compact
                menuOpen: root.previewNewOpen || newPopover.opened
                previewState: root.previewState
                onClicked: root.toggleNewMenu()
            }
        }

        Item {
            id: bottomRail
            anchors.fill: parent
            visible: root.compact
            readonly property real columnWidth: width / LithicTokens.size.railBottomColumns

            CutRect {
                x: root.compactColumn(Math.max(0, Math.min(root.currentIndex,
                                                           root.destinations.length - 1)))
                   * bottomRail.columnWidth
                y: (parent.height - LithicTokens.size.railDestinationCompact) / 2
                width: bottomRail.columnWidth
                height: LithicTokens.size.railDestinationCompact
                fill: Lithic.sel
                cut: LithicTokens.cut.small
                ChiselNumberBehavior on x { duration: LithicTokens.motion.select }
            }

            BrandMark {
                id: compactHomeMark
                x: (bottomRail.columnWidth - width) / 2
                y: (parent.height - height - LithicTokens.size.railItemLabelGap
                    - Lithic.type.caption.lineHeight) / 2
                width: LithicTokens.size.brandMarkCompact
                height: width
                markSize: width
                previewState: root.previewState
                onClicked: root.homeTriggered()
            }
            Label {
                anchors.horizontalCenter: compactHomeMark.horizontalCenter
                y: compactHomeMark.y + compactHomeMark.height
                   + LithicTokens.size.railItemLabelGap
                text: "Home"
                typeRole: "caption"
                inkRole: "fg2"
            }

            Repeater {
                model: root.destinations
                delegate: DestinationButton {
                    required property var modelData
                    required property int index
                    x: root.compactColumn(index) * bottomRail.columnWidth
                    y: (bottomRail.height - height) / 2
                    width: bottomRail.columnWidth
                    height: LithicTokens.size.railDestinationCompact
                    destinationLabel: modelData.label
                    glyphName: modelData.glyph
                    selected: root.currentIndex === index
                    showLabel: true
                    previewState: root.previewState
                    onClicked: {
                        root.currentIndex = index
                        root.destinationTriggered(index, destinationLabel)
                        newPopover.close()
                    }
                }
            }

            NewPlaque {
                id: compactNew
                x: bottomRail.columnWidth * 2
                   + (bottomRail.columnWidth - width) / 2
                y: (parent.height - height) / 2
                compact: true
                menuOpen: root.previewNewOpen || newPopover.opened
                previewState: root.previewState
                onClicked: root.toggleNewMenu()
            }
        }
    }

    Popover {
        id: newPopover
        parent: root.overlayParent ? root.overlayParent : root.parent
        anchorItem: root.compact ? compactNew : verticalNew
        opener: anchorItem
        previewEmbedded: root.previewNewOpen
        placementSide: "right"

        Menu {
            items: root.newMenuItems
            onTriggered: index => {
                const entry = root.newMenuItems[index]
                root.newItemTriggered(index, entry.label || "")
                newPopover.close()
            }
        }
    }

}
