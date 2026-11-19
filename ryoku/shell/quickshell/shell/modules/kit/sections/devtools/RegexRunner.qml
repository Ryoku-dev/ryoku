import QtQuick
import Quickshell
import "devtools.js" as DevTools

ShellRoot {
    Component.onCompleted: Quickshell.watchFiles = false

    Timer {
        interval: 0
        running: true
        onTriggered: {
            let result;
            try {
                const request = JSON.parse(Quickshell.env("RYOKU_KIT_REGEX_REQUEST") || "{}");
                result = DevTools.runTool("regex_tester", request.input ?? "", request.options ?? {});
            } catch (error) {
                result = { output: "", error: String(error.message || error) };
            }
            console.log("RYOKU_KIT_REGEX_RESULT:" + JSON.stringify(result));
            Qt.quit();
        }
    }
}
