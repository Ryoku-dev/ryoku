pragma Singleton
import QtQuick
QtObject {
    property var contexts: []
    function add(context) { contexts = contexts.concat([context]) }
    function find(config) {
        for (let i = contexts.length - 1; i >= 0; --i) {
            if (contexts[i] && contexts[i].config === config)
                return contexts[i]
        }
        return null
    }
}