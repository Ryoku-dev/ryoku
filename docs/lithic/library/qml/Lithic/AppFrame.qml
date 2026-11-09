import QtQuick
import QtQuick.Templates as T

T.Control {
    id: root

    default property alias workData: workHost.data
    property alias listData: listHost.data
    property alias overlayData: overlayHost.data
    property bool showList: true
    property string compactView: "work"
    property bool customDecorations: true
    property int currentDestination: 0
    property var destinations: [
        { label: "Fleet", glyph: "dns" },
        { label: "Activity", glyph: "history" },
        { label: "Settings", glyph: "tune" }
    ]
    property Component windowControlsComponent: defaultWindowControls
    property string previewState: ""
    readonly property string sizeClass: width >= LithicTokens.breakpoint.expanded
                                         ? "expanded"
                                         : width >= LithicTokens.breakpoint.medium
                                           ? "medium" : "compact"
    readonly property bool compact: sizeClass === "compact"
    readonly property real railWidth: sizeClass === "expanded"
                                      ? LithicTokens.size.railExpanded
                                      : sizeClass === "medium"
                                        ? LithicTokens.size.railMedium : 0
    readonly property real listWidth: sizeClass === "expanded"
                                      ? LithicTokens.size.listExpanded
                                      : LithicTokens.size.listMedium
    readonly property real contentX: railWidth + (compact ? 0 : LithicTokens.size.paneGap)
    readonly property real paneBottom: compact
                                       ? height - LithicTokens.size.railBottomHeight
                                         - LithicTokens.size.paneGap
                                       : height

    signal homeTriggered()
    signal destinationTriggered(int index, string label)
    signal newItemTriggered(int index, string label)
    signal minimizeRequested()
    signal maximizeRequested()
    signal closeRequested()

    implicitWidth: LithicTokens.size.windowDefaultWidth
    implicitHeight: LithicTokens.size.windowDefaultHeight
    padding: 0
    clip: true
    Accessible.role: Accessible.Window

    background: Rectangle { color: Lithic.ground }

    contentItem: Item {
        Rail {
            id: rail
            x: 0
            y: root.compact ? root.height - LithicTokens.size.railBottomHeight : 0
            width: root.compact ? root.width : root.railWidth
            height: root.compact ? LithicTokens.size.railBottomHeight : root.height
            sizeClass: root.sizeClass
            currentIndex: root.currentDestination
            destinations: root.destinations
            overlayParent: overlayHost
            previewState: root.previewState
            onHomeTriggered: root.homeTriggered()
            onDestinationTriggered: (index, label) => {
                root.currentDestination = index
                root.destinationTriggered(index, label)
            }
            onNewItemTriggered: (index, label) => root.newItemTriggered(index, label)
        }

        CutRect {
            id: listPane
            visible: root.showList && (!root.compact || root.compactView === "list")
            x: root.compact ? 0 : root.contentX
            y: LithicTokens.size.paneTop
            width: root.compact ? root.width : root.listWidth
            height: Math.max(0, root.paneBottom - y)
            fill: Lithic.stone1
            cutTopLeft: LithicTokens.cut.large
            cutTopRight: LithicTokens.cut.large
            cutBottomLeft: 0
            cutBottomRight: 0

            Item {
                id: listHost
                anchors.fill: parent
                anchors.topMargin: root.customDecorations
                                   ? LithicTokens.size.paneDecorStrip : 0
            }
        }

        CutRect {
            id: workPane
            visible: !root.compact || root.compactView === "work" || !root.showList
            x: root.compact ? 0
                            : root.contentX
                              + (root.showList ? root.listWidth + LithicTokens.size.paneGap : 0)
            y: LithicTokens.size.paneTop
            width: root.compact ? root.width : Math.max(0, root.width - x)
            height: Math.max(0, root.paneBottom - y)
            fill: Lithic.stone2
            cutTopLeft: LithicTokens.cut.large
            cutTopRight: LithicTokens.cut.large
            cutBottomLeft: 0
            cutBottomRight: 0

            Item {
                id: workHost
                anchors.fill: parent
                anchors.leftMargin: LithicTokens.size.workContentInset
                anchors.rightMargin: LithicTokens.size.workContentInset
                anchors.bottomMargin: LithicTokens.size.workContentInset
                anchors.topMargin: LithicTokens.size.workContentInset
                                   + (root.customDecorations
                                      ? LithicTokens.size.paneDecorStrip : 0)
                clip: true
            }

            Loader {
                visible: root.customDecorations
                anchors.right: parent.right
                anchors.rightMargin: LithicTokens.space.s2
                anchors.top: parent.top
                height: LithicTokens.size.paneDecorStrip
                sourceComponent: root.windowControlsComponent
            }
        }

        Item {
            id: overlayHost
            anchors.fill: parent
            z: 100
        }
    }

    Component {
        id: defaultWindowControls
        Row {
            spacing: LithicTokens.space.s1
            Plaque {
                small: true
                kind: "quiet"
                glyph: "remove"
                text: ""
                accessibleName: "Minimize"
                onClicked: root.minimizeRequested()
            }
            Plaque {
                small: true
                kind: "quiet"
                glyph: "crop_square"
                text: ""
                accessibleName: "Maximize"
                onClicked: root.maximizeRequested()
            }
            Plaque {
                small: true
                kind: "quiet"
                glyph: "close"
                text: ""
                accessibleName: "Close"
                onClicked: root.closeRequested()
            }
        }
    }
}
