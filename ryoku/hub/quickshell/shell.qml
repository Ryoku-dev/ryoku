//@ pragma Env QML_XHR_ALLOW_FILE_READ=1
import Quickshell

// Ryoku Settings entry point; `qs -c hub` loads this. Quickshell reads pragmas from the
// root file only, so they stay here; the body loads by file URL, because Qt's disk cache
// skips qs: URLs and everything imported relative to them
ShellRoot {
    LazyLoader {
        active: true
        source: "file://" + Quickshell.shellDir + "/ShellBody.qml"
    }
}
