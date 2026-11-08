import QtQuick
import Ryoku.Bus

QtObject {
    id: topicView

    property string topic: ""
    property bool active: true
    // shared with every reader in the process, never mutate it
    readonly property var data: topicView._data
    readonly property var info: topicView._info

    // no args on purpose, a var signal arg gets deep-copied and arrays stop passing Array.isArray
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
            BusClient.unsubscribe(topicView._token);
            topicView._token = 0;
        }
        if (topicView._complete && topicView.active && topicView.topic !== "")
            topicView._token = BusClient.subscribe(topicView.topic, topicView._deliver);
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
            BusClient.unsubscribe(topicView._token);
    }
}
