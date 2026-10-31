pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell.Io

// A person's profile picture, clipped to a circle. It looks for the picture in
// the places the desktop stack keeps it, first hit wins: the AccountsService
// icon (what login managers and GNOME or KDE settings write), then ~/.face and
// ~/.face.icon for a system without the accounts daemon. `hasImage` is false
// when none loads, so the caller keeps drawing its letter in that case.
Item {
    id: root

    property string userName: ""
    property string home: ""
    // Bump after the picture changed on disk to load it again.
    property int revision: 0

    readonly property bool hasImage: image.status === Image.Ready
    readonly property var candidates: {
        const list = [];
        if (root.userName !== "")
            list.push("/var/lib/AccountsService/icons/" + root.userName);
        if (root.home !== "") {
            list.push(root.home + "/.face");
            list.push(root.home + "/.face.icon");
        }
        return list;
    }

    property int index: 0
    property int stamp: 0

    function reload() {
        root.index = 0;
        root.stamp++;
    }

    onCandidatesChanged: root.reload()
    onRevisionChanged: root.reload()
    onVisibleChanged: if (visible) root.reload()
    FileView {
        path: root.userName !== "" ? "/var/lib/AccountsService/icons/" + root.userName : ""
        watchChanges: true
        printErrors: false
        onFileChanged: root.reload()
    }

    Image {
        id: image
        anchors.fill: parent
        visible: false
        layer.enabled: true
        asynchronous: true
        cache: false
        fillMode: Image.PreserveAspectCrop
        sourceSize: Qt.size(Math.max(1, root.width) * 2, Math.max(1, root.height) * 2)
        // The query is ignored for a local file; it only makes each reload a new URL.
        source: root.index < root.candidates.length
            ? "file://" + root.candidates[root.index] + "?r=" + root.stamp : ""
        onStatusChanged: if (status === Image.Error) root.index++
    }

    Rectangle {
        id: mask
        anchors.fill: parent
        radius: width / 2
        visible: false
        layer.enabled: true
    }

    MultiEffect {
        anchors.fill: parent
        source: image
        maskEnabled: true
        maskSource: mask
        visible: root.hasImage
    }
}
