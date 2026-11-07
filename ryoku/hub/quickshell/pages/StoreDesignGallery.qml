pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../barstudio/ResourceTier.js" as Tier

Item {
    id: gallery

    required property string category
    required property string wornId
    required property var shippedDesigns
    property var categories: [gallery.category]
    property string browseCategory: gallery.category
    // Installed and shipped shell items from shells.json; the catalogue only
    // adds what is not installed yet and the update state.
    property var shellItems: []
    property var barstyleDesigns: []
    property var subshellDesigns: []
    property var otherDesigns: []
    readonly property var catalogueDesigns: {
        let out = [];
        if (gallery.categories.indexOf("subshells") !== -1)
            out = out.concat(gallery.subshellDesigns);
        if (gallery.categories.indexOf("barstyles") !== -1)
            out = out.concat(gallery.barstyleDesigns);
        if (gallery.categories.indexOf(gallery.category) !== -1
                && gallery.category !== "subshells"
                && gallery.category !== "barstyles")
            out = out.concat(gallery.otherDesigns);
        return out;
    }
    readonly property bool shellGallery: gallery.categories.indexOf("subshells") !== -1
        || gallery.categories.indexOf("barstyles") !== -1
    readonly property string cardObjectPrefix: gallery.shellGallery
        ? "shell-item-" : "store-design-" + gallery.category + "-"
    readonly property string updateObjectPrefix: gallery.shellGallery
        ? "shell-item-update-" : "store-design-update-" + gallery.category + "-"
    property string updatingId: ""
    property string extraActionText: ""
    readonly property var designs: gallery.cardModel()

    signal wear(string designId)
    signal extraAction()

    function shippedCard(entry) {
        return {
            id: entry.id,
            name: entry.name || entry.id,
            desc: entry.description || entry.desc || "",
            category: entry.category || gallery.category,
            installed: true,
            version: "",
            installedVersion: "",
            updateAvailable: false,
            unavailable: false,
            unavailableEdition: false,
            unavailableReason: "",
            requiredWindowManager: "",
            downloadPaused: false,
            resourceTier: Tier.tierOf(entry.resourceTier),
            downloadPauseReason: "",
            source: "shipped",
            settings: ({})
        };
    }

    function shellCard(entry, listed) {
        const card = listed ? Object.assign({}, listed) : gallery.shippedCard({
            id: entry.id,
            name: entry.name,
            description: entry.summary,
            category: entry.category
        });
        card.installed = true;
        card.source = entry.source || "store";
        card.installedVersion = entry.version || card.installedVersion || "";
        if (!card.version)
            card.version = card.installedVersion;
        card.settings = entry.settings || card.settings || ({});
        return card;
    }

    function cardModel() {
        const out = [];
        const seen = {};
        const shipped = Array.isArray(gallery.shippedDesigns) ? gallery.shippedDesigns : [];
        for (let i = 0; i < shipped.length; i++) {
            const card = gallery.shippedCard(shipped[i]);
            if (!card.id || seen[card.id])
                continue;
            seen[card.id] = true;
            out.push(card);
        }
        const catalogue = Array.isArray(gallery.catalogueDesigns) ? gallery.catalogueDesigns : [];
        const listed = {};
        for (let k = 0; k < catalogue.length; k++)
            if (catalogue[k].id)
                listed[catalogue[k].id] = catalogue[k];
        const local = Array.isArray(gallery.shellItems) ? gallery.shellItems : [];
        for (let m = 0; m < local.length; m++) {
            const entry = local[m];
            if (!entry || !entry.id || seen[entry.id]
                    || gallery.categories.indexOf(entry.category) === -1)
                continue;
            seen[entry.id] = true;
            out.push(gallery.shellCard(entry, listed[entry.id] || null));
        }
        for (let j = 0; j < catalogue.length; j++) {
            const item = catalogue[j];
            if (!item.id || seen[item.id])
                continue;
            seen[item.id] = true;
            out.push(item);
        }
        return out;
    }

    function browse() {
        Spawn.runApp(["ryostore", "open", gallery.browseCategory], { label: "ryostore" });
    }

    function updateDesign(item) {
        if (!item || !item.id || !item.category || gallery.updatingId !== "")
            return;
        gallery.updatingId = item.id;
        updateProcess.command = ["ryostore", "install", item.category, item.id];
        updateProcess.running = true;
    }

    function cards(text, category) {
        try {
            const catalog = JSON.parse(text || "{}");
            // Every shell item stays visible, dimmed with its reason when this
            // desktop cannot run it; other designs hide what can never be worn.
            return (catalog.items || [])
                .filter(item => item.category === category
                    && (gallery.shellGallery
                        || !(item.unavailable === true && item.installed !== true)))
                .map(item => ({
                    id: item.id,
                    category: category,
                    name: item.name || item.id,
                    desc: item.summary || item.description || "",
                    installed: item.installed === true,
                    version: item.version || "",
                    installedVersion: item.installedVersion || "",
                    updateAvailable: item.updateAvailable === true,
                    unavailable: item.unavailable === true,
                    unavailableEdition: item.unavailableEdition === true,
                    unavailableReason: item.unavailableReason || "",
                    requiredWindowManager: item.requiredWindowManager || "",
                    downloadPaused: item.downloadPaused === true,
                    resourceTier: Tier.tierOf(item.metadata && item.metadata.resourceTier),
                    downloadPauseReason: item.downloadPauseReason || "",
                    source: item.metadata && item.metadata.shipped === true ? "shipped" : "store",
                    settings: item.settings || ({})
                }));
        } catch (e) {
            return [];
        }
    }

    function refresh() {
        barstylesCatalog.running = false;
        subshellsCatalog.running = false;
        otherCatalog.running = false;
        barstylesCatalog.running = gallery.categories.indexOf("barstyles") !== -1;
        subshellsCatalog.running = gallery.categories.indexOf("subshells") !== -1;
        otherCatalog.running = gallery.category !== "barstyles"
            && gallery.category !== "subshells";
    }

    Process {
        id: barstylesCatalog
        command: ["ryostore", "catalog", "--category", "barstyles"]
        running: gallery.categories.indexOf("barstyles") !== -1
        stdout: StdioCollector {
            onStreamFinished: gallery.barstyleDesigns = gallery.cards(this.text, "barstyles")
        }
    }

    Process {
        id: subshellsCatalog
        command: ["ryostore", "catalog", "--category", "subshells"]
        running: gallery.categories.indexOf("subshells") !== -1
        stdout: StdioCollector {
            onStreamFinished: gallery.subshellDesigns = gallery.cards(this.text, "subshells")
        }
    }

    Process {
        id: otherCatalog
        command: ["ryostore", "catalog", "--category", gallery.category]
        running: gallery.category !== "barstyles" && gallery.category !== "subshells"
        stdout: StdioCollector {
            onStreamFinished: gallery.otherDesigns = gallery.cards(this.text, gallery.category)
        }
    }

    Process {
        id: updateProcess
        onExited: {
            gallery.updatingId = "";
            gallery.refresh();
        }
    }

    implicitHeight: galleryBody.height + Tokens.s3 + Tokens.s4
    height: implicitHeight

    Column {
        id: galleryBody
        anchors {
            left: parent.left
            right: parent.right
            top: parent.top
            leftMargin: Tokens.s4
            rightMargin: Tokens.s4
            topMargin: Tokens.s3
        }
        spacing: Tokens.s3

        Flow {
            id: cardFlow
            width: parent.width
            spacing: Tokens.s2
            readonly property int perRow: Math.max(2, Math.floor((width + Tokens.s2) / 210))
            // Warning markers fit the card corner, so every tile can be compact.
            readonly property int tileH: 64
            readonly property int rowCount: Math.max(1, Math.ceil(cardRepeater.count / perRow))
            height: rowCount * tileH + (rowCount - 1) * spacing

            Repeater {
                id: cardRepeater
                model: gallery.designs

                delegate: Rectangle {
                    id: designCard

                    required property var modelData
                    readonly property bool wearable: designCard.modelData.installed && !designCard.modelData.unavailable
                    readonly property bool worn: designCard.wearable && gallery.wornId === designCard.modelData.id
                    readonly property bool updatable: designCard.modelData.updateAvailable === true
                        && designCard.modelData.version.length > 0
                        && !designCard.modelData.unavailable && !designCard.modelData.downloadPaused
                    // ▲ heavy, △ standard, nothing for light or unmeasured.
                    readonly property string tierMark: Tier.markOf(designCard.modelData.resourceTier)
                    readonly property bool subshell: designCard.modelData.category === "subshells"
                    readonly property string subText: {
                        if (designCard.updatable) {
                            const current = designCard.modelData.installedVersion.length > 0
                                ? designCard.modelData.installedVersion : designCard.modelData.version;
                            return I18n.tr("%1 → %2").arg(current).arg(designCard.modelData.version);
                        }
                        if (designCard.wearable)
                            return I18n.tr(designCard.modelData.desc);
                        if (designCard.modelData.unavailable) {
                            const wm = ("" + designCard.modelData.requiredWindowManager).toUpperCase();
                            const tag = designCard.modelData.unavailableEdition ? I18n.tr("Not on this system")
                                : wm.length > 0 ? I18n.tr("%1 only").arg(wm) : I18n.tr("Unavailable");
                            return designCard.modelData.unavailableReason.length > 0
                                ? tag + " · " + I18n.tr(designCard.modelData.unavailableReason) : tag;
                        }
                        if (designCard.modelData.downloadPaused)
                            return designCard.modelData.downloadPauseReason.length > 0
                                ? I18n.tr("Under construction") + " · " + I18n.tr(designCard.modelData.downloadPauseReason)
                                : I18n.tr("Under construction");
                        return I18n.tr("Available · install from RyoStore");
                    }

                    width: Math.floor((cardFlow.width - (cardFlow.perRow - 1) * Tokens.s2) / cardFlow.perRow)
                    height: cardFlow.tileH
                    radius: Tokens.radius
                    opacity: designCard.wearable ? 1.0 : 0.55
                    color: designCard.worn ? Tokens.bone : (cardMouse.containsMouse ? Tokens.tint5 : "transparent")
                    border.width: Tokens.border
                    border.color: designCard.worn ? Tokens.bone : Tokens.line
                    objectName: gallery.cardObjectPrefix + designCard.modelData.id

                    Rectangle {
                        id: resourceMark
                        visible: designCard.tierMark !== ""
                        width: 18
                        height: 18
                        radius: 3
                        color: designCard.worn ? Tokens.inkOnBone : "transparent"
                        anchors.top: parent.top
                        anchors.right: parent.right
                        anchors.topMargin: Tokens.s2
                        anchors.rightMargin: Tokens.s2

                        Text {
                            anchors.centerIn: parent
                            text: designCard.tierMark
                            color: Tokens.bone
                            font.family: Tokens.mono
                            font.pixelSize: 10
                            font.weight: Font.DemiBold
                        }
                    }

                    Text {
                        id: subshellMark
                        visible: designCard.subshell
                        text: I18n.tr("SUBSHELL")
                        color: designCard.worn ? Tokens.inkOnBoneDim : Tokens.inkFaint
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fMicro
                        font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackLabel
                        anchors.verticalCenter: resourceMark.verticalCenter
                        anchors.right: resourceMark.visible ? resourceMark.left : parent.right
                        anchors.rightMargin: resourceMark.visible ? Tokens.s1 : Tokens.s2
                    }

                    MouseArea {
                        id: cardMouse
                        anchors.fill: parent
                        hoverEnabled: true
                        preventStealing: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: designCard.wearable ? gallery.wear(designCard.modelData.id) : gallery.browse()
                    }

                    Column {
                        anchors {
                            left: parent.left
                            right: updateButton.visible ? updateButton.left
                                : (subshellMark.visible ? subshellMark.left
                                    : (resourceMark.visible ? resourceMark.left : parent.right))
                            margins: Tokens.s3
                            rightMargin: updateButton.visible ? Tokens.s2 : Tokens.s3
                            verticalCenter: parent.verticalCenter
                        }
                        spacing: 3

                        Text {
                            text: I18n.tr(designCard.modelData.name).toUpperCase()
                            color: designCard.worn ? Tokens.inkOnBone : Tokens.inkDim
                            font.family: Tokens.ui
                            font.pixelSize: 12
                            font.weight: Font.Medium
                            font.letterSpacing: Tokens.trackLabel
                        }
                        Text {
                            width: parent.width
                            text: designCard.subText
                            color: designCard.worn ? Tokens.inkOnBoneDim : Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fTiny
                            elide: Text.ElideRight
                        }
                    }

                    Btn {
                        id: updateButton
                        anchors {
                            right: parent.right
                            rightMargin: Tokens.s3
                            verticalCenter: parent.verticalCenter
                        }
                        visible: designCard.updatable
                        compact: true
                        armed: gallery.updatingId === ""
                        text: gallery.updatingId === designCard.modelData.id
                            ? I18n.tr("UPDATING") : I18n.tr("UPDATE")
                        objectName: gallery.updateObjectPrefix + designCard.modelData.id
                        onAct: gallery.updateDesign(designCard.modelData)
                    }
                }
            }
        }

        Row {
            spacing: Tokens.s2

            Btn {
                text: I18n.tr("BROWSE RYOSTORE")
                onAct: gallery.browse()
            }
            Btn {
                visible: gallery.extraActionText !== ""
                text: gallery.extraActionText
                onAct: gallery.extraAction()
            }
        }
    }
}
