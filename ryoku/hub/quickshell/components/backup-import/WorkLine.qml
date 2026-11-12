import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a long job's state under its row: nothing for the first second, then a
// working mark, then past ten seconds a progress blade with Cancel; an error
// says what happened and offers Retry
Column {
    id: root

    property bool active: false
    property string text: ""
    property bool cancellable: false
    property string error: ""

    signal cancelled()
    signal retried()

    property bool slow: false
    property bool lengthy: false
    // the control whose job this is: Retry and Cancel hide as they are pressed, so the keys
    // go back to it rather than falling to the page
    property Item origin: null
    // the last control outside this line the keys were on, and whether the keyboard put them there
    property Item lastStop: null
    property bool lastKeyed: false
    // a Cancel pressed while the control is still off waits for the job to end to hand back
    property int handReason: -1

    function holds(item) {
        for (var it = item; it; it = it.parent)
            if (it === root)
                return true;
        return false;
    }
    function contains(outer, inner) {
        for (var it = inner; it; it = it.parent)
            if (it === outer)
                return true;
        return false;
    }
    function noteOrigin() {
        if (root.lastStop)
            root.origin = root.lastStop;
    }
    function handBack(button) {
        if (!button.activeFocus)
            return;
        // pressed under the pointer: no mark, even when the keys had put the focus there first
        var reason = button.visualFocus && !button.hovered ? Qt.TabFocusReason : Qt.OtherFocusReason;
        var o = root.origin;
        if (o && o.visible && o.enabled)
            o.forceActiveFocus(reason);
        else
            root.handReason = reason;
    }
    function finishHand() {
        var reason = root.handReason;
        root.handReason = -1;
        var o = root.origin;
        var f = root.Window.activeFocusItem;
        if (reason >= 0 && o && o.visible && o.enabled && !(f && f.activeFocusOnTab && f.visible))
            o.forceActiveFocus(reason);
    }
    // the control that started the job turned off and left the keys on the item holding it:
    // Cancel, once shown, takes them, so the keys can stop the work, and hands them back after
    function offStop(item, depth) {
        if (!item || depth > 6)
            return null;
        if (item.activeFocusOnTab && !item.enabled && item.visible)
            return item;
        var kids = item.children || [];
        for (var i = 0; i < kids.length; i++) {
            var hit = root.offStop(kids[i], depth + 1);
            if (hit)
                return hit;
        }
        return null;
    }
    function offerCancel() {
        var f = root.Window.activeFocusItem;
        if (!cancel.visible || !f || f.activeFocusOnTab || root.holds(f))
            return;
        var starter = root.offStop(f, 0);
        if (!starter)
            return;
        root.origin = starter;
        cancel.forceActiveFocus(root.lastKeyed ? Qt.TabFocusReason : Qt.OtherFocusReason);
    }

    Connections {
        target: root.Window.window
        function onActiveFocusItemChanged() {
            var f = root.Window.activeFocusItem;
            if (f && f.activeFocusOnTab && !root.holds(f)) {
                root.lastStop = f;
                root.lastKeyed = f.visualFocus === true;
            }
        }
    }

    spacing: LithicTokens.space.s2
    visible: (root.active && root.slow) || root.error.length > 0

    onErrorChanged: {
        if (root.error.length > 0)
            root.noteOrigin();
    }
    onActiveChanged: {
        root.slow = false;
        root.lengthy = false;
        if (root.active) {
            root.noteOrigin();
            slowTimer.restart();
            longTimer.restart();
        } else {
            slowTimer.stop();
            longTimer.stop();
            if (root.handReason >= 0)
                Qt.callLater(root.finishHand);
        }
    }

    Timer { id: slowTimer; interval: 1000; onTriggered: root.slow = true }
    Timer {
        id: longTimer
        interval: 10000
        onTriggered: {
            root.lengthy = true;
            Qt.callLater(root.offerCancel);
        }
    }

    Row {
        visible: root.active && root.slow
        spacing: LithicTokens.space.s2
        Working {
            anchors.verticalCenter: parent.verticalCenter
            active: root.active && root.slow
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: root.text
            typeRole: "detail"
        }
    }

    Row {
        visible: root.active && root.lengthy
        width: parent.width
        spacing: LithicTokens.space.s3
        BladeProgress {
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width - (cancel.visible ? cancel.width + parent.spacing : 0)
            active: root.active && root.lengthy
            indeterminate: true
        }
        Plaque {
            id: cancel
            visible: root.cancellable
            kind: "quiet"
            small: true
            text: I18n.tr("Cancel")
            onClicked: {
                root.handBack(cancel);
                root.cancelled();
            }
        }
    }

    Row {
        visible: root.error.length > 0 && !root.active
        width: parent.width
        spacing: LithicTokens.space.s3
        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - retry.width - parent.spacing)
            text: root.error
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: {
                root.handBack(retry);
                root.retried();
            }
        }
    }
}
