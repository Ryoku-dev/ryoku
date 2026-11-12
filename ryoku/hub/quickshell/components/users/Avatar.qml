import QtQuick
import Quickshell.Io
import Lithic

// a person's picture in a cut square, or their initial when they have none;
// looked up where login managers keep it, then ~/.face and ~/.face.icon
Item {
    id: root

    property string userName: ""
    property string fullName: ""
    property string home: ""
    // bump after the picture changed on disk
    property int revision: 0
    property string found: ""
    property int stamp: 0

    readonly property bool hasImage: root.found !== "" && image.status === Image.Ready
    readonly property var candidates: {
        var list = [];
        if (root.userName !== "")
            list.push("/var/lib/AccountsService/icons/" + root.userName);
        if (root.home !== "") {
            list.push(root.home + "/.face");
            list.push(root.home + "/.face.icon");
        }
        return list;
    }

    implicitWidth: LithicTokens.size.headerMark
    implicitHeight: LithicTokens.size.headerMark

    // the first readable candidate, so a missing file is never handed to Image
    function reload() {
        if (root.candidates.length === 0) {
            root.found = "";
            return;
        }
        if (look.running) {
            look.again = true;
            return;
        }
        look.command = ["sh", "-c", "for f in \"$@\"; do [ -r \"$f\" ] && [ -s \"$f\" ] && { printf '%s' \"$f\"; exit 0; }; done; exit 0", "sh"].concat(root.candidates);
        look.running = true;
    }

    onCandidatesChanged: root.reload()
    onRevisionChanged: root.reload()
    Component.onCompleted: root.reload()

    Process {
        id: look
        property bool again: false
        stdout: StdioCollector { id: lookOut }
        onExited: {
            root.found = lookOut.text.trim();
            root.stamp++;
            if (look.again) {
                look.again = false;
                root.reload();
            }
        }
    }

    FileView {
        path: root.userName !== "" ? "/var/lib/AccountsService/icons/" + root.userName : ""
        watchChanges: true
        printErrors: false
        onFileChanged: root.reload()
    }

    CutRect {
        anchors.fill: parent
        fill: Lithic.field
        strokeColor: Lithic.edge
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.small

        Label {
            anchors.centerIn: parent
            visible: !root.hasImage
            text: {
                var v = String(root.fullName || root.userName || "?").trim();
                return v.length > 0 ? v.charAt(0).toUpperCase() : "?";
            }
            typeRole: "title"
            inkRole: "fg2"
        }
    }

    // the picture clipped to the cut square: a 45 degree square cut down by the rectangle
    Item {
        anchors.fill: parent
        visible: root.hasImage
        clip: true

        Item {
            id: diamond
            readonly property real side: Math.max(0, (root.width + root.height - LithicTokens.cut.small * 2) / Math.SQRT2)
            width: side
            height: side
            x: (root.width - side) / 2
            y: (root.height - side) / 2
            rotation: 45
            clip: true

            Image {
                id: image
                x: (diamond.side - root.width) / 2
                y: (diamond.side - root.height) / 2
                width: root.width
                height: root.height
                rotation: -45
                asynchronous: true
                cache: false
                fillMode: Image.PreserveAspectCrop
                sourceSize: Qt.size(Math.max(1, root.width) * 2, Math.max(1, root.height) * 2)
                // the query only makes each reload a new url
                source: root.found !== "" ? "file://" + root.found + "?r=" + root.stamp : ""
            }
        }
    }
}
