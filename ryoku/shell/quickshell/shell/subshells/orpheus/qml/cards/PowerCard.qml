pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    property bool shown: false
    property var args: ({})
    property bool profileClaimed: false

    implicitWidth: sheet.implicitWidth
    implicitHeight: sheet.implicitHeight

    readonly property var profileModel: (PowerProfiles.profiles || []).map(function(profile) {
        return {
            value: profile,
            label: root.profileLabel(profile),
            icon: root.profileIcon(profile)
        };
    })
    readonly property string remainingText: {
        if (!Battery.hasTime)
            return "";
        if (Battery.charging)
            return "Full in " + Battery.timeStr;
        if (Battery.discharging)
            return Battery.timeStr + " remaining";
        return Battery.timeStr;
    }
    readonly property string batteryState: {
        if (Battery.charging) return "Charging";
        if (Battery.full) return "Plugged in, full";
        if (Battery.discharging) return "Discharging";
        return "Plugged in";
    }

    function profileLabel(profile) {
        if (profile === "power-saver") return "Saver";
        if (profile === "performance") return "Performance";
        if (profile === "balanced") return "Balanced";
        return String(profile || "").replace(/-/g, " ");
    }

    function profileIcon(profile) {
        if (profile === "power-saver") return "battery_saver";
        if (profile === "performance") return "bolt";
        return "balance";
    }

    function setProfileClaim(enabled) {
        if (profileClaimed === enabled)
            return;
        profileClaimed = enabled;
        PowerProfiles.setActive(root, enabled);
    }

    onShownChanged: setProfileClaim(shown)
    Component.onCompleted: setProfileClaim(shown)
    Component.onDestruction: setProfileClaim(false)

    K.Sheet {
        id: sheet
        pageWidth: 320

        Column {
            width: parent.width
            spacing: K.Style.s4

            Item {
                visible: Battery.present
                width: parent.width
                implicitHeight: Math.max(percentLabel.implicitHeight, stateLabel.implicitHeight)

                K.Label {
                    id: percentLabel
                    anchors.left: parent.left
                    text: Battery.pct + "%"
                    role: "display"
                    numeric: true
                }

                K.Label {
                    id: stateLabel
                    anchors.left: percentLabel.right
                    anchors.leftMargin: K.Style.s3
                    anchors.right: parent.right
                    anchors.baseline: percentLabel.baseline
                    text: root.batteryState
                    role: "body"
                    tone: "dim"
                }
            }

            K.Label {
                visible: Battery.present && root.remainingText.length > 0
                width: parent.width
                text: root.remainingText
                role: "body"
                numeric: true
            }

            Item {
                visible: Battery.present
                width: parent.width
                height: K.Style.rowH

                Rectangle {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.top: parent.top
                    height: K.Style.border
                    color: K.Style.lineSoft
                }

                K.Label {
                    anchors.left: parent.left
                    anchors.right: healthValue.left
                    anchors.rightMargin: K.Style.s2
                    anchors.verticalCenter: parent.verticalCenter
                    text: "Battery health"
                    role: "body"
                }

                K.Label {
                    id: healthValue
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: Battery.healthSupported ? Battery.health + "%" : "Unavailable"
                    role: "body"
                    tone: "dim"
                    numeric: Battery.healthSupported
                }
            }

            K.Label {
                visible: !Battery.present
                width: parent.width
                text: "No internal battery"
                role: "body"
                tone: "dim"
            }

            K.Segmented {
                visible: PowerProfiles.available
                width: parent.width
                model: root.profileModel
                value: PowerProfiles.profile
                onPicked: value => PowerProfiles.setProfile(value)
            }

            K.Button {
                width: implicitWidth
                text: "Graphics and power settings"
                icon: "settings"
                variant: "outline"
                onClicked: root.scene.openHub("gpu")
            }
        }
    }
}
