pragma ComponentBehavior: Bound

import QtQuick
import "../kit" as K

Item {
    id: host

    required property var scene
    property string bubbleKey: ""
    property bool shown: bubbleKey !== ""
    property bool failed: false
    readonly property bool hintsSuppressed: scene.cardKey !== "" || scene.settingsOpen

    readonly property bool ready: bubbleLoader.status === Loader.Ready
        && bubbleLoader.item !== null
    readonly property real naturalWidth: ready
        ? Math.max(K.Style.barH, Number(bubbleLoader.item.implicitWidth) || 0)
        : K.Style.barH

    implicitWidth: naturalWidth
    implicitHeight: K.Style.barH
    width: naturalWidth
    height: K.Style.barH
    visible: shown && bubbleKey !== "" && !failed

    function sourceFor(key) {
        if (!key)
            return "";
        const name = key.charAt(0).toUpperCase() + key.slice(1);
        return Qt.resolvedUrl("../voice/" + name + "Bubble.qml");
    }

    function load() {
        host.failed = false;
        const source = sourceFor(host.bubbleKey);
        if (source.toString() === "") {
            bubbleLoader.source = "";
            return;
        }
        bubbleLoader.setSource(source, { scene: host.scene });
    }

    onBubbleKeyChanged: load()
    Component.onCompleted: load()

    Loader {
        id: bubbleLoader
        anchors.fill: parent
        visible: host.ready
        onStatusChanged: if (status === Loader.Error) host.failed = true
    }
}
