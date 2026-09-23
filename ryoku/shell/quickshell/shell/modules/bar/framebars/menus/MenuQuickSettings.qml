pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.FrameBars
import shell.services
import Ryoku.Ui.Singletons

// Quick settings is a fixed-width module host: the configured module rail and
// one content sheet share the same frame. Modules are catalogued centrally and
// loaded from quicksettings/, while device detail pages push over the host.
Item {
    id: root

    property real s: 1
    property bool open: false
    property real avail: 0
    property string initialPage: ""
    signal requestClose()

    implicitHeight: root.avail > 0 ? root.avail : 480

    readonly property var configuredModules: {
        const menus = Config.normalizedFrameBars.menus || {};
        const menu = menus["quick-settings"] || {};
        const requested = menu.modules || [];
        const modules = [];
        for (const id of requested) {
            if (MenuCatalog.quickSettingsModule(id) && modules.indexOf(id) < 0)
                modules.push(id);
        }
        return modules.length > 0 ? modules : MenuCatalog.defaultQuickSettingsModules();
    }
    property string activeModule: ""
    property string previousModule: ""
    property string pendingModule: ""
    property var moduleSeen: ({})

    function moduleEnabled(id) {
        return root.configuredModules.indexOf(id) >= 0;
    }

    function moduleLoader(id) {
        for (let i = 0; i < moduleRepeater.count; ++i) {
            const loader = moduleRepeater.itemAt(i);
            if (loader && loader.objectName === id)
                return loader;
        }
        return null;
    }

    function syncConfiguredModules() {
        if (root.configuredModules.length === 0)
            return;
        const next = root.moduleEnabled(root.activeModule)
            ? root.activeModule : root.configuredModules[0];
        const seen = Object.assign({}, root.moduleSeen);
        seen[next] = true;
        root.moduleSeen = seen;
        root.activeModule = next;
        if (!root.moduleEnabled(root.previousModule))
            root.previousModule = "";
        if (!root.moduleEnabled(root.pendingModule))
            root.pendingModule = "";
    }

    function commitModule(id) {
        root.pendingModule = "";
        if (id === root.activeModule)
            return;
        root.previousModule = root.activeModule;
        root.activeModule = id;
        previousModuleTimer.restart();
    }

    function switchToModule(id) {
        if (!root.applyingInitial)
            root.navReady = true;
        if (!root.moduleEnabled(id))
            return;
        if (id === root.activeModule) {
            root.pendingModule = "";
            return;
        }
        if (!root.moduleSeen[id]) {
            const seen = Object.assign({}, root.moduleSeen);
            seen[id] = true;
            root.moduleSeen = seen;
        }
        root.pendingModule = id;
        const loader = root.moduleLoader(id);
        if (loader && loader.status === Loader.Ready)
            root.commitModule(id);
    }

    function completePendingModule(id, loader) {
        if (root.pendingModule === id && loader.status === Loader.Ready)
            root.commitModule(id);
    }

    Timer {
        id: previousModuleTimer
        interval: Motion.push + 80
        onTriggered: root.previousModule = ""
    }

    property string page: ""
    property var pageSeen: ({})
    property string pendingPage: ""
    // Snap the panel straight to its initial page on open (no home flash); only
    // once the user navigates in-panel do the bands slide.
    property bool navReady: false
    property bool applyingInitial: false

    function pageLoader(id) {
        switch (id) {
        case "network": return networkPageLoader;
        case "bluetooth": return bluetoothPageLoader;
        case "audio-out": return audioOutPageLoader;
        case "audio-in": return audioInPageLoader;
        case "theme": return themePageLoader;
        }
        return null;
    }

    function showPage(id) {
        if (!root.applyingInitial)
            root.navReady = true;
        if (id === "") {
            root.pendingPage = "";
            root.page = "";
            return;
        }
        if (root.page === id) {
            root.pendingPage = "";
            return;
        }
        if (!root.pageSeen[id]) {
            const seen = Object.assign({}, root.pageSeen);
            seen[id] = true;
            root.pageSeen = seen;
        }
        root.pendingPage = id;
        const loader = root.pageLoader(id);
        if (loader && loader.status === Loader.Ready)
            root.completePendingPage(id, loader);
    }

    function completePendingPage(id, loader) {
        if (root.pendingPage === id && loader.status === Loader.Ready) {
            root.pendingPage = "";
            root.page = id;
        }
    }

    function pageTitle() {
        switch (root.page) {
        case "network": return I18n.tr("Wi-Fi");
        case "bluetooth": return I18n.tr("Bluetooth");
        case "audio-out": return I18n.tr("Sound output");
        case "audio-in": return I18n.tr("Microphone");
        case "theme": return I18n.tr("Colour scheme");
        }
        return "";
    }

    // The system pull (module services) holds until the reveal settles, so
    // opening stays smooth on low-resource machines.
    property bool settled: false
    // A brief hidden warm after login primes that pull once, so the first real
    // open already has its data and closing stays fluid.
    property bool warm: false

    function applyInitialPage() {
        if (!root.open || root.initialPage === "")
            return;
        root.applyingInitial = true;
        switch (root.initialPage) {
        case "notifications":
        case "weather":
        case "capture":
        case "media":
        case "stage":
            root.showPage("");
            root.switchToModule(root.initialPage);
            break;
        case "calendar":
            root.showPage("");
            root.switchToModule("home");
            break;
        default:
            root.showPage(root.initialPage);
        }
        root.applyingInitial = false;
    }

    Timer {
        id: initialPageApply
        interval: 0
        onTriggered: if (root.open) root.applyInitialPage()
    }

    Timer {
        id: settleTimer
        interval: 800
        onTriggered: root.settled = true
    }

    Timer {
        id: warmDelay
        interval: 4000
        onTriggered: { root.warm = true; warmHold.restart(); }
    }
    Timer {
        id: warmHold
        interval: 700
        onTriggered: root.warm = false
    }

    function scheduleInitialPage() {
        initialPageApply.restart();
    }

    onConfiguredModulesChanged: root.syncConfiguredModules()
    onOpenChanged: {
        if (root.open) {
            root.navReady = false;
            root.syncConfiguredModules();
            root.scheduleInitialPage();
            settleTimer.restart();
        } else {
            settleTimer.stop();
            root.settled = false;
            previousModuleTimer.stop();
            initialPageApply.stop();
            root.previousModule = "";
            root.pendingModule = "";
            root.pendingPage = "";
            root.page = "";
            root.navReady = false;
        }
    }
    onInitialPageChanged: if (root.open && root.initialPage !== "") root.scheduleInitialPage()
    onPageChanged: {
        if (root.page !== "" && !root.pageSeen[root.page]) {
            const seen = Object.assign({}, root.pageSeen);
            seen[root.page] = true;
            root.pageSeen = seen;
        }
    }
    Component.onCompleted: { root.syncConfiguredModules(); warmDelay.start(); }

    Item {
        id: mainBand
        width: parent.width
        height: parent.height
        x: root.page !== "" ? -root.width / 3 : 0
        visible: x > -root.width / 3 + 0.5

        Behavior on x {
            enabled: root.open && root.navReady
            NumberAnimation { duration: Motion.push; easing.type: Motion.pushCurve }
        }

        QsTabRail {
            id: rail
            z: 10
            height: parent.height
            modules: root.configuredModules
            activeModule: root.activeModule
            onModuleActivated: moduleId => root.switchToModule(moduleId)
            onRequestClose: root.requestClose()
        }

        Item {
            id: contentPane
            x: rail.implicitWidth
            width: parent.width - rail.implicitWidth
            height: parent.height
            clip: true


            Repeater {
                id: moduleRepeater
                model: root.configuredModules

                delegate: Loader {
                    id: moduleLoaderDelegate
                    required property string modelData
                    readonly property string moduleId: modelData
                    objectName: moduleId
                    readonly property var metadata: MenuCatalog.quickSettingsModule(moduleId)

                    width: contentPane.width
                    height: contentPane.height
                    active: root.moduleSeen[moduleId] === true
                    asynchronous: true
                    source: metadata ? "quicksettings/" + metadata.source : ""
                    z: root.activeModule === moduleId ? 2 : root.previousModule === moduleId ? 1 : 0
                    visible: status === Loader.Ready
                        && (root.activeModule === moduleId || root.previousModule === moduleId)
                    x: root.activeModule === moduleId ? 0
                        : root.previousModule === moduleId ? -contentPane.width / 3
                        : contentPane.width

                    Behavior on x {
                        enabled: root.open && root.navReady
                            && (root.activeModule === moduleLoaderDelegate.moduleId
                                || root.previousModule === moduleLoaderDelegate.moduleId)
                        NumberAnimation { duration: Motion.push; easing.type: Motion.pushCurve }
                    }

                    onStatusChanged: root.completePendingModule(moduleId, moduleLoaderDelegate)
                    Binding {
                        target: moduleLoaderDelegate.item
                        property: "s"
                        value: root.s
                        when: moduleLoaderDelegate.status === Loader.Ready
                    }
                    Binding {
                        target: moduleLoaderDelegate.item
                        property: "open"
                        value: ((root.open && root.settled) || root.warm) && root.page === ""
                            && root.activeModule === moduleLoaderDelegate.moduleId
                        when: moduleLoaderDelegate.status === Loader.Ready
                    }
                    onLoaded: {
                        item["navigate"] = pageId => root.showPage(pageId);
                        item["closePanel"] = () => root.requestClose();
                    }
                }
            }
        }
    }

    Item {
        id: pageBand
        width: parent.width
        height: parent.height
        x: root.page !== "" ? 0 : root.width
        visible: x < root.width - 0.5

        Behavior on x {
            enabled: root.open && root.navReady
            NumberAnimation { duration: Motion.push; easing.type: Motion.pushCurve }
        }


        // Opaque surface backing. The page band slides in over the module band
        // (which only shifts part-way off), so without this the modules behind
        // ghost through the page and look like they cut it off. The page owns its
        // own back-nav header, so covering the whole band is correct.
        Rectangle {
            anchors.fill: parent
            color: Theme.surface
        }

        Column {
            anchors.fill: parent
            spacing: 8

            Item {
                width: parent.width
                height: 42

                QsIconButton {
                    anchors.left: parent.left
                    anchors.leftMargin: 8
                    anchors.verticalCenter: parent.verticalCenter
                    icon: "arrow_back"
                    onClicked: root.showPage("")
                }
                Text {
                    anchors.centerIn: parent
                    text: root.pageTitle()
                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                    font.family: Theme.fontPrimary
                    font.pixelSize: Theme.fontMd
                    font.weight: Font.DemiBold
                }
            }

            Flickable {
                id: pageScroll
                width: parent.width
                height: parent.height - 50
                contentWidth: width
                contentHeight: pageStack.implicitHeight
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                interactive: contentHeight > height

                Column {
                    id: pageStack
                    width: parent.width

                    Loader {
                        id: networkPageLoader
                        width: pageStack.width
                        active: root.pageSeen["network"] === true
                        visible: root.page === "network" && status === Loader.Ready
                        asynchronous: true
                        onStatusChanged: root.completePendingPage("network", networkPageLoader)
                        sourceComponent: Component {
                            MenuNetwork {
                                pageMode: true
                                width: parent.width
                                s: root.s
                                open: root.open && root.page === "network"
                            }
                        }
                    }
                    Loader {
                        id: bluetoothPageLoader
                        width: pageStack.width
                        active: root.pageSeen["bluetooth"] === true
                        visible: root.page === "bluetooth" && status === Loader.Ready
                        asynchronous: true
                        onStatusChanged: root.completePendingPage("bluetooth", bluetoothPageLoader)
                        sourceComponent: Component {
                            MenuBluetooth {
                                pageMode: true
                                width: parent.width
                                s: root.s
                                open: root.open && root.page === "bluetooth"
                            }
                        }
                    }
                    Loader {
                        id: audioOutPageLoader
                        width: pageStack.width
                        active: root.pageSeen["audio-out"] === true
                        visible: root.page === "audio-out" && status === Loader.Ready
                        asynchronous: true
                        onStatusChanged: root.completePendingPage("audio-out", audioOutPageLoader)
                        sourceComponent: Component {
                            MenuAudioOutput {
                                pageMode: true
                                width: parent.width
                                s: root.s
                                open: root.open && root.page === "audio-out"
                            }
                        }
                    }
                    Loader {
                        id: audioInPageLoader
                        width: pageStack.width
                        active: root.pageSeen["audio-in"] === true
                        visible: root.page === "audio-in" && status === Loader.Ready
                        asynchronous: true
                        onStatusChanged: root.completePendingPage("audio-in", audioInPageLoader)
                        sourceComponent: Component {
                            MenuAudioInput {
                                pageMode: true
                                width: parent.width
                                s: root.s
                                open: root.open && root.page === "audio-in"
                            }
                        }
                    }
                    Loader {
                        id: themePageLoader
                        width: pageStack.width
                        active: root.pageSeen["theme"] === true
                        visible: root.page === "theme" && status === Loader.Ready
                        asynchronous: true
                        onStatusChanged: root.completePendingPage("theme", themePageLoader)
                        sourceComponent: Component {
                            MenuTheme {
                                width: parent.width
                                s: root.s
                                open: root.open && root.page === "theme"
                            }
                        }
                    }
                }
            }
        }
    }
}
