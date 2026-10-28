pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: kitCard

    property var categories: []
    property bool loaded: false
    property real appear: 1
    property bool interactive: true
    signal requestClose()

    readonly property var tabs: [
        { id: "keys", label: "Keys", source: "tabs/KeysTab.qml" },
        { id: "commands", label: "Commands", source: "commands/CommandsTab.qml" },
        { id: "phone", label: "Phone", source: "phone/PhoneTab.qml" },
        { id: "devtools", label: "Dev tools", source: "devtools/DevToolsTab.qml" },
        { id: "typing", label: "Typing test", source: "typing/TypingTab.qml" }
    ]
    property int currentIndex: 0
    property int tabRevision: 0
    property bool searchAutoSwitchArmed: false
    property bool keepSearchFocus: false
    property bool userSelectedTab: false
    readonly property string normalizedQuery: searchInput.text.trim().toLowerCase()
    property var currentTab: null
    readonly property var currentRail: kitCard.currentTab ? kitCard.currentTab.rail : []
    property string requestedTab: ""
    property string requestedRail: ""
    property int requestRevision: 0
    property string pendingRailTab: ""
    property string pendingRailId: ""
    property bool railEnsureQueued: false

    function loaderAt(index) {
        return tabRepeater.itemAt(index);
    }

    function refreshCurrentTab() {
        const loader = kitCard.loaderAt(kitCard.currentIndex);
        kitCard.currentTab = loader ? loader.item : null;
    }

    function tabIndex(id) {
        for (let i = 0; i < kitCard.tabs.length; i++) {
            if (kitCard.tabs[i].id === id)
                return i;
        }
        return 0;
    }

    function hasTab(id) {
        for (let i = 0; i < kitCard.tabs.length; i++) {
            if (kitCard.tabs[i].id === id)
                return true;
        }
        return false;
    }

    function railItemSelectable(item) {
        return item && item.header !== true;
    }

    function firstSelectableRailId() {
        for (let i = 0; i < kitCard.currentRail.length; i++) {
            if (kitCard.railItemSelectable(kitCard.currentRail[i]))
                return kitCard.currentRail[i].id;
        }
        return "";
    }

    function syncLoader(loader) {
        if (!loader || !loader.item)
            return;
        loader.item.kit = state;
        loader.item.query = kitCard.normalizedQuery;
        loader.item.current = loader.tabIndex === kitCard.currentIndex;
        if (loader.tabIndex === 0) {
            loader.item.categories = kitCard.categories;
            loader.item.loaded = kitCard.loaded;
        }
    }

    function syncTabs() {
        for (let i = 0; i < kitCard.tabs.length; i++)
            kitCard.syncLoader(kitCard.loaderAt(i));
        kitCard.refreshCurrentTab();
        kitCard.scheduleCurrentRail();
    }

    function switchTab(index, remember) {
        kitCard.searchAutoSwitchArmed = false;
        const bounded = (index + kitCard.tabs.length) % kitCard.tabs.length;
        const shouldRemember = remember === undefined ? true : remember;
        if (shouldRemember) {
            kitCard.userSelectedTab = true;
            if (state.ready)
                state.set("frame.tab", kitCard.tabs[bounded].id);
        }
        if (kitCard.currentIndex === bounded) {
            kitCard.activateCurrent();
            return;
        }
        kitCard.currentIndex = bounded;
    }

    function activateCurrent() {
        if (!kitCard.interactive)
            return;
        if (kitCard.currentIndex === 0) {
            searchInput.forceActiveFocus();
            return;
        }
        if (kitCard.currentTab && kitCard.currentTab.activate)
            kitCard.currentTab.activate();
    }

    function focusedItemIsTextEntry() {
        const window = kitCard.Window.window;
        const item = window ? window.activeFocusItem : null;
        return item instanceof TextInput || item instanceof TextEdit;
    }

    function routePrintableToSearch(event) {
        if (!kitCard.interactive || !event.text || event.text.length === 0
                || event.text.charCodeAt(0) < 0x20
                || event.text.charCodeAt(0) === 0x7f
                || (event.modifiers & (Qt.ControlModifier | Qt.AltModifier | Qt.MetaModifier))
                || kitCard.focusedItemIsTextEntry())
            return;
        searchInput.forceActiveFocus();
        searchInput.insert(searchInput.cursorPosition, event.text);
        event.accepted = true;
    }

    function handleFrameKey(event) {
        if (event.key === Qt.Key_Escape) {
            event.accepted = true;
            if (!event.isAutoRepeat)
                kitCard.handleKitEscape();
            return;
        }
        kitCard.routePrintableToSearch(event);
    }

    Keys.onPressed: event => kitCard.handleFrameKey(event)
    Keys.priority: Keys.BeforeItem

    function currentTabRailReady() {
        if (!kitCard.currentTab)
            return false;
        return typeof kitCard.currentTab.railReady === "undefined"
            || kitCard.currentTab.railReady;
    }

    function scheduleCurrentRail() {
        if (kitCard.railEnsureQueued)
            return;
        kitCard.railEnsureQueued = true;
        Qt.callLater(() => {
            if (kitCard.currentTabRailReady()) {
                kitCard.applyPendingRail();
                kitCard.ensureCurrentRail();
            }
            kitCard.railEnsureQueued = false;
        });
    }

    function ensureCurrentRail() {
        if (!kitCard.currentTabRailReady() || !kitCard.currentRail
                || kitCard.currentRail.length === 0)
            return;
        let found = false;
        for (let i = 0; i < kitCard.currentRail.length; i++) {
            if (kitCard.currentRail[i].id === kitCard.currentTab.railCurrent
                    && kitCard.railItemSelectable(kitCard.currentRail[i])) {
                found = true;
                break;
            }
        }
        if (!found)
            kitCard.currentTab.railCurrent = kitCard.firstSelectableRailId();
        Qt.callLater(kitCard.scrollCurrentRailIntoView);
    }

    function applyPendingRail() {
        if (!kitCard.currentTab || kitCard.pendingRailId.length === 0
                || kitCard.tabs[kitCard.currentIndex].id !== kitCard.pendingRailTab)
            return;
        const requested = kitCard.pendingRailId;
        kitCard.pendingRailId = "";
        kitCard.pendingRailTab = "";
        kitCard.currentTab.railCurrent = requested;
        Qt.callLater(kitCard.scrollCurrentRailIntoView);
    }

    function openTab(tab, railId) {
        if (!kitCard.hasTab(tab))
            return;
        kitCard.pendingRailTab = tab;
        kitCard.pendingRailId = railId || "";
        kitCard.switchTab(kitCard.tabIndex(tab), false);
        kitCard.syncTabs();
    }

    function applyInitialSelection() {
        if (kitCard.hasTab(kitCard.requestedTab)) {
            kitCard.openTab(kitCard.requestedTab, kitCard.requestedRail);
            return;
        }
        if (state.ready) {
            kitCard.currentIndex = kitCard.tabIndex(state.get("frame.tab", "keys"));
            kitCard.syncTabs();
        }
    }

    function scrollCurrentRailIntoView() {
        if (!kitCard.currentTab || !railScroll || railScroll.contentHeight <= railScroll.height)
            return;
        for (let i = 0; i < railRepeater.count; i++) {
            const item = railRepeater.itemAt(i);
            if (!item || item.modelData.id !== kitCard.currentTab.railCurrent)
                continue;
            const top = item.y;
            const bottom = top + item.height;
            const maximum = Math.max(0, railScroll.contentHeight - railScroll.height);
            if (top < railScroll.contentY)
                railScroll.contentY = Math.max(0, top);
            else if (bottom > railScroll.contentY + railScroll.height)
                railScroll.contentY = Math.min(maximum, bottom - railScroll.height);
            return;
        }
    }


    function considerSearchSwitch() {
        if (!kitCard.searchAutoSwitchArmed)
            return;
        if (kitCard.normalizedQuery.length === 0) {
            kitCard.searchAutoSwitchArmed = false;
            return;
        }
        if (!kitCard.currentTab)
            return;
        if (kitCard.currentTab.matches !== 0) {
            kitCard.searchAutoSwitchArmed = false;
            return;
        }
        for (let i = 0; i < kitCard.tabs.length; i++) {
            const loader = kitCard.loaderAt(i);
            if (loader && loader.item && loader.item.matches > 0) {
                kitCard.searchAutoSwitchArmed = false;
                kitCard.keepSearchFocus = true;
                kitCard.switchTab(i, false);
                return;
            }
        }
        kitCard.searchAutoSwitchArmed = false;
    }

    function handleKitEscape() {
        if (kitCard.currentTab && kitCard.currentTab.handleEscape
                && kitCard.currentTab.handleEscape())
            return;
        if (searchInput.text.length > 0) {
            searchInput.text = "";
            searchInput.forceActiveFocus();
            return;
        }
        kitCard.requestClose();
    }

    onNormalizedQueryChanged: {
        kitCard.searchAutoSwitchArmed = kitCard.normalizedQuery.length > 0;
        kitCard.syncTabs();
        Qt.callLater(kitCard.considerSearchSwitch);
    }
    onCategoriesChanged: kitCard.syncTabs()
    onLoadedChanged: kitCard.syncTabs()
    onCurrentIndexChanged: {
        kitCard.syncTabs();
        if (kitCard.keepSearchFocus) {
            kitCard.keepSearchFocus = false;
            Qt.callLater(searchInput.forceActiveFocus);
        } else {
            Qt.callLater(kitCard.activateCurrent);
        }
    }
    onCurrentRailChanged: kitCard.scheduleCurrentRail()
    onRequestRevisionChanged: kitCard.openTab(kitCard.requestedTab, kitCard.requestedRail)

    Component.onCompleted: {
        kitCard.applyInitialSelection();
        kitCard.syncTabs();
        if (kitCard.interactive)
            searchInput.forceActiveFocus();
    }
    onInteractiveChanged: {
        if (kitCard.interactive) {
            searchInput.forceActiveFocus();
            Qt.callLater(kitCard.activateCurrent);
        }
    }

    KitState {
        id: state
        onReadyChanged: {
            if (!ready)
                return;
            if (kitCard.userSelectedTab) {
                state.set("frame.tab", kitCard.tabs[kitCard.currentIndex].id);
                return;
            }
            kitCard.applyInitialSelection();
        }
    }

    Shortcut { enabled: kitCard.interactive; sequence: "Ctrl+Tab"; onActivated: kitCard.switchTab(kitCard.currentIndex + 1) }
    Shortcut { enabled: kitCard.interactive; sequence: "Ctrl+Shift+Tab"; onActivated: kitCard.switchTab(kitCard.currentIndex - 1) }
    Shortcut { enabled: kitCard.interactive; sequence: "Ctrl+1"; onActivated: kitCard.switchTab(0) }
    Shortcut { enabled: kitCard.interactive; sequence: "Ctrl+2"; onActivated: kitCard.switchTab(1) }
    Shortcut { enabled: kitCard.interactive; sequence: "Ctrl+3"; onActivated: kitCard.switchTab(2) }
    Shortcut { enabled: kitCard.interactive; sequence: "Ctrl+4"; onActivated: kitCard.switchTab(3) }
    Shortcut { enabled: kitCard.interactive; sequence: "Ctrl+5"; onActivated: kitCard.switchTab(4) }

    Rectangle {
        id: card
        anchors.centerIn: parent
        width: Math.max(1, Math.min(1340, parent.width - 2 * Tokens.s4))
        height: Math.max(1, Math.min(860, parent.height - 2 * Tokens.s4))
        radius: Tokens.radius * 1.5
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.line
        opacity: kitCard.appear
        scale: 0.985 + 0.015 * kitCard.appear

        readonly property int pad: width < 760 ? Tokens.s4 : Tokens.s6
        readonly property int railWidth: width < 760 ? 148 : 212

        MouseArea { anchors.fill: parent }

        Column {
            id: masthead
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.topMargin: card.pad
            anchors.leftMargin: card.pad
            spacing: Tokens.s1

            Row {
                spacing: Tokens.s2
                Rectangle {
                    width: 16
                    height: 1
                    color: Tokens.ink
                    anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: UiPreferences.brandMark("\u529b")
                    color: Tokens.ink
                    font.family: Tokens.jp
                    font.pixelSize: Tokens.fMicro
                    anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: I18n.tr("KIT")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                    font.weight: Font.Medium
                    font.letterSpacing: Tokens.trackMark
                    anchors.verticalCenter: parent.verticalCenter
                }
            }

            Text {
                text: I18n.tr("Kit")
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: Tokens.fTitle
            }
        }

        Marginalia {
            anchors.right: closeButton.left
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: closeButton.verticalCenter
            kana: "\u9053\u5177"
            index: (kitCard.currentIndex < 9 ? "0" : "") + String(kitCard.currentIndex + 1)
            label: I18n.tr(kitCard.tabs[kitCard.currentIndex].label).toUpperCase()
            glyph: "meander"
            glyph2: "torii"
            visible: card.width >= 720
        }

        Rectangle {
            id: closeButton
            anchors.top: parent.top
            anchors.right: parent.right
            anchors.topMargin: card.pad
            anchors.rightMargin: card.pad
            width: 30
            height: 30
            radius: Tokens.radius
            color: closeTap.pressed ? Tokens.tint16
                : (closeHover.hovered ? Tokens.tint10 : "transparent")
            border.width: Tokens.border
            border.color: closeHover.hovered ? Tokens.lineStrong : Tokens.line
            scale: closeTap.pressed ? 0.95 : 1
            Behavior on color {
                enabled: !Tokens.reduceMotion
                ColorAnimation { duration: Tokens.snap }
            }
            Behavior on border.color {
                enabled: !Tokens.reduceMotion
                ColorAnimation { duration: Tokens.snap }
            }
            Behavior on scale {
                enabled: !Tokens.reduceMotion
                NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
            }

            Text {
                anchors.centerIn: parent
                text: "\u2715"
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
            }
            HoverHandler { id: closeHover; cursorShape: Qt.PointingHandCursor }
            TapHandler { id: closeTap; onTapped: kitCard.requestClose() }
        }

        Rectangle {
            id: search
            anchors.top: masthead.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.topMargin: Tokens.s4
            anchors.leftMargin: card.pad
            anchors.rightMargin: card.pad
            height: 40
            radius: Tokens.radius
            color: Tokens.paper
            border.width: searchInput.activeFocus ? 2 : Tokens.border
            border.color: searchInput.activeFocus ? Tokens.ink : Tokens.line
            Behavior on border.color {
                enabled: !Tokens.reduceMotion
                ColorAnimation { duration: Tokens.snap }
            }

            Text {
                id: searchGlyph
                anchors.left: parent.left
                anchors.leftMargin: Tokens.s4
                anchors.verticalCenter: parent.verticalCenter
                text: "\uf002"
                color: searchInput.activeFocus ? Tokens.inkDim : Tokens.inkFaint
                font.family: Tokens.mono
                font.pixelSize: Tokens.fSmall
            }

            TextInput {
                id: searchInput
                anchors.left: searchGlyph.right
                anchors.leftMargin: Tokens.s3
                anchors.right: parent.right
                anchors.rightMargin: Tokens.s4
                anchors.verticalCenter: parent.verticalCenter
                clip: true
                selectByMouse: true
                color: Tokens.ink
                selectionColor: Tokens.lineStrong
                selectedTextColor: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fBody
            }

            Text {
                anchors.left: searchInput.left
                anchors.right: searchInput.right
                anchors.verticalCenter: searchInput.verticalCenter
                visible: searchInput.text.length === 0
                text: I18n.tr("Search Kit")
                color: Tokens.inkFaint
                font: searchInput.font
            }
        }

        Item {
            id: tabStrip
            anchors.top: search.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.topMargin: Tokens.s3
            anchors.leftMargin: card.pad
            anchors.rightMargin: card.pad
            height: 34

            Row {
                anchors.fill: parent
                spacing: Tokens.s2

                Repeater {
                    model: kitCard.tabs
                    delegate: Rectangle {
                        id: tabPlate
                        required property var modelData
                        required property int index
                        width: (tabStrip.width - Tokens.s2 * (kitCard.tabs.length - 1)) / kitCard.tabs.length
                        height: tabStrip.height
                        radius: Tokens.radius
                        readonly property bool selected: kitCard.currentIndex === index
                        readonly property var tabItem: {
                            const observed = kitCard.tabRevision;
                            const loader = kitCard.loaderAt(index);
                            return loader ? loader.item : null;
                        }
                        readonly property int matchCount: tabItem ? tabItem.matches : -1
                        color: selected ? Tokens.bone
                            : (tabTap.pressed ? Tokens.tint16
                            : (tabHover.hovered ? Tokens.tint5 : "transparent"))
                        border.width: Tokens.border
                        border.color: selected ? Tokens.bone
                            : (tabHover.hovered ? Tokens.lineStrong : Tokens.line)
                        scale: tabTap.pressed ? 0.98 : 1
                        Behavior on color {
                            enabled: !Tokens.reduceMotion
                            ColorAnimation { duration: Tokens.snap }
                        }
                        Behavior on border.color {
                            enabled: !Tokens.reduceMotion
                            ColorAnimation { duration: Tokens.snap }
                        }
                        Behavior on scale {
                            enabled: !Tokens.reduceMotion
                            NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
                        }
                        Row {
                            anchors.centerIn: parent
                            spacing: Tokens.s2

                            Text {
                                text: I18n.tr(tabPlate.modelData.label).toUpperCase()
                                color: tabPlate.selected ? Tokens.inkOnBone : Tokens.inkDim
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fMicro
                                font.weight: Font.Medium
                                font.letterSpacing: Tokens.trackLabel
                                elide: Text.ElideRight
                                width: Math.min(implicitWidth,
                                    tabPlate.width - (badge.visible ? badge.width + Tokens.s4 : Tokens.s3))
                                anchors.verticalCenter: parent.verticalCenter
                            }

                            Rectangle {
                                id: badge
                                readonly property bool shown: kitCard.normalizedQuery.length > 0
                                    && tabPlate.matchCount >= 0
                                visible: shown || opacity > 0
                                opacity: shown ? 1 : 0
                                scale: shown ? 1 : 0.82
                                width: Math.max(18, badgeText.implicitWidth + Tokens.s2)
                                height: 18
                                radius: Tokens.radius
                                color: tabPlate.selected ? Tokens.inkOnBone : "transparent"
                                border.width: Tokens.border
                                border.color: tabPlate.selected ? Tokens.inkOnBone : Tokens.line
                                anchors.verticalCenter: parent.verticalCenter
                                Behavior on opacity {
                                    enabled: !Tokens.reduceMotion
                                    NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                                }
                                Behavior on scale {
                                    enabled: !Tokens.reduceMotion
                                    NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                                }
                                Behavior on color {
                                    enabled: !Tokens.reduceMotion
                                    ColorAnimation { duration: Tokens.snap }
                                }

                                Text {
                                    id: badgeText
                                    anchors.centerIn: parent
                                    text: String(tabPlate.matchCount)
                                    color: tabPlate.selected ? Tokens.bone : Tokens.inkFaint
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fTiny
                                }
                            }
                        }

                        HoverHandler { id: tabHover }
                        TapHandler { id: tabTap; onTapped: kitCard.switchTab(tabPlate.index) }
                    }
                }
            }
        }

        Item {
            id: body
            anchors.top: tabStrip.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: footer.top
            anchors.topMargin: Tokens.s5
            anchors.leftMargin: card.pad
            anchors.rightMargin: card.pad
            anchors.bottomMargin: Tokens.s3

            Item {
                id: rail
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                width: kitCard.currentRail.length > 0 ? card.railWidth : 0
                visible: width > 0

                Flickable {
                    id: railScroll
                    anchors.fill: parent
                    clip: true
                    contentHeight: railColumn.implicitHeight
                    boundsBehavior: Flickable.StopAtBounds
                    interactive: contentHeight > height
                    ScrollBar.vertical: ScrollRail {
                        policy: railScroll.contentHeight > railScroll.height
                            ? ScrollBar.AsNeeded : ScrollBar.AlwaysOff
                    }
                    WheelScroll { }

                    Column {
                        id: railColumn
                        width: Math.max(1, railScroll.width
                            - (railScroll.contentHeight > railScroll.height ? Tokens.s2 : 0))
                        spacing: 2

                        Repeater {
                            id: railRepeater
                            model: kitCard.currentRail
                            delegate: Rectangle {
                                id: railItem
                                required property var modelData
                                required property int index
                                readonly property bool header: modelData.header === true
                                readonly property bool indented: modelData.indent === 1
                                property real entryProgress: Tokens.reduceMotion || header ? 1 : 0
                                width: railColumn.width
                                height: header ? (index === 0 ? 24 : 24 + Tokens.s3) : 34
                                radius: header ? 0 : Tokens.radius
                                readonly property bool selected: !header && kitCard.currentTab
                                    && kitCard.currentTab.railCurrent === modelData.id
                                color: selected ? Tokens.tint10
                                    : (!header && railTap.pressed ? Tokens.tint16
                                    : (!header && railHover.hovered ? Tokens.tint5 : "transparent"))
                                scale: !header && railTap.pressed ? 0.985 : 1
                                opacity: entryProgress
                                transform: Translate {
                                    x: railItem.header ? 0 : (1 - railItem.entryProgress) * -6
                                }
                                Behavior on color {
                                    enabled: !Tokens.reduceMotion
                                    ColorAnimation { duration: Tokens.snap }
                                }
                                Behavior on scale {
                                    enabled: !Tokens.reduceMotion
                                    NumberAnimation { duration: Tokens.snap; easing.type: Easing.OutCubic }
                                }

                                SequentialAnimation {
                                    id: railEntry
                                    PauseAnimation { duration: Math.min(railItem.index, 8) * 12 }
                                    NumberAnimation {
                                        target: railItem
                                        property: "entryProgress"
                                        from: 0
                                        to: 1
                                        duration: Tokens.durSmall
                                        easing.type: Easing.OutCubic
                                    }
                                }
                                Component.onCompleted: {
                                    if (!Tokens.reduceMotion && !railItem.header)
                                        railEntry.start();
                                }

                                Text {
                                    visible: railItem.header
                                    anchors.left: parent.left
                                    anchors.leftMargin: Tokens.s4
                                    anchors.right: parent.right
                                    anchors.rightMargin: Tokens.s3
                                    anchors.bottom: parent.bottom
                                    anchors.bottomMargin: Tokens.s1
                                    text: I18n.tr(railItem.modelData.label).toUpperCase()
                                    color: Tokens.inkFaint
                                    font.family: Tokens.ui
                                    font.pixelSize: Tokens.fTiny
                                    font.weight: Font.Medium
                                    font.letterSpacing: Tokens.trackMark
                                    elide: Text.ElideRight
                                }
                                Rectangle {
                                    anchors.left: parent.left
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 2
                                    height: parent.height - Tokens.s4
                                    radius: 1
                                    color: Tokens.ink
                                    visible: railItem.selected
                                }
                                Text {
                                    visible: !railItem.header
                                    anchors.left: parent.left
                                    anchors.leftMargin: Tokens.s4
                                        + (railItem.indented ? Tokens.s4 : 0)
                                    anchors.right: countText.left
                                    anchors.rightMargin: Tokens.s2
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: I18n.tr(railItem.modelData.label)
                                    color: railItem.modelData.count === 0 ? Tokens.inkFaint
                                        : (railItem.selected ? Tokens.ink : Tokens.inkDim)
                                    font.family: Tokens.ui
                                    font.pixelSize: railItem.indented ? Tokens.fMicro : Tokens.fSmall
                                    elide: Text.ElideRight
                                }
                                Text {
                                    id: countText
                                    visible: !railItem.header && railItem.modelData.count >= 0
                                    anchors.right: parent.right
                                    anchors.rightMargin: Tokens.s3
                                    anchors.verticalCenter: parent.verticalCenter
                                    text: visible ? String(railItem.modelData.count) : ""
                                    color: Tokens.inkFaint
                                    font.family: Tokens.mono
                                    font.pixelSize: Tokens.fMicro
                                }
                                HoverHandler {
                                    id: railHover
                                    enabled: !railItem.header
                                    cursorShape: Qt.PointingHandCursor
                                }
                                TapHandler {
                                    id: railTap
                                    enabled: !railItem.header
                                    onTapped: {
                                        if (!kitCard.currentTab)
                                            return;
                                        kitCard.currentTab.railCurrent = railItem.modelData.id;
                                        if (searchInput.text.length > 0)
                                            searchInput.text = "";
                                    }
                                }
                            }
                        }
                    }
                }
            }

            Rectangle {
                id: railDivider
                anchors.left: rail.right
                anchors.leftMargin: rail.visible ? Tokens.s5 : 0
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: rail.visible ? 1 : 0
                color: Tokens.lineSoft
            }

            Item {
                id: pane
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                anchors.left: railDivider.right
                anchors.right: parent.right
                anchors.leftMargin: rail.visible ? Tokens.s6 : 0
                clip: true

                Repeater {
                    id: tabRepeater
                    model: kitCard.tabs
                    delegate: Loader {
                        id: tabLoader
                        required property var modelData
                        required property int index
                        readonly property int tabIndex: index
                        readonly property bool selected: tabIndex === kitCard.currentIndex
                        anchors.fill: parent
                        source: modelData.source
                        active: true
                        enabled: selected
                        opacity: selected ? 1 : 0
                        z: selected ? 1 : 0
                        transform: Translate {
                            x: tabLoader.selected ? 0
                                : (tabLoader.tabIndex < kitCard.currentIndex ? -10 : 10)
                            Behavior on x {
                                enabled: !Tokens.reduceMotion
                                NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                            }
                        }
                        Behavior on opacity {
                            enabled: !Tokens.reduceMotion
                            NumberAnimation { duration: Tokens.durSmall; easing.type: Easing.OutCubic }
                        }
                        onStatusChanged: {
                            kitCard.tabRevision++;
                            kitCard.scheduleCurrentRail();
                            Qt.callLater(kitCard.considerSearchSwitch);
                        }
                        onLoaded: {
                            kitCard.tabRevision++;
                            kitCard.syncLoader(tabLoader);
                            kitCard.refreshCurrentTab();
                            kitCard.scheduleCurrentRail();
                            Qt.callLater(kitCard.considerSearchSwitch);
                        }

                        Connections {
                            target: tabLoader.item
                            ignoreUnknownSignals: true
                            function onMatchesChanged() {
                                Qt.callLater(kitCard.considerSearchSwitch);
                            }
                            function onRailChanged() {
                                kitCard.scheduleCurrentRail();
                            }
                            function onRailReadyChanged() {
                                kitCard.scheduleCurrentRail();
                            }
                            function onRailCurrentChanged() {
                                Qt.callLater(kitCard.scrollCurrentRailIntoView);
                            }
                            function onRequestClose() {
                                kitCard.requestClose();
                            }
                        }
                    }
                }
            }
        }

        Text {
            id: footer
            anchors.bottom: parent.bottom
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottomMargin: Tokens.s4
            text: I18n.tr("Ctrl+Tab switch") + "   \u00b7   "
                + I18n.tr("Ctrl+1-5 tabs") + "   \u00b7   "
                + I18n.tr("Esc back, clear, close")
            color: Tokens.inkFaint
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.letterSpacing: Tokens.trackLabel
        }
    }
}
