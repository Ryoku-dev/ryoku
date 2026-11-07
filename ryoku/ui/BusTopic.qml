import QtQuick
import "Singletons"

// One daemon topic as a QML object. `data` is the latest frame (shared with
// every other reader in the process; never mutate it) and `info` describes it
// ({rev, paths} for state, {seq, dropped, event} for an event). `updated`
// fires for each frame or event, including at once for a frame the process
// already has. It carries no arguments on purpose: a QML `var` signal
// argument is deep-copied into a variant, which costs milliseconds on a large
// frame and turns arrays into sequences that fail Array.isArray. Read `data`
// and `info` in the handler. active: false unsubscribes.
QtObject {
    id: topicView

    property string topic: ""
    property bool active: true
    readonly property var data: topicView._data
    readonly property var info: topicView._info

    signal updated()

    property var _data: null
    property var _info: null
    property int _token: 0
    property bool _complete: false

    function _deliver(data, info) {
        topicView._data = data;
        topicView._info = info;
        topicView.updated();
    }

    function _sync() {
        if (topicView._token !== 0) {
            Bus.unsubscribe(topicView._token);
            topicView._token = 0;
        }
        if (topicView._complete && topicView.active && topicView.topic !== "")
            topicView._token = Bus.subscribe(topicView.topic, topicView._deliver);
    }

    onActiveChanged: topicView._sync()
    onTopicChanged: {
        topicView._data = null;
        topicView._info = null;
        topicView._sync();
    }
    Component.onCompleted: {
        topicView._complete = true;
        topicView._sync();
    }
    Component.onDestruction: {
        if (topicView._token !== 0)
            Bus.unsubscribe(topicView._token);
    }
}
