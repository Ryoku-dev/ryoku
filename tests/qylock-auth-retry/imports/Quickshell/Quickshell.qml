pragma Singleton
import QtQuick
QtObject {
    readonly property string shellDir: ""
    property string faceLock: ""
    function env(name) {
        if (name === "RYOKU_QYLOCK_FACELOCK") return faceLock
        return name === "USER" ? "test-user" : ""
    }
    function execDetached(command) {}
}
