pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui.Singletons

Rectangle {
    id: root

    property var cpuValues: []
    property var ramValues: []
    property var diskValues: []
    property var netValues: []
    property int samplePeriodSeconds: 5

    function humanRate(value) {
        value = Number(value) || 0;
        var units = ["B/s", "KB/s", "MB/s", "GB/s"];
        var index = 0;
        while (value >= 1024 && index < units.length - 1) {
            value /= 1024;
            index++;
        }
        return (value >= 100 ? value.toFixed(0) : value.toFixed(1)) + " " + units[index];
    }

    implicitHeight: 300
    color: "transparent"
    border.width: Tokens.border
    border.color: Tokens.line
    antialiasing: false

    Grid {
        anchors.fill: parent
        anchors.margins: 1
        columns: 2

        Item {
            width: root.width / 2 - 1
            height: (root.height - 2) / 2
            MetricGraph {
                anchors.fill: parent
                anchors.margins: Tokens.s3
                title: I18n.tr("CPU usage")
                values: root.cpuValues
                fixedMax: 100
                unit: "%"
                samplePeriodSeconds: root.samplePeriodSeconds
            }
            Rectangle { anchors.right: parent.right; width: 1; height: parent.height; color: Tokens.lineSoft }
            Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: Tokens.lineSoft }
        }
        Item {
            width: root.width / 2 - 1
            height: (root.height - 2) / 2
            MetricGraph {
                anchors.fill: parent
                anchors.margins: Tokens.s3
                title: I18n.tr("Memory usage")
                values: root.ramValues
                fixedMax: 100
                unit: "%"
                samplePeriodSeconds: root.samplePeriodSeconds
            }
            Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: Tokens.lineSoft }
        }
        Item {
            width: root.width / 2 - 1
            height: (root.height - 2) / 2
            MetricGraph {
                id: disk
                anchors.fill: parent
                anchors.margins: Tokens.s3
                title: I18n.tr("Disk I/O")
                values: root.diskValues
                formatter: (value) => root.humanRate(value)
                samplePeriodSeconds: root.samplePeriodSeconds
            }
            Rectangle { anchors.right: parent.right; width: 1; height: parent.height; color: Tokens.lineSoft }
        }
        Item {
            width: root.width / 2 - 1
            height: (root.height - 2) / 2
            MetricGraph {
                id: network
                anchors.fill: parent
                anchors.margins: Tokens.s3
                title: I18n.tr("Network I/O")
                values: root.netValues
                formatter: (value) => root.humanRate(value)
                samplePeriodSeconds: root.samplePeriodSeconds
            }
        }
    }
}
