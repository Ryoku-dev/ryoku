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
    }
}
