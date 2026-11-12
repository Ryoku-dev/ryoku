import QtQuick
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/fonts-icons"
import "../schema/fonts-icons.js" as Schema

// typefaces, text size, icons, the pointer and the desktop's glosses
Item {
    id: root

    required property var hub
    readonly property var store: Changes.stores["fonts-icons"] || null
    readonly property var pointerOptions: [{ value: "DYNAMIC", label: I18n.tr("Follow the wallpaper") }]
        .concat(root.store ? root.store.pointerThemes : [])

    function focusRow(rowId) {
        page.focusRow(rowId);
    }

    Component.onCompleted: Changes.ensureStore("fonts-icons", Qt.resolvedUrl("../components/fonts-icons/LookStore.qml"))

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "fonts-icons"
        schema: Schema
        controls: ({
            "interface-font": interfaceFont,
            "monospace-font": monospaceFont,
            "icon-theme": iconTheme,
            "pointer-theme": pointerTheme
        })
    }

    Component {
        id: interfaceFont
        ChoiceField {
            dialogTitle: I18n.tr("Interface font")
            options: root.store ? root.store.fonts : []
            fontPreview: true
            failure: root.store ? root.store.fontsFailure : ""
            overlayParent: root.hub.overlayItem
            onRetry: root.store.readFonts()
        }
    }

    Component {
        id: monospaceFont
        ChoiceField {
            dialogTitle: I18n.tr("Monospace font")
            options: root.store ? root.store.monoFonts : []
            fontPreview: true
            failure: root.store ? root.store.fontsFailure : ""
            overlayParent: root.hub.overlayItem
            onRetry: root.store.readFonts()
        }
    }

    Component {
        id: iconTheme
        ChoiceField {
            dialogTitle: I18n.tr("Icon theme")
            options: root.store ? root.store.iconThemes : []
            busy: root.store ? root.store.applying : false
            emptyText: root.store && root.store.iconsFailure !== "" ? I18n.tr("Could not read the installed icon themes") : I18n.tr("Reading installed icon themes")
            failure: root.store ? root.store.iconsFailure : ""
            overlayParent: root.hub.overlayItem
            onRetry: root.store.readIcons()
        }
    }

    Component {
        id: pointerTheme
        ChoiceField {
            dialogTitle: I18n.tr("Pointer style")
            options: root.pointerOptions
            failure: root.store ? root.store.pointersFailure : ""
            overlayParent: root.hub.overlayItem
            onRetry: root.store.readPointers()
        }
    }
}
