import QtQuick
import QtTest
import Lithic

Item {
    id: host
    width: 600
    height: 300
    property int hostReturns: 0
    Keys.onReturnPressed: host.hostReturns++

    Column {
        width: parent.width
        SettingRow {
            id: plain
            width: parent.width
            title: "Name"
            TextField {
                id: field
                width: 200
            }
        }
        SettingRow {
            id: acting
            width: parent.width
            title: "Night light"
            activatable: true
        }
        SettingRow {
            id: narrowChanged
            width: 200
            title: "Temperature units for the weather"
            changed: true
        }
        SettingRow {
            id: wideChanged
            width: parent.width
            title: "Temperature units"
            changed: true
        }
        SettingRow {
            id: focusedCard
            width: parent.width
            title: "Pomodoro"
            activatable: false
            focusPolicy: Qt.StrongFocus
        }
        SettingRow {
            id: resettable
            width: parent.width
            title: "Brightness"
            changed: true
            onResetRequested: changed = false
            TextField {
                id: resetField
                width: 120
            }
        }
    }

    SignalSpy {
        id: actingSpy
        target: acting
        signalName: "activated"
    }

    TestCase {
        name: "RowReturn"
        when: windowShown

        function test_fieldReturnReachesHost() {
            field.forceActiveFocus();
            host.hostReturns = 0;
            keyClick(Qt.Key_Return);
            compare(host.hostReturns, 1, "Return in a field inside a plain row reaches the host");
        }

        function test_actingRowStillActivates() {
            acting.forceActiveFocus();
            actingSpy.clear();
            host.hostReturns = 0;
            keyClick(Qt.Key_Return);
            compare(actingSpy.count, 1);
            compare(host.hostReturns, 0, "an acting row keeps its own Return");
        }

        function labelWithText(item, text) {
            if (item.text === text && item.lineCount !== undefined)
                return item;
            for (let index = 0; index < item.children.length; ++index) {
                const found = labelWithText(item.children[index], text);
                if (found)
                    return found;
            }
            return null;
        }

        function test_changedTagFollowsAWholeTitle() {
            const title = labelWithText(narrowChanged, narrowChanged.title);
            const tag = labelWithText(narrowChanged, "Changed");
            verify(title.lineCount > 1, "the narrow title wraps");
            verify(tag.y >= title.y + title.height - 0.5, "a wrapped title keeps the tag below it, not mid-label");
            compare(tag.x, title.x);

            const wideTitle = labelWithText(wideChanged, wideChanged.title);
            const wideTag = labelWithText(wideChanged, "Changed");
            compare(wideTitle.lineCount, 1);
            fuzzyCompare(wideTag.y + wideTag.baselineOffset, wideTitle.y + wideTitle.baselineOffset, 0.5,
                         "a title that fits keeps the tag on its baseline");
            verify(wideTag.x > wideTitle.x + wideTitle.contentWidth, "after the title");
        }

        function ringShows(item) {
            if (item.keyboardFocusVisible === true)
                return true;
            for (let index = 0; index < item.children.length; ++index) {
                if (ringShows(item.children[index]))
                    return true;
            }
            return false;
        }

        function test_aRowAPageMakesFocusableShowsTheRing() {
            focusedCard.forceActiveFocus(Qt.TabFocusReason);
            verify(ringShows(focusedCard), "keyboard focus on a row a page made focusable shows the ring");
        }

        function named(item, name) {
            if (item.accessibleName === name)
                return item;
            for (let index = 0; index < item.children.length; ++index) {
                const found = named(item.children[index], name);
                if (found)
                    return found;
            }
            return null;
        }

        function test_aResetFromTheKeysMovesFocusToTheRowControl() {
            resettable.changed = true;
            const reset = named(resettable, "Reset Brightness");
            verify(reset !== null, "a changed row offers its reset");
            reset.forceActiveFocus(Qt.TabFocusReason);
            keyClick(Qt.Key_Return);
            compare(resettable.changed, false, "Return on the reset resets the row");
            verify(!reset.activeFocus, "the keys leave the hidden reset button");
            verify(resetField.activeFocus, "the keys land on the row's own control");
            compare(resetField.focusReason, Qt.TabFocusReason, "with the keyboard mark");
        }
    }
}
