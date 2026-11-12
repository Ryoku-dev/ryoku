import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    property string sizeClass: "expanded"
    property int currentIndex: 0
    property var destinations: [
        { label: Lithic.tr("Fleet"), glyph: "dns" },
        { label: Lithic.tr("Activity"), glyph: "history" },
        { label: Lithic.tr("Settings"), glyph: "tune" }
    ]
    property var newMenuItems: [
        { label: Lithic.tr("Server"), glyph: "dns" },
        { label: Lithic.tr("Console"), glyph: "terminal" },
        { label: Lithic.tr("Instant machine"), glyph: "bolt" }
    ]
    property Item overlayParent: null
    property string previewState: ""
    property bool previewNewOpen: false
    property bool showNew: true
    property int footIndex: destinations.length - 1
    property int notchIndex: -1
    property Component brandMark: null
    readonly property bool compact: sizeClass === "compact"
    readonly property bool expanded: sizeClass === "expanded"
    readonly property real railWidth: expanded ? LithicTokens.size.railExpanded
                                                : LithicTokens.size.railMedium
    readonly property real tileSize: expanded ? LithicTokens.size.railDestinationExpanded
                                               : LithicTokens.size.railDestinationMedium
    readonly property real startY: expanded ? LithicTokens.size.railStartExpanded
                                             : LithicTokens.size.railStartMedium
    // expanded tiles share the widest label's width, from the tile size to the rail less s1 a side
    readonly property real tileWidth: {
        if (!expanded)
            return tileSize;
        // reading the font measures the width again once the caption face is set
        void captionMetrics.font;
        let widest = 0;
        for (let index = 0; index < destinations.length; ++index)
            widest = Math.max(widest, captionMetrics.advanceWidth(destinations[index].label || ""));
        return Math.max(tileSize, Math.min(railWidth - LithicTokens.space.s1 * 2,
                                           Math.ceil(widest) + LithicTokens.space.s1 * 2));
    }
    readonly property real _labelWidth: tileWidth - LithicTokens.space.s1 * 2
    // a label too wide for its tile wraps between words to a second line, and that tile grows
    // by one; a word too wide for the tile on its own is shortened with an ellipsis, never split
    property var _labels: []
    readonly property var _tileHeights: {
        const heights = [];
        for (let index = 0; index < destinations.length; ++index) {
            const label = _labels[index];
            const extra = expanded && label ? Math.min(2, label.lines) - 1 : 0;
            heights.push(tileSize + Math.max(0, extra) * Lithic.type.caption.lineHeight);
        }
        return heights;
    }

    // a short window gives the stacked tiles less room than they ask for: they then share
    // what there is, 4 apart, and drop their labels once a label no longer fits; the foot
    // group keeps its size
    readonly property int _stackCount: Math.max(0, Math.min(footIndex, destinations.length))
    readonly property real _stackRoom: {
        let y = height - LithicTokens.size.railItemBottomInset;
        for (let below = destinations.length - 1; below >= _stackCount; --below)
            y -= (_tileHeights[below] || tileSize) + LithicTokens.size.railTileGap;
        return y - startY - (showNew ? tileSize + LithicTokens.size.railTileGap : 0);
    }
    readonly property real _stackNeed: {
        let need = 0;
        for (let index = 0; index < _stackCount; ++index)
            need += (_tileHeights[index] || tileSize) + (index > 0 ? LithicTokens.size.railTileGap : 0);
        return need;
    }
    readonly property bool _squeezed: !compact && _stackCount > 0 && height > 0 && _stackNeed > _stackRoom
    readonly property real _stackGap: _squeezed ? LithicTokens.space.s1 : LithicTokens.size.railTileGap
    readonly property real _squeezedTile: Math.max(LithicTokens.size.iconPlaqueSmall,
                                                   Math.min(tileSize, Math.floor((_stackRoom + _stackGap) / Math.max(1, _stackCount)) - _stackGap))
    readonly property bool _labelsFit: !_squeezed || _squeezedTile >= LithicTokens.size.railDestinationCompact

    function labelText(index) {
        const label = _labels[index];
        return label ? label.text : (destinations[index] ? destinations[index].label || "" : "");
    }

    // Lays each label out at the tile's label width: a run with no break inside (a German
    // compound, unlike Japanese, which may break between characters) wider than the tile is
    // elided on its own; the rest wraps between words.
    function _layoutLabels() {
        // before completion the caption metrics are not set yet, and reading the tile width
        // then would keep a width measured in the default font
        if (!_completed)
            return;
        const result = [];
        labelProbe.width = _labelWidth;
        wordElider.elideWidth = _labelWidth;
        for (let index = 0; index < destinations.length; ++index) {
            const words = String(destinations[index].label || "").split(" ");
            for (let word = 0; word < words.length; ++word) {
                labelProbe.text = words[word];
                if (labelProbe.contentWidth > _labelWidth + 0.5) {
                    wordElider.text = words[word];
                    words[word] = wordElider.elidedText;
                }
            }
            const text = words.join(" ");
            labelProbe.text = text;
            result.push({ text: text, lines: Math.max(1, labelProbe.lineCount) });
        }
        labelProbe.text = "";
        _labels = result;
    }
    onDestinationsChanged: _layoutLabels()
    on_LabelWidthChanged: _layoutLabels()
    readonly property real newY: (expanded ? LithicTokens.size.railNewYExpanded
                                           : LithicTokens.size.railNewYMedium)
                                 + (destinations.length > 0 ? tileHeight(0) - tileSize : 0)
    // -1 means no destination is current (the brand mark is home); the tile reappears in place
    property int _shownIndex: -1
    property bool _glide: false
    readonly property int _selectionIndex: Math.max(0, Math.min(_shownIndex, destinations.length - 1))
    onCurrentIndexChanged: {
        _glide = _shownIndex >= 0 && currentIndex >= 0;
        _shownIndex = currentIndex;
    }
    property bool _completed: false
    Component.onCompleted: {
        _shownIndex = currentIndex
        _completed = true
        _layoutLabels()
    }

    signal homeTriggered()
    signal destinationTriggered(int index, string label)
    signal newItemTriggered(int index, string label)

    implicitWidth: compact ? LithicTokens.size.windowMinWidth : railWidth
    implicitHeight: compact ? LithicTokens.size.railBottomHeight
                            : LithicTokens.size.windowMinHeight
    padding: 0
    Accessible.role: Accessible.Pane

    function tileHeight(index) {
        if (_squeezed && index < _stackCount)
            return _squeezedTile
        return index >= 0 && index < _tileHeights.length ? _tileHeights[index] : tileSize
    }

    // destinations before footIndex stack from startY, New taking the slot after the first;
    // the rest keep their order as one group standing on the foot
    function destinationY(index) {
        const gap = LithicTokens.size.railTileGap
        const foot = Math.max(0, Math.min(footIndex, destinations.length))
        if (index < foot) {
            let y = startY + (showNew && index > 0 ? tileSize + gap : 0)
            for (let above = 0; above < index; ++above)
                y += tileHeight(above) + _stackGap
            return y
        }
        let y = height - LithicTokens.size.railItemBottomInset
        for (let below = destinations.length - 1; below >= index; --below)
            y -= tileHeight(below) + (below > index ? gap : 0)
        return y
    }

    readonly property int compactColumns: 1 + destinations.length + (showNew ? 1 : 0)

    function compactColumn(index) {
        return index === 0 ? 1 : index + 1 + (showNew ? 1 : 0)
    }

    function toggleNewMenu() {
        if (newPopover.opened)
            newPopover.close()
        else if (showNew)
            newPopover.open()
    }

    FontMetrics {
        id: captionMetrics
        font.family: Lithic.type.caption.family
        font.pixelSize: Lithic.type.caption.pixelSize
        font.weight: Lithic.type.caption.weight
        onFontChanged: root._layoutLabels()
    }

    // measures a label the way a tile draws it, wrapping only where a line may break
    Label {
        id: labelProbe
        visible: false
        typeRole: "caption"
        wrapMode: Text.WordWrap
        elide: Text.ElideNone
    }

    TextMetrics {
        id: wordElider
        font: labelProbe.font
        elide: Text.ElideRight
    }

    component DestinationButton: T.AbstractButton {
        id: destination
        required property string destinationLabel
        // what the tile draws: the label, or with a word too wide for the tile shortened
        property string shownLabel: destinationLabel
        required property string glyphName
        required property bool selected
        property bool notched: false
        property string previewState: ""
        property bool showLabel: true
        // 0 keeps the label on one line, as the medium and compact rails draw it
        property real labelWidth: 0
        property int labelLines: 2
        // a tile that shows no label says its name under the pointer
        readonly property var tip: tipLoader.item

        Loader {
            id: tipLoader
            active: !destination.showLabel
            sourceComponent: Component {
                Tooltip {
                    anchorItem: destination
                    text: destination.destinationLabel
                }
            }
        }

        hoverEnabled: true
        activeFocusOnTab: true
        Accessible.name: destinationLabel
        Accessible.role: Accessible.Button
        // Return goes to a destination as Space does, like every other control that acts
        Keys.onReturnPressed: event => visible ? click() : (event.accepted = false)
        Keys.onEnterPressed: event => visible ? click() : (event.accepted = false)

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
                keyboardFocusVisible: destination.visualFocus
                                      || destination.previewState === "focus"
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
                width: labelWidth > 0 ? labelWidth : undefined
                text: shownLabel
                typeRole: "caption"
                ink: selected ? Lithic.onSel : Lithic.fg2
                horizontalAlignment: Text.AlignHCenter
                wrapMode: labelWidth > 0 ? Text.WordWrap : Text.NoWrap
                maximumLineCount: labelLines
            }
        }

        // the one badge the rail may show: something here needs you
        Notch {
            visible: notched
            anchors.left: parent.left
            anchors.top: parent.top
            size: LithicTokens.cut.notch
            fill: Lithic.amber
            corner: "topLeft"
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
                visible: root._shownIndex >= 0
                x: (parent.width - root.tileWidth) / 2
                y: root.destinationY(root._selectionIndex)
                width: root.tileWidth
                height: root.tileHeight(root._selectionIndex)
                fill: Lithic.sel
                cut: LithicTokens.cut.mid
                ChiselNumberBehavior on y { enabled: root._glide; duration: LithicTokens.motion.select }
                ChiselNumberBehavior on height { enabled: root._glide; duration: LithicTokens.motion.select }
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
                mark: root.brandMark
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
                    width: root.tileWidth
                    height: root.tileHeight(index)
                    destinationLabel: modelData.label
                    glyphName: modelData.glyph
                    selected: root.currentIndex === index
                    notched: root.notchIndex === index
                    showLabel: root.expanded && (index >= root._stackCount || root._labelsFit)
                    labelWidth: root.expanded ? root._labelWidth : 0
                    labelLines: root._squeezed && index < root._stackCount ? 1 : 2
                    shownLabel: root.expanded ? root.labelText(index) : modelData.label
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
                visible: !root.compact && root.showNew
                menuOpen: root.previewNewOpen || newPopover.opened
                previewState: root.previewState
                onClicked: root.toggleNewMenu()
            }
        }

        Item {
            id: bottomRail
            anchors.fill: parent
            visible: root.compact
            readonly property real columnWidth: width / root.compactColumns

            CutRect {
                visible: root._shownIndex >= 0
                x: root.compactColumn(root._selectionIndex) * bottomRail.columnWidth
                y: (parent.height - LithicTokens.size.railDestinationCompact) / 2
                width: bottomRail.columnWidth
                height: LithicTokens.size.railDestinationCompact
                fill: Lithic.sel
                cut: LithicTokens.cut.small
                ChiselNumberBehavior on x { enabled: root._glide; duration: LithicTokens.motion.select }
            }

            BrandMark {
                id: compactHomeMark
                x: (bottomRail.columnWidth - width) / 2
                y: (parent.height - height - LithicTokens.size.railItemLabelGap
                    - Lithic.type.caption.lineHeight) / 2
                width: LithicTokens.size.brandMarkCompact
                height: width
                markSize: width
                mark: root.brandMark
                previewState: root.previewState
                onClicked: root.homeTriggered()
            }
            Label {
                anchors.horizontalCenter: compactHomeMark.horizontalCenter
                y: compactHomeMark.y + compactHomeMark.height
                   + LithicTokens.size.railItemLabelGap
                text: Lithic.tr("Home")
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
                    notched: root.notchIndex === index
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
                visible: root.showNew
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
        previewEmbedded: root.previewNewOpen && root.showNew
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
