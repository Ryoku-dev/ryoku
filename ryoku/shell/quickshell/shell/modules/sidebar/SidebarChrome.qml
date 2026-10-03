pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services
import "SidebarCatalog.js" as SidebarCatalog

Item {
    id: root

    required property var screen
    required property string side
    property real reveal: 0
    property bool open: false

    readonly property real s: Tokens.uiScaleFor(screen ? screen.name : "")
    readonly property real pad: Tokens.s4 * s
    readonly property string requestedTab: SidebarState.activeTab(screen, side)
    readonly property var pluginCards: pluginHost.cards(side)
    readonly property var tabs: buildTabs()
    property string selectedTab: ""
    property date now: new Date()

    LayoutMirroring.enabled: Qt.application.layoutDirection === Qt.RightToLeft
    LayoutMirroring.childrenInherit: true

    function buildTabs() {
        var configured = Config.sidebars[root.side].cards || [];
        var groups = [];
        for (var i = 0; i < configured.length; ++i) {
            var entry = SidebarCatalog.byId(configured[i]);
            if (!entry || entry.side !== root.side)
                continue;
            var group = null;
            for (var j = 0; j < groups.length; ++j) {
                if (groups[j].id === entry.tab) {
                    group = groups[j];
                    break;
                }
            }
            if (!group) {
                group = { id: entry.tab, label: entry.label, glyph: entry.glyph, cards: [] };
                groups.push(group);
            }
            group.cards.push({ id: entry.id, entry: null });
        }
        var orderedPlugins = root.pluginCards.slice();
        orderedPlugins.sort(function(a, b) {
            var ai = configured.indexOf(a.id);
            var bi = configured.indexOf(b.id);
            if (ai < 0 && bi < 0)
                return 0;
            if (ai < 0)
                return 1;
            if (bi < 0)
                return -1;
            return ai - bi;
        });
        for (var k = 0; k < orderedPlugins.length; ++k) {
            var plugin = orderedPlugins[k];
            var tabLabel = plugin.tab || "Plugins";
            var tabId = tabLabel === "Plugins" ? "plugins"
                : "plugin-" + tabLabel.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
            var pluginGroup = null;
            for (var m = 0; m < groups.length; ++m) {
                if (groups[m].pluginTab === true && groups[m].id === tabId) {
                    pluginGroup = groups[m];
                    break;
                }
            }
            if (!pluginGroup) {
                pluginGroup = {
                    id: tabId,
                    label: tabLabel,
                    glyph: "extension",
                    pluginTab: true,
                    cards: []
                };
                groups.push(pluginGroup);
            }
            pluginGroup.cards.push({ id: plugin.id, entry: plugin.entry });
        }
        return groups;
    }

    function resolvedRequestedTab() {
        var requested = root.requestedTab;
        if (requested === "compress" || requested === "install")
            requested = "tools";
        for (var i = 0; i < root.tabs.length; ++i) {
            if (root.tabs[i].id === requested)
                return requested;
            for (var j = 0; j < root.tabs[i].cards.length; ++j) {
                if (root.tabs[i].cards[j].id === requested
                        || root.tabs[i].cards[j].id === "plugin:" + requested)
                    return root.tabs[i].id;
            }
        }
        return root.tabs.length > 0 ? root.tabs[0].id : "";
    }

    function syncSelection() {
        root.selectedTab = root.resolvedRequestedTab();
    }

    function activeIndex() {
        for (var i = 0; i < root.tabs.length; ++i)
            if (root.tabs[i].id === root.selectedTab) return i;
        return 0;
    }

    function motionDuration(duration) {
        return Math.round(duration * SidebarState.motionMultiplier);
    }

    onRequestedTabChanged: syncSelection()
    onTabsChanged: syncSelection()
    Component.onCompleted: syncSelection()

    Timer {
        interval: 1000
        repeat: true
        running: root.open
        triggeredOnStart: true
        onTriggered: root.now = new Date()
    }

    SidebarPlugins { id: pluginHost }

    Column {
        anchors.fill: parent
        anchors.margins: root.pad
        spacing: Tokens.s3 * root.s

        Item {
            id: header
            width: parent.width
            height: 52 * root.s

            Row {
                anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                spacing: Tokens.s3 * root.s

                Rectangle {
                    width: 34 * root.s
                    height: width
                    radius: Tokens.radius * root.s
                    color: Tokens.bone

                    Text {
                        anchors.centerIn: parent
                        text: Config.markText || "力"
                        color: Tokens.inkOnBone
                        font.family: Tokens.jp
                        font.pixelSize: Tokens.fRow * root.s
                        font.weight: Font.DemiBold
                    }
                }

                Column {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 1
                    Text {
                        text: Config.brandName || "Ryoku"
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fValue * root.s
                    }
                    Text {
                        text: root.side === "left" ? I18n.tr("Desktop controls") : I18n.tr("Desktop companion")
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fMicro * root.s
                        font.letterSpacing: Tokens.trackLabel
                    }
                }
            }

            Column {
                anchors { right: parent.right; verticalCenter: parent.verticalCenter }
                spacing: 1
                Text {
                    anchors.right: parent.right
                    text: Qt.formatTime(root.now, "hh:mm")
                    color: Tokens.ink
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fRow * root.s
                    font.weight: Font.DemiBold
                }
                Text {
                    anchors.right: parent.right
                    text: Qt.formatDate(root.now, "ddd, MMM d")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro * root.s
                }
            }
        }

        Rectangle {
            id: rail
            width: parent.width
            height: 42 * root.s
            radius: height / 2
            color: Tokens.paperLift
            border.width: Tokens.border
            border.color: Tokens.line
            clip: true

            Flickable {
                id: tabFlick
                anchors.fill: parent
                anchors.margins: 4 * root.s
                contentWidth: tabRow.width
                contentHeight: height
                boundsBehavior: Flickable.StopAtBounds
                flickableDirection: Flickable.HorizontalFlick

                Item {
                    id: railContent
                    width: Math.max(tabFlick.width, tabRow.width)
                    height: tabFlick.height

                    Rectangle {
                        id: activeIndicator
                        readonly property var target: tabRepeater.itemAt(root.activeIndex())
                        x: target ? target.x : 0
                        width: target ? target.width : 0
                        height: parent.height
                        radius: height / 2
                        color: Tokens.bone
                        Behavior on x {
                            enabled: !Motion.reduce && !Tokens.reduceMotion
                            NumberAnimation { duration: root.motionDuration(Tokens.move); easing.type: Tokens.ease }
                        }
                        Behavior on width {
                            enabled: !Motion.reduce && !Tokens.reduceMotion
                            NumberAnimation { duration: root.motionDuration(Tokens.move); easing.type: Tokens.ease }
                        }
                    }

                    Row {
                        id: tabRow
                        height: parent.height
                        spacing: Tokens.s1 * root.s

                        Repeater {
                            id: tabRepeater
                            model: root.tabs
                            delegate: Item {
                                id: tabButton
                                required property var modelData
                                required property int index
                                readonly property bool active: root.selectedTab === modelData.id
                                readonly property bool compact: root.tabs.length > 4
                                width: compact
                                    ? (tabFlick.width - tabRow.spacing * (root.tabs.length - 1)) / root.tabs.length
                                    : Math.max(42 * root.s,
                                        tabLabel.implicitWidth + tabGlyph.implicitWidth
                                            + Tokens.s1 * root.s + 24 * root.s)
                                height: tabRow.height
                                transform: [
                                    Translate {
                                        y: hover.hovered && !tap.pressed ? -1 * root.s : 0
                                        Behavior on y {
                                            enabled: !Motion.reduce && !Tokens.reduceMotion
                                            NumberAnimation {
                                                duration: root.motionDuration(Tokens.snap)
                                                easing.type: Tokens.easeSnap
                                            }
                                        }
                                    },
                                    Scale {
                                        id: pressScale
                                        origin.x: tabButton.width / 2
                                        origin.y: tabButton.height / 2
                                        xScale: tap.pressed ? 0.98 : 1
                                        yScale: xScale
                                        Behavior on xScale {
                                            enabled: !Motion.reduce && !Tokens.reduceMotion
                                            NumberAnimation { duration: root.motionDuration(Tokens.snap); easing.type: Tokens.easeSnap }
                                        }
                                        Behavior on yScale {
                                            enabled: !Motion.reduce && !Tokens.reduceMotion
                                            NumberAnimation { duration: root.motionDuration(Tokens.snap); easing.type: Tokens.easeSnap }
                                        }
                                    }
                                ]

                                Row {
                                    anchors.centerIn: parent
                                    spacing: Tokens.s1 * root.s
                                    Text {
                                        id: tabGlyph
                                        visible: !tabButton.compact
                                        text: tabButton.modelData.glyph
                                        color: tabButton.active ? Tokens.inkOnBone : Tokens.inkMuted
                                        font.family: "Material Symbols Rounded"
                                        font.pixelSize: Tokens.fRow * root.s
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                    Text {
                                        id: tabLabel
                                        text: I18n.tr(tabButton.modelData.label)
                                        color: tabButton.active ? Tokens.inkOnBone : Tokens.inkDim
                                        font.family: Tokens.ui
                                        font.pixelSize: Tokens.fMicro * root.s
                                        font.weight: tabButton.active ? Font.DemiBold : Font.Medium
                                        anchors.verticalCenter: parent.verticalCenter
                                    }
                                }

                                HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
                                TapHandler {
                                    id: tap
                                    onTapped: SidebarState.selectTab(root.side, root.screen, tabButton.modelData.id)
                                }
                            }
                        }
                    }
                }
            }
        }

        Item {
            id: pages
            width: parent.width
            height: parent.height - header.height - rail.height - parent.spacing * 2
            clip: true

            Repeater {
                model: root.tabs
                delegate: Item {
                    id: page
                    required property var modelData
                    required property int index
                    readonly property bool active: root.selectedTab === modelData.id
                    y: 0
                    width: parent.width
                    height: parent.height
                    visible: active || opacity > 0.01
                    enabled: active
                    opacity: active ? 1 : 0
                    x: active ? 0 : (index < root.activeIndex() ? -18 : 18) * root.s

                    Behavior on opacity {
                        enabled: !Motion.reduce && !Tokens.reduceMotion
                        NumberAnimation {
                            duration: root.motionDuration(Tokens.swap)
                            easing.type: Tokens.ease
                        }
                    }
                    Behavior on x {
                        enabled: !Motion.reduce && !Tokens.reduceMotion
                        NumberAnimation {
                            duration: root.motionDuration(Tokens.swap)
                            easing.type: Tokens.ease
                        }
                    }

                    Flickable {
                        id: cardFlick
                        anchors.fill: parent
                        contentHeight: cardsColumn.height + Tokens.s4 * root.s
                        clip: true
                        boundsBehavior: Flickable.StopAtBounds
                        flickableDirection: Flickable.VerticalFlick
                        QQC.ScrollBar.vertical: ScrollRail { policy: QQC.ScrollBar.AsNeeded }
                        WheelScroll { }

                        Column {
                            id: cardsColumn
                            width: cardFlick.width - Tokens.s3 * root.s
                            spacing: Tokens.s3 * root.s

                            Repeater {
                                model: page.modelData.cards
                                delegate: SidebarCardHost {
                                    required property var modelData
                                    required property int index
                                    cardIndex: index
                                    page: root.requestedTab
                                    width: cardsColumn.width
                                    cardId: modelData.id
                                    pluginEntry: modelData.entry
                                    s: root.s
                                    open: root.open && root.reveal >= 0.999
                                    reveal: root.reveal
                                    tabActive: page.active
                                    onRequestClose: SidebarState.closeAll(root.screen)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
