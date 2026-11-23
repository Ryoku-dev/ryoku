pragma ComponentBehavior: Bound

import QtQuick
import "../kit" as K
import "../Metrics.js" as M

Item {
    id: host

    required property var scene
    required property var body
    property string cardKey: ""
    property var cardArgs: ({})
    property string retainedKey: ""
    property var retainedArgs: ({})
    property var activeLoader: null
    property var pendingLoader: null

    readonly property bool ready: activeLoader !== null
        && activeLoader.status === Loader.Ready && activeLoader.item !== null
    // the width ceiling follows the retained card, so a closing page keeps its size
    readonly property real maxW: host.maxWidthFor(retainedKey)
    readonly property real cardW: ready
        ? Math.min(host.maxWidthFor(activeLoader.loadedKey), Math.max(0,
            Number(activeLoader.item.implicitWidth) || 0)) : 0
    readonly property real cardH: ready
        ? Math.min(scene.cardMaxHeightFor(activeLoader.loadedKey), Math.max(0,
            Number(activeLoader.item.implicitHeight) || 0)) : 0
    readonly property var launcherSheet: ready && activeLoader.loadedKey === "launcher"
        && activeLoader.item.sheet ? activeLoader.item.sheet : null
    readonly property real reveal: Math.max(0, Math.min(1,
        (body.cardProgress - 0.35) / 0.25))
    readonly property bool contentShown: cardKey !== ""
        && body.cardProgress >= 0.35
    readonly property bool hovered: cardHover.hovered

    x: body.cardX
    y: K.Style.barH
    width: body.cardCurW
    height: Math.max(0, body.cardY + body.cardCurH - y)
    implicitWidth: cardW
    implicitHeight: cardH
    clip: true
    visible: retainedKey !== "" && body.cardProgress > 0.001

    // The content stretches with the welded blob while the page travels, so a
    // move reads as liquid, not a rigid slide (the frame popout's deform).
    transform: Matrix4x4 { matrix: host.body.cardDeform }

    function maxWidthFor(key) {
        return key === "launcher" ? M.launcherMaxW * K.Style.scale : M.cardMaxW;
    }

    function sourceFor(key) {
        switch (key) {
        case "media": return Qt.resolvedUrl("../cards/MediaCard.qml");
        case "today": return Qt.resolvedUrl("../cards/TodayCard.qml");
        case "weather": return Qt.resolvedUrl("../cards/WeatherCard.qml");
        case "controls": return Qt.resolvedUrl("../cards/ControlsCard.qml");
        case "notifications": return Qt.resolvedUrl("../cards/NotificationsCard.qml");
        case "tray": return Qt.resolvedUrl("../cards/TrayCard.qml");
        case "power": return Qt.resolvedUrl("../cards/PowerCard.qml");
        case "system": return Qt.resolvedUrl("../cards/SystemCard.qml");
        case "session": return Qt.resolvedUrl("../cards/SessionCard.qml");
        case "recording": return Qt.resolvedUrl("../cards/RecordingCard.qml");
        case "vpn": return Qt.resolvedUrl("../cards/VpnCard.qml");
        case "launcher": return Qt.resolvedUrl("../cards/LauncherCard.qml");
        default: return "";
        }
    }

    function inactiveLoader() {
        return host.activeLoader === firstLoader ? secondLoader : firstLoader;
    }

    // The launcher card stays built between opens while the launcher is warm,
    // so Super+Space reopens it without building it again.
    function keptLauncher(loader) {
        return host.scene.launcherWarm === true && loader.loadedKey === "launcher"
            && loader.status === Loader.Ready && loader.item !== null;
    }

    function freeLoader() {
        return host.keptLauncher(firstLoader) ? secondLoader : firstLoader;
    }

    function loadCard() {
        const source = host.sourceFor(host.retainedKey);
        if (source.toString() === "") {
            Qt.callLater(host.scene.closeCard);
            return;
        }
        if (host.retainedKey === "launcher") {
            const kept = host.keptLauncher(firstLoader) ? firstLoader
                : host.keptLauncher(secondLoader) ? secondLoader : null;
            if (kept !== null) {
                kept.loadedArgs = host.retainedArgs;
                host.pendingLoader = kept;
                host.activate(kept);
                kept.item.prepare();
                return;
            }
        }
        const loader = host.activeLoader === null ? host.freeLoader() : host.inactiveLoader();
        loader.selected = false;
        loader.loadedKey = host.retainedKey;
        loader.loadedArgs = host.retainedArgs;
        host.pendingLoader = loader;
        loader.setSource(source, {
            scene: host.scene,
            shown: false,
            args: host.retainedArgs
        });
        if (loader.status === Loader.Ready)
            host.activate(loader);
    }

    function activate(loader) {
        if (host.pendingLoader !== loader || loader.loadedKey !== host.retainedKey)
            return;
        const previous = host.activeLoader;
        host.activeLoader = loader;
        host.pendingLoader = null;
        loader.selected = true;
        if (previous && previous !== loader)
            previous.selected = false;
    }

    function releaseCards() {
        host.retainedKey = "";
        host.retainedArgs = ({});
        host.activeLoader = null;
        host.pendingLoader = null;
        firstLoader.selected = false;
        secondLoader.selected = false;
        if (!host.keptLauncher(firstLoader))
            host.unload(firstLoader);
        if (!host.keptLauncher(secondLoader))
            host.unload(secondLoader);
    }

    function unload(loader) {
        loader.source = "";
        loader.loadedKey = "";
    }

    // the launcher host went away, so a launcher card kept for it goes too
    function dropKeptLauncher() {
        for (const loader of [firstLoader, secondLoader]) {
            if (loader.loadedKey === "launcher" && loader !== host.activeLoader
                    && loader !== host.pendingLoader)
                host.unload(loader);
        }
    }

    onCardKeyChanged: {
        if (cardKey === "")
            return;
        retainedKey = cardKey;
        retainedArgs = cardArgs || ({});
        loadCard();
    }

    onCardArgsChanged: {
        if (cardKey === "")
            return;
        retainedArgs = cardArgs || ({});
        if (activeLoader)
            activeLoader.loadedArgs = retainedArgs;
    }

    Connections {
        target: host.body
        function onCardProgressChanged() {
            if (host.body.cardProgress <= 0.001 && host.cardKey === "")
                host.releaseCards();
        }
    }

    Connections {
        target: host.scene
        function onLauncherWarmChanged() {
            if (!host.scene.launcherWarm)
                host.dropKeptLauncher();
        }
    }

    Loader {
        id: firstLoader
        property string loadedKey: ""
        property var loadedArgs: ({})
        property bool selected: false
        readonly property real pageW: status === Loader.Ready && item
            ? Math.min(host.maxWidthFor(loadedKey),
                Math.max(0, Number(item.implicitWidth) || 0)) : 0
        readonly property real pageH: status === Loader.Ready && item
            ? Math.min(host.scene.cardMaxHeightFor(loadedKey),
                Math.max(0, Number(item.implicitHeight) || 0)) : 0

        x: Math.round((host.width - width) / 2)
        y: 0
        width: pageW
        height: pageH
        opacity: selected ? host.reveal : 0
        // the launcher stays visible while it fades in, so its field can take focus
        visible: status === Loader.Ready
            && (opacity > 0.001 || (selected && loadedKey === "launcher"))

        Behavior on opacity {
            NumberAnimation {
                duration: firstLoader.selected ? K.Style.ms(200) : K.Style.ms(90)
                easing.type: Easing.OutCubic
            }
        }

        onStatusChanged: {
            if (status === Loader.Ready)
                host.activate(firstLoader);
            else if (status === Loader.Error && host.pendingLoader === firstLoader)
                Qt.callLater(host.scene.closeCard);
        }
    }

    Loader {
        id: secondLoader
        property string loadedKey: ""
        property var loadedArgs: ({})
        property bool selected: false
        readonly property real pageW: status === Loader.Ready && item
            ? Math.min(host.maxWidthFor(loadedKey),
                Math.max(0, Number(item.implicitWidth) || 0)) : 0
        readonly property real pageH: status === Loader.Ready && item
            ? Math.min(host.scene.cardMaxHeightFor(loadedKey),
                Math.max(0, Number(item.implicitHeight) || 0)) : 0

        x: Math.round((host.width - width) / 2)
        y: 0
        width: pageW
        height: pageH
        opacity: selected ? host.reveal : 0
        visible: status === Loader.Ready
            && (opacity > 0.001 || (selected && loadedKey === "launcher"))

        Behavior on opacity {
            NumberAnimation {
                duration: secondLoader.selected ? K.Style.ms(200) : K.Style.ms(90)
                easing.type: Easing.OutCubic
            }
        }

        onStatusChanged: {
            if (status === Loader.Ready)
                host.activate(secondLoader);
            else if (status === Loader.Error && host.pendingLoader === secondLoader)
                Qt.callLater(host.scene.closeCard);
        }
    }

    HoverHandler {
        id: cardHover
        enabled: host.visible
    }

    Binding {
        target: firstLoader.item
        property: "scene"
        value: host.scene
        when: firstLoader.status === Loader.Ready && firstLoader.item !== null
        restoreMode: Binding.RestoreNone
    }

    Binding {
        target: firstLoader.item
        property: "shown"
        value: firstLoader.selected && host.contentShown
        when: firstLoader.status === Loader.Ready && firstLoader.item !== null
        restoreMode: Binding.RestoreNone
    }

    Binding {
        target: firstLoader.item
        property: "args"
        value: firstLoader.loadedArgs
        when: firstLoader.status === Loader.Ready && firstLoader.item !== null
        restoreMode: Binding.RestoreNone
    }

    Binding {
        target: secondLoader.item
        property: "scene"
        value: host.scene
        when: secondLoader.status === Loader.Ready && secondLoader.item !== null
        restoreMode: Binding.RestoreNone
    }

    Binding {
        target: secondLoader.item
        property: "shown"
        value: secondLoader.selected && host.contentShown
        when: secondLoader.status === Loader.Ready && secondLoader.item !== null
        restoreMode: Binding.RestoreNone
    }

    Binding {
        target: secondLoader.item
        property: "args"
        value: secondLoader.loadedArgs
        when: secondLoader.status === Loader.Ready && secondLoader.item !== null
        restoreMode: Binding.RestoreNone
    }
}
