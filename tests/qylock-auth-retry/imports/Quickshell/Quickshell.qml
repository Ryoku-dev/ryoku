pragma Singleton
import QtQuick
QtObject {
    readonly property string shellDir: ""
    function env(name) { return name === "USER" ? "test-user" : "" }
    function execDetached(command) {}
}
