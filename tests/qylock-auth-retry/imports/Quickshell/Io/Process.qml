import QtQuick
QtObject {
    property bool running: false
    property var command: []
    property var stdout: null
    property var stderr: null
    signal exited(int exitCode, int exitStatus)
}
