pragma ComponentBehavior: Bound

import QtCore
import QtQuick
import QtQuick.Dialogs

// The system file or folder dialog, kept only as long as the window it opened
// in.
//
// QtQuick.Dialogs' non-native dialog parents its implementation to the window
// it first opens in and keeps a bare pointer to it. Quickshell destroys the
// QQuickWindow behind a layer-shell or popup window each time that window
// hides, which deletes the implementation while a dialog declared in a menu,
// a settings sheet or a bar island lives on in the proxy's item tree. The next
// open() then dereferences freed memory: on Qt 6.12 the shell segfaults in
// onShow, in setPopupType or in selectFile once a pick was made. Dropping the
// dialog whenever its window changes keeps the two lifetimes equal; the next
// open() builds one for the window that is there by then.
//
// Declare it where a FileDialog would go: `title`, `nameFilters` and
// `currentFolder` are the same, `open()` shows it, and `accepted` fires with
// the pick in `selectedFile` (or `selectedFolder` when `folders` is set).
Item {
    id: host

    visible: false
    width: 0
    height: 0

    property string title: ""
    property list<string> nameFilters: []
    property url currentFolder: StandardPaths.writableLocation(StandardPaths.HomeLocation)
    // Choose a folder instead of a file.
    property bool folders: false

    property url selectedFile: ""
    property url selectedFolder: ""
    readonly property bool showing: host.dialog !== null && host.dialog.visible

    signal accepted()
    signal rejected()

    property var dialog: null

    Window.onWindowChanged: host.drop()

    function drop(): void {
        if (!host.dialog)
            return;
        host.dialog.destroy();
        host.dialog = null;
    }

    // A dialog moves its own folder as the reader browses, which drops its binding
    // to the host's currentFolder, and a folder dialog keeps the folder it showed
    // last. So every open() builds a fresh dialog that starts at the host's folder,
    // and two rows sharing one host each open where they point.
    function open(): void {
        if (host.showing)
            return;
        host.drop();
        host.dialog = (host.folders ? host.folderComponent : host.fileComponent).createObject(host);
        host.dialog.open();
    }

    readonly property Component fileComponent: FileDialog {
        id: fileDlg
        title: host.title
        nameFilters: host.nameFilters
        currentFolder: host.currentFolder
        onAccepted: {
            host.selectedFile = fileDlg.selectedFile;
            host.accepted();
        }
        onRejected: host.rejected()
    }

    readonly property Component folderComponent: FolderDialog {
        id: folderDlg
        title: host.title
        currentFolder: host.currentFolder
        onAccepted: {
            host.selectedFolder = folderDlg.selectedFolder;
            host.accepted();
        }
        onRejected: host.rejected()
    }
}
