import QtQuick
import QtTest
import "../../ryoku/ui"

// Every open() starts at the host's currentFolder, even after an earlier
// dialog was moved elsewhere: pages share one host between several rows.
Item {
    id: root
    width: 800
    height: 600

    FileDialogHost {
        id: folders
        folders: true
        currentFolder: "file:///tmp"
    }

    FileDialogHost {
        id: files
        currentFolder: "file:///tmp"
    }

    TestCase {
        name: "FileDialogHost"
        when: windowShown

        function cycle(host) {
            host.open();
            verify(host.dialog !== null, "open() builds a dialog");
            compare(String(host.dialog.currentFolder), "file:///tmp");
            // the dialog moves itself as the reader browses, which drops its binding
            host.dialog.currentFolder = "file:///usr";
            host.dialog.close();
            host.currentFolder = "file:///var";
            host.open();
            compare(String(host.dialog.currentFolder), "file:///var", "the next open starts at the host's folder");
            host.dialog.close();
            host.currentFolder = "file:///tmp";
            host.open();
            compare(String(host.dialog.currentFolder), "file:///tmp", "and again after the host moves back");
            host.dialog.close();
        }

        function test_folderDialogStartsAtTheHostsFolder() {
            cycle(folders);
        }

        function test_fileDialogStartsAtTheHostsFolder() {
            cycle(files);
        }
    }
}
