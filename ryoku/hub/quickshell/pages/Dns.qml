import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/network"
import "../components/wifi"
import "../schema/dns.js" as Schema

// the system-wide resolver, set through the shell daemon's network.dnsSet (the call the bar's DNS
// picker makes), so Settings and the bar never disagree about which servers are in use
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    property bool busy: false
    property string error: ""
    // Custom picked but not applied yet: the servers field shows first
    property bool customOpen: false
    property string customText: ""
    signal focusCustom()

    readonly property string provider: NetworkLink.dnsProvider
    readonly property bool customShown: root.customOpen || root.provider === "custom"
    readonly property var providers: [
        { value: "dhcp", label: I18n.tr("DHCP") },
        { value: "cloudflare", label: I18n.tr("Cloudflare") },
        { value: "google", label: I18n.tr("Google") },
        { value: "custom", label: I18n.tr("Custom") }
    ]

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: []
    })

    readonly property string stateLine: {
        if (!NetworkLink.ready)
            return "";
        if (root.provider === "custom")
            return NetworkLink.dnsServers.length > 0 ? I18n.tr("Custom: %1").arg(NetworkLink.dnsServers.join(", ")) : I18n.tr("Custom servers");
        var names = { cloudflare: I18n.tr("Cloudflare"), google: I18n.tr("Google") };
        return names[root.provider] || I18n.tr("From the network (DHCP)");
    }

    // the field follows the servers in use until the reader types; a new frame never undoes typing
    function syncCustom() {
        root.customText = root.provider === "custom" ? NetworkLink.dnsServers.join(", ") : "";
    }
    Component.onCompleted: root.syncCustom()
    Connections {
        target: NetworkLink
        function onDnsChanged() { root.syncCustom(); }
    }

    function servers(text) {
        return String(text || "").split(/[\s,]+/).filter(s => s.length > 0);
    }

    function apply(provider, list) {
        root.busy = true;
        root.error = "";
        NetworkLink.setDns(provider, list, function (ok, error) {
            root.busy = false;
            if (ok)
                root.customOpen = false;
            else
                root.error = error || I18n.tr("Could not change the DNS servers.");
        });
    }

    function choose(value) {
        if (value === "custom") {
            root.customOpen = true;
            root.error = "";
            if (root.provider !== "custom")
                root.customText = "";
            Qt.callLater(root.focusCustom);
            return;
        }
        root.customOpen = false;
        if (value !== root.provider)
            root.apply(value, []);
    }

    function landable(rowId) {
        return page.layout.length > 0 && (!!page.rowItems[rowId] || root.isSectionLink(rowId));
    }

    function isSectionLink(id) {
        return !Schema.rows.some(r => r.id === id) && Schema.sections.some(s => s.id === id);
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (rowId !== root.pendingRow)
            focusRetry.tries = 0;
        if (!root.landable(rowId) && focusRetry.tries < 30) {
            root.pendingRow = rowId;
            focusRetry.tries++;
            focusRetry.restart();
            return;
        }
        root.pendingRow = "";
        if (root.isSectionLink(rowId)) {
            page.focusRow(rowId);
            return;
        }
        if (page.rowItems[rowId])
            page.land(rowId);
        else
            page.chooseSection(0);
    }

    Timer {
        id: focusRetry
        property int tries: 0
        interval: 100
        onTriggered: root.focusRow(root.pendingRow)
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "dns"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ resolver: resolverBody })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Pace { id: applyPace; running: root.busy }

    Component {
        id: resolverBody

        Column {
            width: parent ? parent.width : 0
            spacing: LithicTokens.space.s4

            LinkNotice {
                host: page
            }

            Slab {
                width: parent.width

                Column {
                    width: parent.width

                    NetRow {
                        host: page
                        rowId: "dns-provider"
                        first: true
                        title: I18n.tr("DNS provider")
                        detail: applyPace.slow ? I18n.tr("Applying…") : I18n.tr("Where this computer looks up names. DHCP uses the servers the network hands out.")
                        error: root.error
                        canRetry: false

                        Working {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: applyPace.slow
                            active: visible
                        }
                        Segmented {
                            id: providerSeg
                            anchors.verticalCenter: parent.verticalCenter
                            enabled: NetworkLink.usable && !root.busy
                            model: root.providers.map(p => p.label)
                            accessibleName: I18n.tr("DNS provider")
                            onActivated: index => root.choose(root.providers[index].value)
                            Binding {
                                target: providerSeg
                                property: "currentIndex"
                                value: Math.max(0, root.providers.findIndex(p => p.value === (root.customOpen ? "custom" : root.provider)))
                            }
                        }
                    }

                    LongWait {
                        visible: applyPace.lagging
                        cancellable: false
                        text: I18n.tr("Still applying. A password prompt may be waiting for you.")
                    }

                    NetRow {
                        host: page
                        rowId: root.customShown ? "dns-custom" : ""
                        visible: root.customShown
                        title: I18n.tr("Custom servers")
                        detail: I18n.tr("IPv4 or IPv6 addresses, separated by spaces or commas")

                        TextField {
                            id: customField
                            anchors.verticalCenter: parent.verticalCenter
                            width: Math.min(LithicTokens.size.settingsControlMax, page.columnWidth * 0.4)
                            mono: true
                            enabled: NetworkLink.usable && !root.busy
                            placeholderText: "1.1.1.1, 9.9.9.9"
                            text: root.customText
                            Accessible.name: I18n.tr("Custom DNS servers")
                            onTextEdited: root.customText = customField.text
                            onAccepted: if (root.servers(root.customText).length > 0) root.apply("custom", root.servers(root.customText))

                            Connections {
                                target: root
                                function onFocusCustom() { customField.forceActiveFocus(); }
                            }
                        }
                        Plaque {
                            anchors.verticalCenter: parent.verticalCenter
                            kind: "gold"
                            small: true
                            busy: root.busy
                            enabled: NetworkLink.usable && !root.busy && root.servers(root.customText).length > 0
                            text: I18n.tr("Apply")
                            onClicked: root.apply("custom", root.servers(root.customText))
                        }
                    }

                    NetRow {
                        host: page
                        rowId: "dns-in-use"
                        title: I18n.tr("In use")
                        detail: !NetworkLink.ready ? NetworkLink.unavailable
                              : NetworkLink.dnsServers.length > 0 ? NetworkLink.dnsServers.join("   ")
                              : I18n.tr("The network's own servers (DHCP)")
                    }
                }
            }
        }
    }
}
