import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "lib/store.js" as StoreLogic

FocusScope {
    id: detail

    property var item: null
    property bool open: false
    property rect originRect: Qt.rect(0, 0, 0, 0)
    property string busyKey: ""
    property string installStage: ""
    property string installErrorKey: ""
    property string installError: ""
    property bool reducedMotion: false
    // off by default: a product is shown as its author made it, and the dither is
    // a look you ask for (and the variant the install then fetches)
    property bool ditherOn: false
    readonly property bool hasDither: String(actionItem.artRaw || "") !== ""
    property real transitionProgress: open ? 1 : 0
    property int lightboxIndex: -1
    readonly property bool lightboxOpen: lightboxIndex >= 0 && lightboxIndex < galleryCount
    onOpenChanged: if (!open) lightboxIndex = -1

    function focusInitialAction() {
        closeButton.forceActiveFocus(Qt.OtherFocusReason);
    }

    signal closeRequested()
    signal installRequested(var item, bool dither, var components)
    signal retryRequested(var item, bool dither, var components)
    signal settingsRequested(var item)
    signal removeRequested(var item)

    readonly property var actionItem: item || ({})
    readonly property string actionKey: StoreLogic.itemKey(actionItem)
    readonly property var screenshots: Array.isArray(actionItem.screenshots) ? actionItem.screenshots : []
    readonly property int screenshotCount: screenshots.length
    // The gallery and lightbox present whatever previews the product ships:
    // its screenshots, or — for a product that ships only cover art, like most
    // bar styles — that single image, so the preview is still expandable.
    readonly property var galleryShots: screenshotCount > 0
            ? screenshots
            : (String(actionItem.art || "") !== "" ? [String(actionItem.art)] : [])
    readonly property int galleryCount: galleryShots.length
    property int galleryIndex: 0
    readonly property var tags: Array.isArray(actionItem.tags) ? actionItem.tags : []
    readonly property color accentColor: actionItem.accent ? Qt.color(actionItem.accent) : Tokens.sun
    readonly property string errorText: installErrorKey === actionKey ? installError : ""
    readonly property string transitionMode: reducedMotion ? "immediate" : "shared"
    readonly property string metadataText: [
        actionItem.author ? I18n.tr("AUTHOR / %1").arg(actionItem.author) : "",
        actionItem.version ? I18n.tr("VERSION / %1").arg(actionItem.version) : "",
        actionItem.size ? I18n.tr("SIZE / %1").arg(actionItem.size) : "",
        actionItem.compatibility ? I18n.tr("COMPATIBILITY / %1").arg(valueText(actionItem.compatibility)) : "",
        actionItem.contents ? I18n.tr("CONTENTS / %1").arg(valueText(actionItem.contents)) : ""
    ].filter(Boolean).join("\n")
    readonly property bool isBundle: String(actionItem.category || "") === "bundles"
    // A plugins-category item the registry did not mark official: the store shows
    // it, but Ryoku neither reviews nor maintains it. Drives the warning + tag.
    readonly property bool isCommunityPlugin: String(actionItem.category || "") === "plugins"
            && !!(actionItem.metadata && actionItem.metadata.community === true)
    readonly property var components: (actionItem.metadata && Array.isArray(actionItem.metadata.items)) ? actionItem.metadata.items : []
    readonly property var coreComponents: components.filter(c => String(c.tier || "core") !== "optional")
    readonly property var optionalComponents: components.filter(c => String(c.tier || "core") === "optional")
    // Bundle items may carry a `group` label. Fold a tier's items into ordered
    // sections by that label (first appearance wins) so a long bundle reads as
    // labelled blocks instead of one wall. Items with no group collapse into a
    // single unlabelled section, which is what an ungrouped bundle still gets.
    // A plain object keyed by label would inherit Object.prototype, so a group
    // named "constructor" or "toString" would silently merge into the wrong
    // section. A bundle has a handful of sections, so scan them.
    function groupSections(list) {
        var out = [];
        for (var i = 0; i < list.length; i++) {
            var label = String(list[i].group || "");
            var section = null;
            for (var j = 0; j < out.length; j++) {
                if (out[j].label === label) {
                    section = out[j];
                    break;
                }
            }
            if (section === null) {
                section = { label: label, items: [] };
                out.push(section);
            }
            section.items.push(list[i]);
        }
        return out;
    }
    readonly property var coreGroups: groupSections(coreComponents)
    readonly property var optionalGroups: groupSections(optionalComponents)
    property var sel: ({})
    property var lastComponents: null
    readonly property var selectedNames: components.filter(c => sel[String(c.name)] === true).map(c => String(c.name))
    readonly property var allNames: components.map(c => String(c.name))
    onItemChanged: { rebuildSelection(); galleryIndex = 0; }
    readonly property real targetX: Tokens.s6
    readonly property real targetY: Tokens.s6
    // An art-only product's dossier carries only text, so the hero can take more
    // of the page; a product with a gallery keeps the hero narrower so the dossier
    // stays wide enough for the preview beside it.
    readonly property real targetWidth: hasBodyContent
            ? Math.min(430, width * 0.42)
            : Math.round(Math.min(width * 0.5, 600))
    // The body carries a gallery (real screenshots) or a bundle's component list.
    // An art-only product has neither, so its dossier centres instead of pinning
    // a foot below a void.
    readonly property bool hasBodyContent: isBundle || screenshotCount > 0
    readonly property real targetHeight: Math.max(1, height - Tokens.s6 * 2)
    readonly property rect effectiveOrigin: originRect.width > 0 && originRect.height > 0
            ? originRect
            : Qt.rect(targetX, targetY, targetWidth, targetHeight)

    visible: open || transitionProgress > 0
    focus: open
    clip: true

    function valueText(value) {
        return Array.isArray(value) ? value.join(", ") : String(value || "");
    }

    function mix(from, to) {
        return from + (to - from) * transitionProgress;
    }

    function triggerClose() {
        closeRequested();
    }

    function triggerInstall() {
        if (item && busyKey === "" && StoreLogic.primaryAction(actionItem) !== "INSTALLED"
                && !StoreLogic.isDownloadPaused(actionItem) && !StoreLogic.isUnavailable(actionItem)) {
            lastComponents = null;
            installRequested(actionItem, ditherOn, null);
        }
    }

    function triggerInstallAll() {
        if (item && busyKey === "" && !StoreLogic.isDownloadPaused(actionItem)
                && !StoreLogic.isUnavailable(actionItem)) {
            lastComponents = allNames;
            installRequested(actionItem, false, allNames);
        }
    }

    function triggerInstallSelected() {
        if (item && busyKey === "" && selectedNames.length > 0 && !StoreLogic.isDownloadPaused(actionItem)
                && !StoreLogic.isUnavailable(actionItem)) {
            lastComponents = selectedNames;
            installRequested(actionItem, false, selectedNames);
        }
    }

    function triggerRetry() {
        if (item && busyKey === "" && errorText !== "" && !StoreLogic.isDownloadPaused(actionItem)
                && !StoreLogic.isUnavailable(actionItem))
            retryRequested(actionItem, ditherOn, lastComponents);
    }

    function rebuildSelection() {
        var it = item || {};
        var comps = (it.metadata && Array.isArray(it.metadata.items)) ? it.metadata.items : [];
        var next = {};
        for (var i = 0; i < comps.length; i++) {
            var c = comps[i];
            next[String(c.name)] = (c.installed === true) || (String(c.tier || "core") !== "optional");
        }
        sel = next;
    }

    function toggleSel(name) {
        var next = {};
        for (var k in sel)
            next[k] = sel[k];
        next[name] = !next[name];
        sel = next;
    }

    function triggerSettings() {
        if (item && StoreLogic.secondaryAction(actionItem) !== "")
            settingsRequested(actionItem);
    }

    function triggerRemove() {
        if (item && busyKey === "" && StoreLogic.isInstalled(actionItem))
            removeRequested(actionItem);
    }

    function openLightbox(i) {
        if (i >= 0 && i < galleryCount)
            lightboxIndex = i;
    }
    function closeLightbox() {
        lightboxIndex = -1;
    }
    function stepLightbox(delta) {
        if (galleryCount > 0)
            lightboxIndex = (lightboxIndex + delta + galleryCount) % galleryCount;
    }

    Behavior on transitionProgress {
        enabled: !detail.reducedMotion
        NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
    }

    // A provenance link as a small icon action: a Nerd Font glyph in a hairline
    // chip that opens the URL in the user's browser. Hidden when the product
    // names no such link. Mirrors the header's account-action affordance.
    component LinkChip: Rectangle {
        property string glyph: ""
        property string url: ""
        property string label: ""
        implicitWidth: 30
        implicitHeight: 26
        radius: Tokens.radius
        color: activeFocus || linkHover.hovered ? Tokens.tint10 : "transparent"
        border.width: Tokens.border
        border.color: activeFocus ? Tokens.bone : (linkHover.hovered ? Tokens.lineStrong : Tokens.line)
        activeFocusOnTab: visible
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Accessible.role: Accessible.Button
        Accessible.name: label
        Accessible.onPressAction: openLink()
        function openLink() { if (url !== "") Qt.openUrlExternally(url); }
        Keys.onPressed: event => {
            if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
                openLink();
                event.accepted = true;
            }
        }
        Text {
            anchors.centerIn: parent
            text: glyph
            color: parent.activeFocus ? Tokens.ink : Tokens.inkDim
            font.family: Tokens.mono
            font.pixelSize: Tokens.fBody
        }
        HoverHandler { id: linkHover; cursorShape: Qt.PointingHandCursor }
        TapHandler { onTapped: openLink() }
    }

    // Modal input sink: swallow every pointer event that misses an interactive
    // child so the browse grid behind the open dossier never reacts to a click,
    // hover, or scroll. Declared first so the cover and dossier stay on top.
    MouseArea {
        anchors.fill: parent
        enabled: detail.open
        hoverEnabled: true
        acceptedButtons: Qt.AllButtons
        onPressed: mouse => mouse.accepted = true
        onWheel: wheel => wheel.accepted = true
    }

    Rectangle {
        anchors.fill: parent
        color: detail.actionItem.surface || Tokens.paper
        opacity: detail.transitionProgress
    }

    ProductCover {
        id: cover
        objectName: "ryostore-detail-cover"
        x: detail.mix(detail.effectiveOrigin.x, detail.targetX)
        y: detail.mix(detail.effectiveOrigin.y, detail.targetY)
        width: detail.mix(detail.effectiveOrigin.width, detail.targetWidth)
        height: detail.mix(detail.effectiveOrigin.height, detail.targetHeight)
        item: detail.actionItem
        // the cover leads with the colour original, so the override is now the bake
        artOverride: detail.hasDither && detail.ditherOn ? String(detail.actionItem.art || "") : ""
        mode: "plate"
        // The cover is the shared-element hero; tapping it opens the preview at
        // full size, the same lightbox the gallery uses.
        HoverHandler { enabled: detail.galleryCount > 0; cursorShape: Qt.PointingHandCursor }
        TapHandler { enabled: detail.galleryCount > 0; onTapped: detail.openLightbox(detail.galleryIndex) }
    }

    Item {
        id: dossier
        objectName: "ryostore-detail-dossier"
        x: detail.targetX + detail.targetWidth + Tokens.s6
        y: detail.targetY
        width: Math.max(1, detail.width - x - Tokens.s6)
        height: detail.targetHeight
        opacity: detail.transitionProgress
        clip: true

        // An art-only product's dossier holds only text; centre the head and foot
        // as one panel so it reads as a framed hero beside balanced info rather
        // than a top-loaded block above a void. Driven by AnchorChanges, not
        // ternary anchors, so the sibling anchor rebinds cleanly.
        states: State {
            name: "centered"
            when: !detail.hasBodyContent
            AnchorChanges {
                target: dossierHead
                anchors.top: undefined
                anchors.verticalCenter: dossier.verticalCenter
            }
            PropertyChanges {
                target: dossierHead
                anchors.verticalCenterOffset: -(dossierFoot.height + Tokens.s4) / 2
            }
            AnchorChanges {
                target: dossierFoot
                anchors.bottom: undefined
                anchors.top: dossierHead.bottom
            }
            PropertyChanges {
                target: dossierFoot
                anchors.topMargin: Tokens.s4
            }
        }

        // Row delegate for the bundle component list (used by both tier repeaters).
        Component {
            id: componentDelegate
            Item {
                id: compRow
                required property var modelData
                readonly property string cname: String(modelData.name || "")
                readonly property bool ison: detail.sel[cname] === true
                readonly property bool isinstalled: modelData.installed === true
                width: bundleCol.width
                implicitHeight: rowInfo.implicitHeight + Tokens.s2
                height: implicitHeight

                Rectangle {
                    id: box
                    width: 16
                    height: 16
                    radius: Tokens.radius
                    anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                    color: (compRow.ison || compRow.isinstalled) ? Tokens.bone : "transparent"
                    border.width: Tokens.border
                    border.color: (compRow.ison || compRow.isinstalled) ? Tokens.bone : Tokens.line
                    Text {
                        anchors.centerIn: parent
                        visible: compRow.ison || compRow.isinstalled
                        text: "\u2713"
                        color: Tokens.inkOnBone
                        font.family: Tokens.ui
                        font.pixelSize: 11
                    }
                }

                Column {
                    id: rowInfo
                    anchors { left: box.right; leftMargin: Tokens.s3; right: parent.right; verticalCenter: parent.verticalCenter }
                    spacing: 0
                    Text {
                        width: parent.width
                        text: compRow.cname + (compRow.isinstalled ? I18n.tr("  \u00b7 INSTALLED") : "")
                        color: Tokens.ink
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fMicro
                        font.letterSpacing: Tokens.trackLabel
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        text: String(compRow.modelData.summary || "")
                        visible: text !== ""
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro
                        wrapMode: Text.Wrap
                        maximumLineCount: 2
                        elide: Text.ElideRight
                    }
                }

                HoverHandler { cursorShape: Qt.PointingHandCursor; enabled: !compRow.isinstalled }
                TapHandler { enabled: !compRow.isinstalled; onTapped: detail.toggleSel(compRow.cname) }
            }
        }

        // One labelled block of component rows, used by both tier repeaters. The
        // label sits below the tier mark in the hierarchy: dimmer ink, tighter
        // tracking. An unlabelled section renders as bare rows.
        Component {
            id: groupSectionDelegate
            Column {
                id: groupSection
                required property var modelData
                width: bundleCol.width
                spacing: Tokens.s2

                Text {
                    visible: String(groupSection.modelData.label) !== ""
                    text: I18n.tr(String(groupSection.modelData.label))
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fMicro
                    font.letterSpacing: Tokens.trackLabel
                    topPadding: Tokens.s1
                }

                Repeater { model: groupSection.modelData.items; delegate: componentDelegate }
            }
        }

        // HEAD: identity, description, tags, metadata (fixed at the top).
        Column {
            id: dossierHead
            anchors { top: parent.top; left: parent.left; right: parent.right }
            spacing: Tokens.s3

            Text {
                width: parent.width
                text: String(detail.actionItem.categoryName || detail.actionItem.category || "").toUpperCase()
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackMark
                elide: Text.ElideRight
            }

            Row {
                width: parent.width
                spacing: Tokens.s3

                Text {
                    id: detailTitle
                    objectName: "ryostore-detail-title"
                    width: parent.width - (communityTag.visible ? communityTag.width + parent.spacing : 0)
                    text: String(detail.actionItem.name || detail.actionItem.id || "")
                    color: Tokens.ink
                    font.family: Tokens.display
                    font.pixelSize: Tokens.fTitle
                    font.weight: Font.Medium
                    wrapMode: Text.Wrap
                    maximumLineCount: 2
                    elide: Text.ElideRight
                }

                // Sits where an OFFICIAL mark would: a small, colourless tag that
                // names an unreviewed community plugin next to its title.
                Rectangle {
                    id: communityTag
                    objectName: "ryostore-detail-community-tag"
                    visible: detail.isCommunityPlugin
                    anchors.verticalCenter: parent.verticalCenter
                    width: communityTagLabel.implicitWidth + Tokens.s3 * 2
                    height: communityTagLabel.implicitHeight + Tokens.s2
                    radius: Tokens.radius
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line

                    Text {
                        id: communityTagLabel
                        anchors.centerIn: parent
                        text: I18n.tr("COMMUNITY")
                        color: Tokens.inkDim
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fMicro
                        font.letterSpacing: Tokens.trackLabel
                    }
                }
            }

            Text {
                objectName: "ryostore-detail-description"
                width: parent.width
                text: String(detail.actionItem.description || detail.actionItem.summary || "")
                visible: text !== ""
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fBody
                wrapMode: Text.Wrap
                maximumLineCount: detail.isBundle ? 2 : 4
                elide: Text.ElideRight
            }

            Flow {
                objectName: "ryostore-detail-tags"
                width: parent.width
                spacing: Tokens.s2
                visible: detail.tags.length > 0

                Repeater {
                    model: detail.tags

                    delegate: Rectangle {
                        required property string modelData
                        width: tagLabel.implicitWidth + Tokens.s3 * 2
                        height: tagLabel.implicitHeight + Tokens.s2
                        radius: Tokens.radius
                        color: Qt.rgba(detail.accentColor.r, detail.accentColor.g, detail.accentColor.b, 0.12)
                        border.width: Tokens.border
                        border.color: Qt.rgba(detail.accentColor.r, detail.accentColor.g, detail.accentColor.b, 0.4)

                        Text {
                            id: tagLabel
                            anchors.centerIn: parent
                            text: parent.modelData.toUpperCase()
                            color: Tokens.ink
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro
                            font.letterSpacing: Tokens.trackLabel
                        }
                    }
                }
            }

            Text {
                objectName: "ryostore-detail-metadata"
                width: parent.width
                text: detail.metadataText
                visible: text !== ""
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
                wrapMode: Text.Wrap
            }

            // Provenance: the project's home (a github glyph) and, when it names
            // one, its community (a discord glyph). Rendered only when present.
            Row {
                objectName: "ryostore-detail-links"
                spacing: Tokens.s2
                readonly property string upstreamUrl: String(detail.actionItem.upstream || "")
                readonly property string discordUrl: String(detail.actionItem.discord || "")
                visible: upstreamUrl !== "" || discordUrl !== ""

                LinkChip {
                    objectName: "ryostore-detail-upstream"
                    glyph: "\uf09b"
                    url: parent.upstreamUrl
                    visible: url !== ""
                    label: I18n.tr("Open the project page")
                }

                LinkChip {
                    objectName: "ryostore-detail-discord"
                    glyph: "\uf1ff"
                    url: parent.discordUrl
                    visible: url !== ""
                    label: I18n.tr("Open the Discord community")
                }
            }

            // A community plugin is unreviewed third-party code that runs in the
            // shell with the user's permissions, so warn before trust. House
            // style: hairline frame, dim ink, a warning mark, no colour.
            Rectangle {
                objectName: "ryostore-detail-community-warning"
                width: parent.width
                visible: detail.isCommunityPlugin
                implicitHeight: communityWarningRow.implicitHeight + Tokens.s3 * 2
                height: implicitHeight
                radius: Tokens.radius
                color: "transparent"
                border.width: Tokens.border
                border.color: Tokens.line

                Row {
                    id: communityWarningRow
                    anchors {
                        left: parent.left; leftMargin: Tokens.s3
                        right: parent.right; rightMargin: Tokens.s3
                        verticalCenter: parent.verticalCenter
                    }
                    spacing: Tokens.s3

                    Text {
                        id: communityWarningMark
                        text: "\uf071"
                        color: Tokens.inkDim
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fBody
                    }

                    Text {
                        width: parent.width - communityWarningMark.width - parent.spacing
                        text: I18n.tr("Community plugin. Ryoku does not review or maintain it: it runs inside your shell with your permissions, so inspect its code before you trust it.")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall
                        wrapMode: Text.Wrap
                    }
                }
            }
        }

        // FOOT: status, error, and the action row, pinned to the bottom so the
        // buttons never scroll off no matter how long the component list is.
        Column {
            id: dossierFoot
            anchors { bottom: parent.bottom; left: parent.left; right: parent.right }
            spacing: Tokens.s3

            Text {
                objectName: "ryostore-detail-selected"
                width: parent.width
                visible: detail.isBundle && detail.components.length > 0
                text: I18n.tr("%1 / %2 SELECTED").arg(detail.selectedNames.length).arg(detail.components.length)
                color: Tokens.inkDim
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro
                font.letterSpacing: Tokens.trackLabel
            }

            StatusReadout {
                objectName: "ryostore-detail-status"
                item: detail.actionItem
                busyKey: detail.busyKey
                installStage: detail.installStage
                installErrorKey: detail.installErrorKey
                installError: detail.installError
            }

            Text {
                objectName: "ryostore-detail-foreign-wm"
                width: parent.width
                visible: StoreLogic.isUnavailable(detail.actionItem)
                text: StoreLogic.unavailableReason(detail.actionItem).length > 0
                    ? StoreLogic.unavailableReason(detail.actionItem)
                    : StoreLogic.unavailableLabel(detail.actionItem)
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.Wrap
                textFormat: Text.PlainText
                maximumLineCount: 3
                elide: Text.ElideRight
            }

            Text {
                objectName: "ryostore-detail-pause"
                width: parent.width
                visible: StoreLogic.isDownloadPaused(detail.actionItem)
                text: I18n.tr("Under construction.") + " " + StoreLogic.downloadPauseReason(detail.actionItem)
                color: Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.Wrap
                textFormat: Text.PlainText
            }

            Text {
                objectName: "ryostore-detail-error"
                width: parent.width
                text: detail.errorText
                visible: text !== ""
                color: Tokens.alert
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                wrapMode: Text.Wrap
                textFormat: Text.PlainText
            }

            Row {
                spacing: Tokens.s2

                Btn {
                    objectName: "ryostore-detail-dither"
                    visible: detail.hasDither
                    text: detail.ditherOn ? I18n.tr("DITHER / ON") : I18n.tr("DITHER / OFF")
                    armed: detail.busyKey === ""
                    Accessible.role: Accessible.Button
                    Accessible.name: text
                    onAct: detail.ditherOn = !detail.ditherOn
                    Accessible.onPressAction: detail.ditherOn = !detail.ditherOn
                }

                Btn {
                    objectName: "ryostore-detail-install-selected"
                    visible: detail.isBundle
                    text: detail.busyKey === detail.actionKey && detail.installStage !== ""
                            ? I18n.tr(detail.installStage)
                            : I18n.tr("INSTALL SELECTED")
                    primary: true
                    armed: detail.item !== null && detail.busyKey === "" && detail.selectedNames.length > 0
                            && !StoreLogic.isDownloadPaused(detail.actionItem) && !StoreLogic.isUnavailable(detail.actionItem)
                    Accessible.role: Accessible.Button
                    Accessible.name: text
                    onAct: detail.triggerInstallSelected()
                    Accessible.onPressAction: detail.triggerInstallSelected()
                }

                Btn {
                    objectName: "ryostore-detail-install-all"
                    visible: detail.isBundle
                    text: I18n.tr("INSTALL ALL")
                    armed: detail.item !== null && detail.busyKey === ""
                            && !StoreLogic.isDownloadPaused(detail.actionItem) && !StoreLogic.isUnavailable(detail.actionItem)
                    Accessible.role: Accessible.Button
                    Accessible.name: text
                    onAct: detail.triggerInstallAll()
                    Accessible.onPressAction: detail.triggerInstallAll()
                }

                Btn {
                    objectName: "ryostore-detail-install"
                    visible: !detail.isBundle
                    text: detail.busyKey === detail.actionKey && detail.installStage !== ""
                            ? I18n.tr(detail.installStage)
                            : I18n.tr(StoreLogic.primaryAction(detail.actionItem))
                    primary: true
                    armed: detail.item !== null && detail.busyKey === ""
                            && StoreLogic.primaryAction(detail.actionItem) !== "INSTALLED"
                            && !StoreLogic.isDownloadPaused(detail.actionItem)
                            && !StoreLogic.isUnavailable(detail.actionItem)
                    Accessible.role: Accessible.Button
                    Accessible.name: text
                    onAct: detail.triggerInstall()
                    Accessible.onPressAction: detail.triggerInstall()
                }

                Btn {
                    objectName: "ryostore-detail-retry"
                    text: I18n.tr("RETRY")
                    visible: detail.errorText !== ""
                    armed: visible && detail.busyKey === ""
                            && !StoreLogic.isDownloadPaused(detail.actionItem) && !StoreLogic.isUnavailable(detail.actionItem)
                    Accessible.role: Accessible.Button
                    Accessible.name: text
                    onAct: detail.triggerRetry()
                    Accessible.onPressAction: detail.triggerRetry()
                }

                Btn {
                    objectName: "ryostore-detail-remove"
                    text: I18n.tr("REMOVE")
                    visible: StoreLogic.isInstalled(detail.actionItem)
                    armed: visible && detail.busyKey === ""
                    Accessible.role: Accessible.Button
                    Accessible.name: text
                    onAct: detail.triggerRemove()
                    Accessible.onPressAction: detail.triggerRemove()
                }
                Btn {
                    objectName: "ryostore-detail-settings"
                    text: I18n.tr("OPEN IN SETTINGS")
                    visible: StoreLogic.secondaryAction(detail.actionItem) !== ""
                    armed: visible
                    Accessible.role: Accessible.Button
                    Accessible.name: text
                    onAct: detail.triggerSettings()
                    Accessible.onPressAction: detail.triggerSettings()
                }

                Btn {
                    id: closeButton
                    objectName: "ryostore-detail-close"
                    focus: detail.open
                    text: I18n.tr("BACK")
                    Accessible.role: Accessible.Button
                    Accessible.name: text
                    onAct: detail.triggerClose()
                    Accessible.onPressAction: detail.triggerClose()
                }
            }
        }

        // BODY: fills the space between head and foot. Bundles get the scrollable
        // component list here (as tall as the window allows); everything else gets
        // the preview gallery, which fills the same space.
        Item {
            id: dossierBody
            anchors {
                top: dossierHead.bottom
                topMargin: Tokens.s4
                bottom: dossierFoot.top
                bottomMargin: Tokens.s4
                left: parent.left
                right: parent.right
            }
            clip: true

            Flickable {
                id: bundleComponents
                objectName: "ryostore-detail-components"
                anchors.fill: parent
                visible: detail.isBundle && detail.components.length > 0
                contentWidth: width
                contentHeight: bundleCol.implicitHeight
                clip: true
                interactive: contentHeight > height
                boundsBehavior: Flickable.StopAtBounds

                Column {
                    id: bundleCol
                    width: bundleComponents.width - Tokens.s3
                    spacing: Tokens.s2

                    Text {
                        visible: detail.coreComponents.length > 0
                        text: I18n.tr("CORE")
                        color: Tokens.inkDim
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fMicro
                        font.letterSpacing: Tokens.trackMark
                    }
                    Repeater { model: detail.coreGroups; delegate: groupSectionDelegate }

                    Text {
                        visible: detail.optionalComponents.length > 0
                        text: I18n.tr("OPTIONAL")
                        color: Tokens.inkDim
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fMicro
                        font.letterSpacing: Tokens.trackMark
                        topPadding: Tokens.s2
                    }
                    Repeater { model: detail.optionalGroups; delegate: groupSectionDelegate }
                }

                Rectangle {
                    id: scrollThumb
                    width: 3
                    radius: 1.5
                    color: bundleComponents.moving ? Tokens.inkDim : Tokens.line
                    visible: bundleComponents.interactive
                    x: bundleComponents.width - width
                    height: Math.max(30, bundleComponents.height * bundleComponents.height / Math.max(1, bundleComponents.contentHeight))
                    y: bundleComponents.contentY + (bundleComponents.contentHeight > bundleComponents.height
                        ? (bundleComponents.contentY / (bundleComponents.contentHeight - bundleComponents.height)) * (bundleComponents.height - scrollThumb.height)
                        : 0)
                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                }
            }

            // The gallery fills the body: a large main preview and, when the
            // product ships more than one shot, a thumbnail rail beneath it.
            // Tapping the preview opens the lightbox at full size.
            Item {
                id: gallery
                objectName: "ryostore-detail-screenshots"
                anchors.fill: parent
                visible: !detail.isBundle && detail.screenshotCount > 0

                readonly property bool hasRail: detail.galleryCount > 1
                readonly property int railHeight: hasRail ? 72 : 0

                ProductMedia {
                    id: mainPreview
                    anchors { top: parent.top; left: parent.left; right: parent.right }
                    height: parent.height - gallery.railHeight - (gallery.hasRail ? Tokens.s2 : 0)
                    source: detail.galleryShots[detail.galleryIndex] || ""
                    mode: "plate"
                    surface: detail.actionItem.surface || Tokens.paper
                    active: detail.open

                    HoverHandler { id: previewHover; cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: detail.openLightbox(detail.galleryIndex) }
                    Accessible.role: Accessible.Button
                    Accessible.name: I18n.tr("Preview %1 of %2, expand").arg(detail.galleryIndex + 1).arg(detail.galleryCount)
                    Accessible.onPressAction: detail.openLightbox(detail.galleryIndex)

                    // An expand mark that brightens on hover, so the preview reads
                    // as tappable rather than a flat picture.
                    Rectangle {
                        anchors { top: parent.top; right: parent.right; margins: Tokens.s3 }
                        width: expandMark.implicitWidth + Tokens.s2 * 2
                        height: expandMark.implicitHeight + Tokens.s1 * 2
                        radius: Tokens.radius
                        color: "#8c000000"
                        border.width: Tokens.border
                        border.color: previewHover.hovered
                                ? Qt.rgba(detail.accentColor.r, detail.accentColor.g, detail.accentColor.b, 0.9)
                                : "#33ffffff"
                        Text {
                            id: expandMark
                            anchors.centerIn: parent
                            text: "\uf065"
                            color: "#e8ffffff"
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro
                        }
                    }
                }

                Flickable {
                    id: rail
                    visible: gallery.hasRail
                    anchors { bottom: parent.bottom; left: parent.left; right: parent.right }
                    height: gallery.railHeight
                    contentWidth: railRow.width
                    contentHeight: height
                    clip: true
                    flickableDirection: Flickable.HorizontalFlick
                    boundsBehavior: Flickable.StopAtBounds

                    Row {
                        id: railRow
                        height: parent.height
                        spacing: Tokens.s2

                        Repeater {
                            model: detail.galleryShots

                            delegate: Item {
                                id: thumb
                                required property var modelData
                                required property int index
                                readonly property bool current: detail.galleryIndex === thumb.index
                                width: Math.round(rail.height * 16 / 9)
                                height: rail.height
                                Accessible.role: Accessible.Button
                                Accessible.name: I18n.tr("Show preview %1 of %2").arg(index + 1).arg(detail.galleryCount)
                                Accessible.onPressAction: detail.galleryIndex = thumb.index

                                ProductMedia {
                                    anchors.fill: parent
                                    source: thumb.modelData
                                    mode: "cover"
                                    active: detail.open
                                }

                                Rectangle {
                                    anchors.fill: parent
                                    color: "transparent"
                                    border.width: (thumb.current || thumbHover.hovered) ? Tokens.border * 2 : Tokens.border
                                    border.color: thumb.current
                                            ? Qt.rgba(detail.accentColor.r, detail.accentColor.g, detail.accentColor.b, 0.9)
                                            : (thumbHover.hovered ? "#66ffffff" : "#28ffffff")
                                }

                                HoverHandler { id: thumbHover; cursorShape: Qt.PointingHandCursor }
                                TapHandler { onTapped: detail.galleryIndex = thumb.index }
                            }
                        }
                    }
                }
            }
        }
    }

    // Lightbox: a tapped screenshot fills the dossier so it can actually be
    // read, with a shot counter and prev/next; a tap on the backdrop or Escape
    // (via the app) closes it. Declared last so it sits above the dossier.
    Item {
        objectName: "ryostore-detail-lightbox"
        anchors.fill: parent
        visible: detail.lightboxOpen
        z: 5

        Rectangle {
            anchors.fill: parent
            color: Qt.rgba(0, 0, 0, 0.92)
            TapHandler { onTapped: detail.closeLightbox() }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
        }

        ProductMedia {
            anchors.fill: parent
            anchors.margins: Tokens.s7
            source: detail.lightboxOpen ? detail.galleryShots[detail.lightboxIndex] : ""
            mode: "plate"
            active: detail.lightboxOpen
        }

        Text {
            anchors { top: parent.top; horizontalCenter: parent.horizontalCenter; topMargin: Tokens.s5 }
            text: String(detail.lightboxIndex + 1) + " / " + String(detail.galleryCount)
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            font.letterSpacing: Tokens.trackLabel
        }

        Btn {
            anchors { left: parent.left; verticalCenter: parent.verticalCenter; leftMargin: Tokens.s4 }
            text: I18n.tr("PREV")
            visible: detail.galleryCount > 1
            armed: visible
            onAct: detail.stepLightbox(-1)
            Accessible.role: Accessible.Button
            Accessible.name: I18n.tr("Previous screenshot")
            Accessible.onPressAction: detail.stepLightbox(-1)
        }

        Btn {
            anchors { right: parent.right; verticalCenter: parent.verticalCenter; rightMargin: Tokens.s4 }
            text: I18n.tr("NEXT")
            visible: detail.galleryCount > 1
            armed: visible
            onAct: detail.stepLightbox(1)
            Accessible.role: Accessible.Button
            Accessible.name: I18n.tr("Next screenshot")
            Accessible.onPressAction: detail.stepLightbox(1)
        }
    }
}
