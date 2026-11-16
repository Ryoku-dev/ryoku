import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."
import "../../registry/rarity.js" as Rarity
import "../badge"

// A friend's card stays theirs. Only its picture may become part of your own design.
Item {
    id: root
    anchors.fill: parent
    property bool opened: false
    property var entries: []
    property var selected: null
    property string job: ""
    property bool inspecting: false
    property string error: ""
    property string message: ""
    property real arrival: 1
    readonly property string configDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"
    readonly property string cardsStoreRoot: (Quickshell.env("XDG_DATA_HOME") || (Quickshell.env("HOME") + "/.local/share")) + "/ryoku/cards"
    visible: shade.opacity > 0
    focus: root.opened
    Keys.onEscapePressed: root.close()
    Certificate {
        id: certificate
        parent: root.parent
        badge: root.selected && root.selected.verified === true && root.selected.badgeCheck === "ok" ? root.selected.badge || null : null
        spec: certificate.badge && root.selected.design && root.selected.design.spec ? root.selected.design.spec.badge : null
    }

    function refresh() {
        root.run("refresh", ["ryoku-hub", "cards", "refresh"], function(out) {
            try {
                var id = root.selected ? root.selected.id : "";
                root.entries = JSON.parse(out);
                if (id !== "")
                    root.selected = root.entries.find(entry => entry.id === id) || null;
                if (!root.selected)
                    root.inspecting = false;
                root.message = I18n.tr("Binder refreshed");
            } catch (e) { root.error = I18n.tr("Your binder could not be refreshed. Try again."); }
        });
    }

    function verification(entry) {
        if (!entry)
            return "";
        if (entry.badgeCheck === "offline")
            return I18n.tr("Offline: badge and card updates could not be checked. Connect and press Refresh.");
        if (entry.badgeCheck === "unverified")
            return I18n.tr("Unverified: the signed card record could not be checked, so these facts come from the picture.");
        if (!entry.verified || !entry.badge)
            return I18n.tr("Unverified: this card has no verified badge and signed card record.");
        var badge = entry.badge;
        var role = badge.role.charAt(0).toUpperCase() + badge.role.slice(1);
        return I18n.tr("Verified %1 · issued by %2 · %3").replace("%1", role)
            .replace("%2", badge.issuer === "owner" ? "nero" : "Mika").replace("%3", badge.issued);
    }

    function open() {
        root.opened = true;
        root.forceActiveFocus();
        root.load();
    }
    function close() {
        root.opened = false;
        arrive.stop();
        root.arrival = 1;
    }
    function show(entry, fresh) {
        root.selected = entry;
        root.inspecting = true;
        root.message = fresh ? I18n.tr("Added to your binder") : "";
        if (fresh && root.opened && !Lithic.reduceMotion)
            arrive.restart();
    }
    function refusal(err, code, action) {
        var e = String(err || "");
        if (/your own card/i.test(e))
            return I18n.tr("That is your own card; your binder holds your friends'.");
        if (/no picture|clipboard.*empty/i.test(e))
            return I18n.tr("The clipboard holds no picture. Copy a friend's card picture, then paste it here.");
        if (/card code|not a Ryoku card/i.test(e))
            return I18n.tr("That card could not be read. Ask for the whole card picture, not a crop or part of a screenshot.");
        if (/no space left/i.test(e))
            return I18n.tr("The disk is full. Free some space, then try again.");
        if (/permission denied|read-only|operation not permitted/i.test(e))
            return I18n.tr("Settings cannot read that picture or write to your binder. Check the file and folder permissions, then try again.");
        if (code === -1)
            return /missing|not found|not installed/i.test(e) ? I18n.tr("The card tool is missing. Update Ryoku, then try again.")
                : I18n.tr("That took too long. Try again.");
        if (action === "list")
            return I18n.tr("Your binder could not be read. Close it and open it again to retry.");
        if (action === "refresh")
            return I18n.tr("Your binder could not be refreshed. Check your connection and try again.");
        if (action === "remove")
            return I18n.tr("That card could not be removed. Try again.");
        if (action === "picture")
            return I18n.tr("Their picture could not be used. Check that the picture is still there, then try again.");
        return I18n.tr("That picture could not be added. Choose the original card picture with Open a card, or paste it again.");
    }
    function run(action, argv, done) {
        root.job = action;
        root.error = "";
        root.message = "";
        Changes.run(argv, function (ok, out, err, code) {
            root.job = "";
            if (!ok) {
                root.error = root.refusal(err, code, action);
                return;
            }
            done(out);
        }, { timeout: 30000 });
    }
    function load() {
        if (root.job !== "")
            return;
        root.run("list", ["ryoku-hub", "cards", "list"], function (out) {
            try { root.entries = JSON.parse(out); }
            catch (e) { root.error = I18n.tr("Your binder could not be read. Close it and open it again to retry."); }
        });
    }
    function add(file) {
        if (root.job !== "")
            return;
        if (file && !/\.(png|jpe?g|gif)$/i.test(file)) {
            root.error = "";
            root.job = "convert";
            convert.source = "";
            convert.pending = file;
            convert.source = chooser.urlOf(file);
            return;
        }
        root.addPicture(file);
    }
    function addPicture(file) {
        root.run("add", ["ryoku-hub", "cards", "add", file || "--clipboard"], function (out) {
            try {
                var entry = JSON.parse(out);
                root.entries = [entry].concat(root.entries.filter(e => e.id !== entry.id))
                    .sort((a, b) => Date.parse(b.added) - Date.parse(a.added));
                root.show(entry, true);
            } catch (e) {
                root.error = I18n.tr("The card was added, but could not be displayed. Close the binder and open it again.");
            }
        });
    }
    function remove() {
        var id = root.selected.id;
        root.run("remove", ["ryoku-hub", "cards", "remove", id], function () {
            root.entries = root.entries.filter(e => e.id !== id);
            root.inspecting = false;
            root.message = I18n.tr("Card removed from your binder");
        });
    }
    function setHero(patch) {
        Changes.ensureStore("card", Qt.resolvedUrl("CardStore.qml"));
        var bind = { store: "card", key: "hero", fallback: { kind: "default", source: "" } };
        Changes.set(bind, Object.assign({}, Changes.value(bind) || {}, patch), I18n.tr("Card picture"), "");
        root.message = I18n.tr("Your card uses their picture. Your words and machine stay yours.");
    }
    function picturePath(entry) {
        if (!entry || !entry.hero || !/^[0-9a-f]{16}$/.test(String(entry.id || "")) || root.cardsStoreRoot.charAt(0) !== "/")
            return "";
        var verified = entry.verified === true && entry.badgeCheck === "ok";
        var base = root.cardsStoreRoot + "/" + entry.id;
        var path = String(entry.hero.file || "");
        return path === base + ".png" || (verified && path === base + "-record.png") ? path : "";
    }
    function usePicture() {
        var hero = root.selected.hero;
        if (hero.kind === "gallery") {
            if (/^[a-z0-9_-]+(?:\.(png|gif))?$/.test(String(hero.source || "")))
                root.setHero({ kind: "gallery", source: hero.source });
            return;
        }
        if (hero.kind !== "custom")
            return;
        var picture = root.picturePath(root.selected);
        if (picture === "") {
            root.error = root.refusal("", 1, "picture");
            return;
        }
        root.run("picture", ["install", "-D", "-m", "644", "--", picture, root.configDir + "/profile/hero.png"], function () {
            root.setHero({ kind: "custom", source: "hero.png", style: "photo", fit: false, zoom: 1,
                focalX: 0.5, focalY: 0.5, brightness: 0, contrast: 0, saturation: 0 });
        });
    }
    NumberAnimation {
        id: arrive
        target: root
        property: "arrival"
        from: 0
        to: 1
        duration: Lithic.duration(260)
        easing.type: Easing.OutCubic
    }
    Item {
        id: conversionFrame
        width: 0
        height: 0
        clip: true
    }
    Image {
        id: convert
        property string pending: ""
        parent: conversionFrame
        asynchronous: true
        // Both axes must be bounded before Qt decodes a foreign image or grabs its conversion.
        sourceSize.width: 2048
        sourceSize.height: 2861
        width: Math.min(2048, implicitWidth)
        height: Math.min(2861, implicitHeight)
        fillMode: Image.PreserveAspectFit
        onStatusChanged: {
            if (convert.pending === "" || status === Image.Loading || status === Image.Null)
                return;
            convert.pending = "";
            if (status !== Image.Ready) {
                root.job = "";
                root.error = root.refusal("", 1, "add");
                return;
            }
            var file = (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-binder-open.png";
            var grabbed = convert.grabToImage(function (result) {
                root.job = "";
                if (result.saveToFile(file))
                    root.addPicture(file);
                else
                    root.error = root.refusal("", 1, "add");
            }, Qt.size(convert.width, convert.height));
            if (!grabbed) {
                root.job = "";
                root.error = root.refusal("", 1, "add");
            }
        }
    }
    ImageChooser {
        id: chooser
        overlayParent: root.parent
        title: I18n.tr("Open a card")
        onAccepted: root.add(chooser.localPath(chooser.selectedFile))
    }
    Rectangle {
        id: shade
        anchors.fill: parent
        color: Lithic.alpha(Lithic.ground, Lithic.dark ? 0.96 : 0.9)
        opacity: root.opened ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
        MouseArea { anchors.fill: parent; onClicked: root.close() }
    }
    CutRect {
        id: panel
        anchors.centerIn: parent
        width: Math.min(1160, root.width - 48)
        height: root.height - 48
        cut: LithicTokens.cut.large
        fill: Lithic.stone1
        strokeWidth: 1
        strokeColor: Lithic.alpha(Lithic.fg, 0.2)
        opacity: shade.opacity
        MouseArea { anchors.fill: parent }
        Row {
            id: heading
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.margins: LithicTokens.space.s5
            spacing: LithicTokens.space.s3
            Glyph { anchors.verticalCenter: parent.verticalCenter; name: "collections_bookmark"; size: 28; ink: Lithic.gold }
            Label { text: I18n.tr("Binder"); typeRole: "title" }
            Label { anchors.verticalCenter: parent.verticalCenter; text: String(root.entries.length); typeRole: "mono"; inkRole: "fg2" }
            Label { anchors.verticalCenter: parent.verticalCenter; visible: UiPreferences.showJapaneseGloss; text: "収集"; typeRole: "caption"; inkRole: "fg3" }
        }
        Row {
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.margins: LithicTokens.space.s4
            spacing: LithicTokens.space.s2
            Plaque { kind: "gold"; small: true; glyph: "content_paste"; text: I18n.tr("Paste a card"); enabled: root.job === ""; busy: root.job === "add"; onClicked: root.add("") }
            Plaque { kind: "line"; small: true; glyph: "file_open"; text: I18n.tr("Open a card"); enabled: root.job === ""; busy: root.job === "convert"; onClicked: chooser.open() }
            Plaque { kind: "quiet"; small: true; glyph: "refresh"; text: I18n.tr("Refresh"); enabled: root.job === ""; busy: root.job === "refresh"; onClicked: root.refresh() }
            IconButton { glyph: "close"; accessibleName: I18n.tr("Close the binder"); onClicked: root.close() }
        }
        Label {
            id: notice
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: heading.bottom
            anchors.margins: LithicTokens.space.s5
            text: root.error || root.message || (root.job === "list" ? I18n.tr("Reading your binder…") : "")
            visible: text !== ""
            typeRole: "detail"
            ink: root.error !== "" ? Lithic.jasper : Lithic.fg2
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Loader {
            id: body
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: notice.visible ? notice.bottom : heading.bottom
            anchors.bottom: parent.bottom
            anchors.margins: LithicTokens.space.s5
            active: root.opened
            sourceComponent: root.inspecting ? detail : collection
        }
    }
    Component {
        id: collection
        Item {
            // empty: three blank cards fanned like a hand waiting to be dealt
            Column {
                anchors.centerIn: parent
                width: Math.min(460, parent.width)
                spacing: LithicTokens.space.s5
                visible: root.entries.length === 0 && root.job !== "list"
                Item {
                    anchors.horizontalCenter: parent.horizontalCenter
                    width: 260
                    height: 190
                    Repeater {
                        model: [-14, 0, 14]
                        delegate: CutRect {
                            required property var modelData
                            required property int index
                            z: index === 1 ? 1 : 0
                            x: 130 - width / 2 + modelData * 4
                            y: 20 + Math.abs(modelData) * 0.6
                            width: 112
                            height: 156
                            rotation: modelData
                            transformOrigin: Item.Bottom
                            cut: 8
                            fill: index === 1 ? Lithic.stone2 : Lithic.stone1
                            strokeWidth: 1
                            strokeColor: index === 1 ? Lithic.alpha(Lithic.gold, 0.7) : Lithic.vein
                            Label {
                                visible: index === 1
                                anchors.centerIn: parent
                                text: "力"
                                typeRole: "title"
                                font.pixelSize: 40
                                ink: Lithic.alpha(Lithic.gold, 0.6)
                            }
                        }
                    }
                }
                Label { width: parent.width; text: I18n.tr("A place for your friends' cards"); typeRole: "title"; horizontalAlignment: Text.AlignHCenter; wrapMode: Text.Wrap; elide: Text.ElideNone }
                Label {
                    width: parent.width
                    text: I18n.tr("Ask a friend to press Share on their Home and send you the picture. Copy the picture, then press Paste a card here. Their card comes alive in your binder.")
                    typeRole: "body"
                    inkRole: "fg2"
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }
            GridView {
                id: grid
                readonly property real cardWidth: 248
                readonly property int columns: Math.max(1, Math.min(root.entries.length, Math.floor(parent.width / (cardWidth + 48))))
                anchors.fill: parent
                // the cards stand centred, as a binder page is laid out
                leftMargin: Math.max(0, (width - columns * cellWidth) / 2)
                visible: root.entries.length > 0
                clip: true
                cellWidth: cardWidth + 48
                cellHeight: cardWidth * 880 / 630 + 84
                model: root.entries
                boundsBehavior: Flickable.StopAtBounds
                delegate: Item {
                    id: slot
                    required property var modelData
                    readonly property var rarity: Rarity.of(modelData.hw)
                    width: grid.cellWidth
                    height: grid.cellHeight
                    BinderCard {
                        id: thumbnail
                        anchors.horizontalCenter: parent.horizontalCenter
                        // a card under the pointer lifts out of its pocket
                        y: 14 - (thumbnail.pointed ? 8 : 0)
                        Behavior on y { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
                        width: grid.cardWidth
                        entry: slot.modelData
                        cardsStoreRoot: root.cardsStoreRoot
                        onChosen: root.show(slot.modelData, false)
                    }
                    Label {
                        anchors.top: thumbnail.bottom
                        anchors.topMargin: 14
                        width: parent.width - 16
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: slot.modelData.name
                        typeRole: "label"
                        font.weight: Font.DemiBold
                        horizontalAlignment: Text.AlignHCenter
                        elide: Text.ElideRight
                    }
                    Label {
                        anchors.top: thumbnail.bottom
                        anchors.topMargin: 38
                        width: parent.width
                        text: slot.rarity.tier.mark + "  " + I18n.tr(slot.rarity.tier.name) + "  ·  " + I18n.tr("%1 pts").replace("%1", slot.rarity.points)
                        typeRole: "caption"
                        horizontalAlignment: Text.AlignHCenter
                        ink: slot.rarity.level >= 2 ? Lithic.gold : Lithic.fg2
                    }
                }
            }
        }
    }
    Component {
        id: detail
        Item {
            id: detailRoot
            readonly property real cardWidth: Math.min(430, width * 0.43, height * 630 / 880)
            BinderCard {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: detailRoot.cardWidth
                entry: root.selected
                cardsStoreRoot: root.cardsStoreRoot
                interactive: false
                opacity: 0.5 + 0.5 * root.arrival
                scale: 0.94 + 0.06 * root.arrival
            }
            Flickable {
                anchors.left: parent.left
                anchors.leftMargin: detailRoot.cardWidth + LithicTokens.space.s6
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                contentWidth: width
                contentHeight: facts.height
                boundsBehavior: Flickable.StopAtBounds
                clip: true
                Column {
                    id: facts
                    width: parent.width
                    spacing: LithicTokens.space.s4
                    Plaque { kind: "quiet"; small: true; glyph: "arrow_back"; text: I18n.tr("Back"); onClicked: { root.inspecting = false; root.message = ""; } }
                    Label { width: parent.width; text: root.selected.name; typeRole: "name"; elide: Text.ElideRight }
                    Label {
                        width: parent.width
                        text: I18n.tr("Traded card · added %1").replace("%1", Qt.formatDate(new Date(root.selected.added), "yyyy-MM-dd"))
                        typeRole: "detail"
                        inkRole: "fg2"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    Label {
                        width: parent.width
                        text: root.verification(root.selected)
                        typeRole: "detail"
                        ink: root.selected.verified ? Lithic.gold : Lithic.fg2
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    Plaque {
                        visible: root.selected.verified === true && !!root.selected.badge && root.selected.badgeCheck === "ok"
                        small: true
                        kind: "line"
                        text: I18n.tr("View certificate")
                        onClicked: certificate.open()
                    }
                    Label { text: I18n.tr(Rarity.of(root.selected.hw).tier.name) + " · " + I18n.tr("%1 pts").replace("%1", Rarity.of(root.selected.hw).points); typeRole: "label"; ink: Lithic.gold }
                    BinderCompare { width: parent.width; entry: root.selected }
                    Flow {
                        width: parent.width
                        spacing: LithicTokens.space.s2
                        Plaque {
                            visible: root.selected.hero.kind === "custom" || root.selected.hero.kind === "gallery"
                            kind: "gold"
                            glyph: "image"
                            text: I18n.tr("Use their picture")
                            enabled: root.job === ""
                            busy: root.job === "picture"
                            onClicked: root.usePicture()
                        }
                        Plaque { kind: "quiet"; glyph: "delete"; text: I18n.tr("Remove"); enabled: root.job === ""; busy: root.job === "remove"; onClicked: root.remove() }
                    }
                    Label {
                        width: parent.width
                        text: I18n.tr("Traded cards stay in your binder. They cannot be copied, saved or shared as your own.")
                        typeRole: "caption"
                        inkRole: "fg3"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                }
            }
        }
    }
}
