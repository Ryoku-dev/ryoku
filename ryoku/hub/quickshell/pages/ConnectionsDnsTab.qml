pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// Connections > DNS: the system-wide resolver, set through the shell daemon's
// `network.dnsSet` (the same call the bar's DNS picker makes), so the Hub and
// the bar never disagree about which servers are in use.
Item {
    id: tab

    property bool busy: false
    property string error: ""
    // "custom" picked but not applied yet: the servers field shows first
    property bool customOpen: false
    readonly property string provider: NetworkTopic.dnsProvider
    readonly property bool customShown: tab.customOpen || tab.provider === "custom"

    function servers(text) {
        return String(text || "").split(/[\s,]+/).filter(function(s) { return s.length > 0; });
    }
    function apply(provider, list) {
        tab.busy = true;
        tab.error = "";
        NetworkTopic.setDns(provider, list, function(ok, error) {
            tab.busy = false;
            if (ok)
                tab.customOpen = false;
            else
                tab.error = error || I18n.tr("Could not change the DNS servers.");
        });
    }

    Flickable {
        id: flick
        anchors.fill: parent
        clip: true
        contentWidth: width
        contentHeight: content.implicitHeight + Tokens.s4
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        Column {
            id: content
            width: flick.width - Tokens.s4
            spacing: Tokens.s4

            CardColumns {
                id: cards
                width: parent.width
                minColumnWidth: 400

                SettingCard {
                    property bool fullWidth: true
                    width: cards.width
                    title: I18n.tr("DNS")
                    collapsible: false

                    SettingRow {
                        width: parent.width
                        label: I18n.tr("Provider")
                        desc: I18n.tr("Where this machine looks up names. DHCP uses the servers the network hands out.")
                        controlWidth: providerSeg.implicitWidth
                        enabled: NetworkTopic.ready && !tab.busy
                        Seg {
                            id: providerSeg
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: ["dhcp", "cloudflare", "google", "custom"]
                            labels: ({
                                "dhcp": I18n.tr("DHCP"),
                                "cloudflare": I18n.tr("Cloudflare"),
                                "google": I18n.tr("Google"),
                                "custom": I18n.tr("Custom")
                            })
                            current: tab.customOpen ? "custom" : tab.provider
                            onChose: function(key) {
                                if (key === "custom") {
                                    tab.customOpen = true;
                                    tab.error = "";
                                    if (tab.provider !== "custom")
                                        customField.text = "";
                                    return;
                                }
                                tab.customOpen = false;
                                if (key !== tab.provider)
                                    tab.apply(key, []);
                            }
                        }
                    }

                    SettingRow {
                        width: parent.width
                        visible: tab.customShown
                        label: I18n.tr("Custom servers")
                        desc: I18n.tr("IPv4 or IPv6 addresses, separated by spaces or commas.")
                        controlWidth: customControls.implicitWidth
                        enabled: NetworkTopic.ready && !tab.busy
                        Row {
                            id: customControls
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Field {
                                id: customField
                                width: 260
                                anchors.verticalCenter: parent.verticalCenter
                                tabular: true
                                placeholder: "1.1.1.1, 9.9.9.9"
                                text: tab.provider === "custom" ? NetworkTopic.dnsServers.join(", ") : ""
                                onAccepted: tab.apply("custom", tab.servers(customField.text))
                            }
                            Btn {
                                anchors.verticalCenter: parent.verticalCenter
                                primary: true
                                armed: tab.servers(customField.text).length > 0 && !tab.busy
                                text: I18n.tr("Apply")
                                onAct: tab.apply("custom", tab.servers(customField.text))
                            }
                        }
                    }

                    SettingRow {
                        width: parent.width
                        label: I18n.tr("In use")
                        desc: !NetworkTopic.ready ? I18n.tr("Waiting for the shell daemon...")
                            : tab.busy ? I18n.tr("Applying...")
                            : NetworkTopic.dnsServers.length > 0 ? NetworkTopic.dnsServers.join("  ")
                            : I18n.tr("The network's own servers (DHCP)")
                    }

                    Item {
                        visible: tab.error !== ""
                        width: parent.width
                        implicitHeight: visible ? errorText.implicitHeight + Tokens.s3 * 2 : 0
                        Text {
                            id: errorText
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.margins: Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            text: tab.error
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall
                            wrapMode: Text.WordWrap
                        }
                    }
                }
            }
        }
    }
}
