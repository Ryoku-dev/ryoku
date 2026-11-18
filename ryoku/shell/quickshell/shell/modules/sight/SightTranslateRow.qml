pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    required property var panel
    required property var catalog
    property real s: 1
    property string source: ""
    property string sourceOverride: ""
    property string target: ""
    readonly property bool popupOpen: languages.visible || (styleLoader.item !== null && styleLoader.item.visible)
    property bool choosingSource: false
    property string query: ""
    readonly property var choices: {
        const value = query.trim().toLowerCase();
        const automatic = choosingSource ? I18n.tr("Detect automatically") : I18n.tr("Your language");
        const selected = choosingSource ? sourceOverride : target;
        const rows = [{ label: automatic, code: "", glyph: selected === "" ? "check" : "" }];
        for (var i = 0; i < I18n.langs.length; ++i) {
            const language = I18n.langs[i];
            const name = String(language.name);
            const nativeName = String(language.native || "");
            const code = String(language.code);
            if (value && name.toLowerCase().indexOf(value) < 0 && nativeName.toLowerCase().indexOf(value) < 0 && code.toLowerCase().indexOf(value) < 0)
                continue;
            const label = I18n.tr(name);
            rows.push({ label: label, shortcut: nativeName && nativeName !== label ? nativeName : "", code: code, glyph: selected === code ? "check" : "" });
        }
        return rows;
    }
    readonly property string sourceName: source ? I18n.tr(panel.languageName(source)) : I18n.tr("Detect language")
    readonly property string targetName: target ? I18n.tr(panel.languageName(target)) : I18n.tr("Your language")
    signal sourceRequested(string source)
    signal targetRequested(string target)
    readonly property bool styleEnabled: catalog.feature("translatestyle")
    readonly property string translationStyle: String(catalog.option("translate.style", "natural"))
    readonly property bool simpler: catalog.option("translate.simpler", false) === true
    readonly property bool showBack: styleEnabled && catalog.option("translate.back", false) === true
    readonly property var requestOptions: {
        if (!styleEnabled) return ({});
        const options = { style: translationStyle };
        if (simpler) options.simpler = "on";
        return options;
    }
    readonly property string sourceText: String(panel.ocrDisplay || "").trim()
    readonly property int sourceWords: sourceText ? sourceText.split(/\s+/).length : 0
    readonly property bool dictionaryEnabled: catalog.feature("dictionary") && sourceWords > 0 && sourceWords <= 3
    readonly property bool pictureEnabled: catalog.feature("overimage") && panel.card.kind === "image"
    readonly property bool hasExtras: styleEnabled || dictionaryEnabled || pictureEnabled
    property bool onPicture: false
    property bool pictureBusy: false
    property string pictureError: ""
    property int pictureSerial: 0
    property int backSerial: 0
    property string backRequest: ""
    property string pictureRequest: ""
    property bool backBusy: false
    property string backText: ""
    property string backError: ""
    property var lifetime: ({ closed: false })
    readonly property string pictureDirection: JSON.stringify([source, panel.effectiveTarget])
    readonly property string backKey: visible && !panel.closing && showBack && !panel.session.busy && panel.session.queued === null
        && String(panel.session.answers.translate || "").trim().length > 0
        ? JSON.stringify([panel.session.answers.translate, source, panel.effectiveTarget]) : ""
    readonly property Component backTranslationView: Component {
        Column {
            spacing: 6 * root.s
            readonly property real lithicScale: root.s
            Label { width: parent.width; text: I18n.tr("Translated back"); typeRole: "detail"; inkRole: "fg3" }
            Row {
                visible: root.backBusy
                spacing: 8 * root.s
                Working { active: root.backBusy && root.visible; size: 14 * root.s; anchors.verticalCenter: parent.verticalCenter }
                Label { text: I18n.tr("Translating back"); typeRole: "detail"; inkRole: "fg3" }
            }
            TextEdit {
                width: parent.width
                visible: root.backText.length > 0
                text: root.backText
                textFormat: TextEdit.PlainText
                color: Lithic.fg2
                font.family: Lithic.type.body.family
                font.pixelSize: 14 * root.s
                wrapMode: TextEdit.Wrap
                readOnly: true
                selectByMouse: true
                selectionColor: Lithic.sel
                selectedTextColor: Lithic.onSel
                Keys.forwardTo: [root.panel.inputItem]
            }
            Label {
                width: parent.width
                visible: root.backError.length > 0
                text: root.backError
                typeRole: "detail"; inkRole: "jasper"; wrapMode: Text.Wrap; elide: Text.ElideNone
            }
            SightActionButton {
                visible: root.backError.length > 0 && root.source.length > 0
                s: root.s; text: I18n.tr("Try again")
                onClicked: root.refreshBack()
            }
        }
    }
    implicitHeight: 32 * s + (extras.active && extras.item ? extras.item.implicitHeight + 8 * s : 0)
        + (pictureStatus.active && pictureStatus.item ? pictureStatus.item.implicitHeight + 8 * s : 0)
    function close() {
        languages.close();
        if (styleLoader.item) styleLoader.item.close();
    }
    function openStyle() {
        if (!styleEnabled || !styleLoader.item) return;
        if (styleLoader.item.visible) { styleLoader.item.close(); return; }
        languages.close();
        panel.preparePopup("translate-style");
        styleLoader.item.open();
    }
    function changeTranslation(key, value) {
        if (key === "back") return;
        const options = Object.assign({}, requestOptions);
        if (key === "style") options.style = value;
        else if (value === true) options.simpler = "on";
        else delete options.simpler;
        panel.resetTranslation();
        panel.runAction("translate", options);
        if (onPicture) translatePicture(options);
    }
    function refreshBack() {
        const serial = ++backSerial;
        if (backRequest) panel.session.cancelWriting(backRequest);
        backRequest = "";
        backBusy = false;
        backText = "";
        backError = "";
        if (!backKey || lifetime.closed) return;
        const originalLanguage = panel.languageName(source);
        if (!originalLanguage) {
            backError = I18n.tr("Choose a source language to translate back");
            return;
        }
        backBusy = true;
        const token = lifetime;
        const key = backKey;
        backRequest = panel.session.transform({
            action: "translate", text: String(panel.session.answers.translate || ""),
            source: panel.languageName(panel.effectiveTarget), target: originalLanguage
        }, function(result, error) {
            if (token.closed || serial !== root.backSerial || key !== root.backKey) return;
            root.backRequest = "";
            root.backBusy = false;
            if (error) root.backError = String(error.message || I18n.tr("Could not translate back"));
            else {
                root.backText = String(result && result.text || "");
                if (!root.backText) root.backError = I18n.tr("No translation came back");
            }
        });
    }
    function clearPicture() {
        ++pictureSerial;
        if (pictureRequest) panel.session.cancelWriting(pictureRequest);
        pictureRequest = "";
        pictureBusy = false;
        pictureError = "";
        panel.overlayLines = [];
    }
    function togglePicture() {
        onPicture = !onPicture;
        if (onPicture) translatePicture(requestOptions);
        else clearPicture();
    }
    function translatePicture(options) {
        clearPicture();
        if (!onPicture || !pictureEnabled || !visible || lifetime.closed) return;
        const serial = pictureSerial;
        const token = lifetime;
        const chosenOptions = Object.assign({}, options || requestOptions);
        const from = panel.languageName(source);
        const to = panel.languageName(panel.effectiveTarget);
        pictureBusy = true;
        Bus.call("sight.ocrBoxes", { id: String(panel.card.id) }, function(ok, result) {
            if (token.closed || serial !== root.pictureSerial) return;
            if (!ok) {
                root.pictureBusy = false;
                root.pictureError = String(result && result.message || I18n.tr("Could not read text positions"));
                return;
            }
            const boxes = result && result.lines || [];
            if (boxes.length === 0) {
                root.pictureBusy = false;
                root.pictureError = I18n.tr("No text was found in this picture");
                return;
            }
            const texts = [];
            for (var i = 0; i < boxes.length; ++i) texts.push(String(boxes[i].text || ""));
            root.pictureRequest = root.panel.session.transform({
                action: "lines", lines: texts, source: from, target: to, options: chosenOptions
            }, function(answer, error) {
                if (token.closed || serial !== root.pictureSerial) return;
                root.pictureRequest = "";
                root.pictureBusy = false;
                if (error) {
                    root.pictureError = String(error.message || I18n.tr("Could not translate the picture"));
                    return;
                }
                const translated = answer && answer.lines || [];
                if (translated.length !== boxes.length) {
                    root.pictureError = I18n.tr("The translated lines did not match the picture");
                    return;
                }
                const lines = [];
                for (var i = 0; i < boxes.length; ++i) {
                    const box = boxes[i];
                    lines.push({ text: String(translated[i]), x: box.x, y: box.y, width: box.width, height: box.height });
                }
                root.panel.overlayLines = lines;
            });
        });
    }
    onBackKeyChanged: Qt.callLater(refreshBack)
    onPictureEnabledChanged: if (!pictureEnabled) { onPicture = false; clearPicture(); }
    onPictureDirectionChanged: if (onPicture) translatePicture(requestOptions)
    onVisibleChanged: if (!visible) { close(); onPicture = false; clearPicture(); }
    Component.onDestruction: {
        lifetime.closed = true;
        ++backSerial;
        ++pictureSerial;
        if (backRequest) panel.session.cancelWriting(backRequest);
        if (pictureRequest) panel.session.cancelWriting(pictureRequest);
    }
    Connections {
        target: root.panel
        function onClosingChanged() {
            if (root.panel.closing) { root.onPicture = false; root.clearPicture(); }
        }
    }
    function open(sourceMenu) {
        if (languages.visible && choosingSource === sourceMenu) { languages.close(); return; }
        languages.close();
        if (styleLoader.item) styleLoader.item.close();
        choosingSource = sourceMenu;
        query = "";
        panel.preparePopup("language");
        languages.open();
    }
    Row {
        width: parent.width
        spacing: 8 * root.s
        SightActionButton {
            id: sourceChip
            s: root.s; text: root.sourceName
            width: Math.max(1, (root.width - arrow.width - 16 * root.s) / 2)
            contentItem: Item {
                Label { anchors.left: parent.left; anchors.right: sourceChevron.left; anchors.rightMargin: 6 * root.s; anchors.verticalCenter: parent.verticalCenter; text: sourceChip.text; typeRole: "label"; inkRole: "fg2"; elide: Text.ElideRight }
                Glyph { id: sourceChevron; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; name: "expand_more"; size: 17 * root.s; ink: Lithic.fg2 }
            }
            onClicked: root.open(true)
        }
        Glyph { id: arrow; anchors.verticalCenter: parent.verticalCenter; name: "arrow_forward"; size: 17 * root.s; ink: Lithic.fg3 }
        SightActionButton {
            id: targetChip
            s: root.s; text: root.targetName
            width: Math.max(1, (root.width - arrow.width - 16 * root.s) / 2)
            contentItem: Item {
                Label { anchors.left: parent.left; anchors.right: targetChevron.left; anchors.rightMargin: 6 * root.s; anchors.verticalCenter: parent.verticalCenter; text: targetChip.text; typeRole: "label"; inkRole: "fg2"; elide: Text.ElideRight }
                Glyph { id: targetChevron; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; name: "expand_more"; size: 17 * root.s; ink: Lithic.fg2 }
            }
            onClicked: root.open(false)
        }
    }
    Loader {
        id: extras
        y: 40 * root.s
        width: parent.width
        active: root.visible && root.hasExtras
        sourceComponent: Component {
            Flow {
                width: extras.width
                spacing: 6 * root.s
                readonly property real lithicScale: root.s
                readonly property var styleAnchor: styleChip
                TheiaChip {
                    id: styleChip
                    s: root.s
                    visible: root.styleEnabled
                    text: I18n.tr("Style")
                    icon: "tune"
                    selected: styleLoader.item !== null && styleLoader.item.visible
                    onClicked: root.openStyle()
                }
                TheiaChip {
                    s: root.s
                    visible: root.dictionaryEnabled
                    text: I18n.tr("Dictionary")
                    icon: "dictionary"
                    enabled: !root.panel.busy
                    onClicked: root.panel.runAction("define")
                }
                TheiaChip {
                    s: root.s
                    visible: root.pictureEnabled
                    text: I18n.tr("On the picture")
                    icon: "image"
                    selected: root.onPicture
                    onClicked: root.togglePicture()
                }
            }
        }
    }
    Loader {
        id: styleLoader
        active: root.visible && root.styleEnabled && extras.item !== null
        sourceComponent: Component {
            TheiaTranslateStyle {
                panel: root.panel; catalog: root.catalog; s: root.s
                parent: root
                anchorItem: extras.item ? extras.item.styleAnchor : root
                opener: anchorItem
                onOptionChanged: (key, value) => root.changeTranslation(key, value)
            }
        }
    }
    Loader {
        id: pictureStatus
        y: extras.y + extras.height + 8 * root.s
        width: parent.width
        active: root.visible && root.pictureEnabled && (root.pictureBusy || root.pictureError.length > 0)
        sourceComponent: Component {
            Column {
                width: pictureStatus.width
                spacing: 6 * root.s
                readonly property real lithicScale: root.s
                Row {
                    visible: root.pictureBusy
                    spacing: 8 * root.s
                    Working { active: root.pictureBusy && root.visible; size: 14 * root.s; anchors.verticalCenter: parent.verticalCenter }
                    Label { text: I18n.tr("Translating the picture"); typeRole: "detail"; inkRole: "fg3" }
                }
                Label {
                    width: parent.width
                    visible: root.pictureError.length > 0
                    text: root.pictureError
                    typeRole: "detail"; inkRole: "jasper"; wrapMode: Text.Wrap; elide: Text.ElideNone
                }
                SightActionButton {
                    visible: root.pictureError.length > 0
                    s: root.s; text: I18n.tr("Try again")
                    onClicked: root.translatePicture(root.requestOptions)
                }
            }
        }
    }
    Popover {
        id: languages
        parent: root
        anchorItem: root.choosingSource ? sourceChip : targetChip
        opener: root.choosingSource ? sourceChip : targetChip
        menuFocus: false
        closePolicy: T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutsideParent
        implicitWidth: Math.min(320 * root.s, Math.max(1, root.panel.usable.width
                                   - LithicTokens.size.popoverAnchorGap * 2 * root.s))
        implicitHeight: search.implicitHeight + languageBody.spacing + menu.implicitHeight
        function calculatePlacement() {
            return root.panel.popupPlacement(anchorItem, implicitWidth, implicitHeight, root);
        }
        onOpened: search.forceActiveFocus()
        CutRect {
            implicitWidth: languages.implicitWidth
            implicitHeight: languages.implicitHeight
            fill: Lithic.stone4
            strokeColor: Lithic.vein
            strokeWidth: LithicTokens.size.hairline
            cut: LithicTokens.cut.menu
            Column {
                id: languageBody
                anchors.fill: parent
                readonly property real lithicScale: root.s
                spacing: LithicTokens.size.menuPaddingVertical * root.s
                TextField {
                    id: search
                    width: parent.width
                    height: implicitHeight
                    placeholderText: I18n.tr("Find a language")
                    text: root.query
                    onTextEdited: root.query = text
                    Keys.onDownPressed: menu.focusItem(-1, true)
                }
                Menu {
                    id: menu
                    width: parent.width
                    height: Math.min(implicitHeight,
                                     Math.max(0, languageBody.height - search.height - languageBody.spacing))
                    menuWidth: languages.implicitWidth
                    background: null
                    items: root.choices
                    onTriggered: index => {
                        const code = String(root.choices[index].code);
                        if (root.choosingSource) root.sourceRequested(code);
                        else root.targetRequested(code);
                        languages.close();
                    }
                }
            }
        }
    }
}
