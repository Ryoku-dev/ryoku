import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import ".."
import "../Singletons"
import "../components"
import "../schema/notifications.js" as Schema
import "../ShellIndex.js" as ShellIndex

// the notification card and pop-up designs, and what notifications show
Item {
    id: root

    required property var hub

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    // the bar style in use may draw these overlays itself
    readonly property var styleItem: {
        var id = String(Settings.get("barStyle") || "") || shells.index.fallback;
        return ShellIndex.item(shells.index, id);
    }
    function styleOwns(surface) {
        var item = root.styleItem;
        if (!item || !Array.isArray(item.owns) || item.owns.indexOf(surface) < 0)
            return false;
        var key = String((item.ownsSetting || {})[surface] || "");
        return key === "" || Settings.get(key) !== false;
    }
    function ownerNote(surface) {
        if (!root.styleOwns(surface))
            return "";
        var name = root.styleItem.name || root.styleItem.id;
        return surface === "osd"
            ? I18n.tr("%1 draws its own pop-ups, so the design here shows when you use another bar style.").arg(name)
            : I18n.tr("%1 draws its own notifications, so the design here shows when you use another bar style.").arg(name);
    }

    readonly property string stateLine: I18n.tr("Notifications: %1. Pop-ups: %2.")
        .arg(notifications.activeCard.name).arg(popups.activeCard.name)

    readonly property var life: ({ alive: true })
    Component.onDestruction: root.life.alive = false

    // what went wrong with the last test, shown under its section with Retry
    property string sendError: ""
    property string showError: ""

    // a missing notify-send or one that hangs ends in plain words, never in silence
    function sendTest() {
        root.sendError = "";
        var life = root.life;
        Changes.run(["notify-send", "-a", I18n.tr("Ryoku"), I18n.tr("Ryoku notification"), I18n.tr("This is how your notifications look.")],
            function (ok, out, err, code) {
                if (!life.alive || ok)
                    return;
                var why = err.trim().replace(/[.\s]+$/, "");
                root.sendError = code === -1 && why !== ""
                    ? I18n.tr("Could not send the test notification. %1.").arg(why)
                    : I18n.tr("Could not send the test notification. Check that your desktop is running, then try again.");
            }, { timeout: 10000 });
    }

    // the desktop draws the pop-up; a desktop that refuses or never answers says so
    function showTest() {
        root.showError = "";
        var life = root.life;
        var ask = { id: 0, over: false };
        ask.id = Bus.ipc("osd", "preview", [], function (ok) {
            if (ask.over)
                return;
            ask.over = true;
            if (life.alive && !ok)
                root.showError = I18n.tr("Could not show the test pop-up. Check that your desktop is running, then try again.");
        });
        previewWait.ask = ask;
        previewWait.restart();
    }
    Timer {
        id: previewWait
        property var ask: null
        interval: 10000
        onTriggered: {
            if (!previewWait.ask || previewWait.ask.over)
                return;
            previewWait.ask.over = true;
            Bus.cancel(previewWait.ask.id);
            root.showError = I18n.tr("The desktop did not answer, so the test pop-up did not show. Check that it is running, then try again.");
        }
    }

    ShellIndexFile { id: shells }

    DesignShelf {
        id: notifications
        category: "notifications"
        settingKey: "designs.notification"
        changeLabel: "Notification design"
        pageId: "notifications"
    }

    DesignShelf {
        id: popups
        category: "osds"
        settingKey: "designs.osd"
        changeLabel: "Pop-up design"
        pageId: "notifications"
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "notifications"
        schema: Schema
        stateLine: root.stateLine
        bodies: ({ design: notificationGallery, popups: popupGallery })
        footers: ({ design: root.sendError !== "" ? sendTrouble : null, popups: root.showError !== "" ? showTrouble : null })
        controls: ({ "test-notification": tryNotification, "test-popup": tryPopup })
    }

    // a failed test with Retry; the note goes when the test works, so Retry first hands focus to the stop before it
    component TestTrouble: Row {
        id: note
        property string text: ""
        signal retry()
        width: parent ? parent.width : implicitWidth
        spacing: LithicTokens.space.s3
        // the note lands under the section's last row, often out of view after a press higher up
        Component.onCompleted: Qt.callLater(() => {
            var y = note.mapToItem(page.flickable, 0, 0).y;
            if (y < 0 || y + note.height > page.flickable.height)
                page.reveal(note);
        })

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, note.width - again.width - LithicTokens.space.s3)
            text: note.text
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: again
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: {
                var back = again.activeFocus ? again.nextItemInFocusChain(false) : null;
                var reason = again.visualFocus ? Qt.TabFocusReason : Qt.OtherFocusReason;
                if (back)
                    back.forceActiveFocus(reason);
                note.retry();
            }
        }
    }

    Component {
        id: sendTrouble
        TestTrouble {
            text: root.sendError
            onRetry: root.sendTest()
        }
    }

    Component {
        id: showTrouble
        TestTrouble {
            text: root.showError
            onRetry: root.showTest()
        }
    }

    Component {
        id: notificationGallery
        DesignChooser {
            shelf: notifications
            accessibleName: I18n.tr("Notification designs")
            ownerNote: root.ownerNote("notifications")
        }
    }

    Component {
        id: popupGallery
        DesignChooser {
            shelf: popups
            accessibleName: I18n.tr("Pop-up designs")
            ownerNote: root.ownerNote("osd")
        }
    }

    Component {
        id: tryNotification
        Plaque {
            property var row
            function activate() {
                root.sendTest();
            }
            kind: "quiet"
            small: true
            glyph: "notifications"
            text: I18n.tr("Send")
            accessibleName: I18n.tr("Send a test notification")
            onClicked: activate()
        }
    }

    Component {
        id: tryPopup
        Plaque {
            property var row
            function activate() {
                root.showTest();
            }
            kind: "quiet"
            small: true
            glyph: "volume_up"
            text: I18n.tr("Show")
            accessibleName: I18n.tr("Show a test pop-up")
            onClicked: activate()
        }
    }
}
