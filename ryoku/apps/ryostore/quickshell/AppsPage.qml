import QtQuick
import Ryoku.Ui.Singletons
import "lib/store.js" as StoreLogic

// The Apps catalogue: a featured shelf, a sort strip and compact app tiles.
// Other categories keep the artwork grid in App.qml.
Item {
    id: page

    property var items: []
    property string selectedKey: ""
    property string sortKey: "trending"
    property bool reducedMotion: false
    property string contentKey: ""
    property int seed: 0

    signal selectionRequested(var item)
    signal activated(var item)
    signal sortRequested(string key)

    readonly property var sortTabs: [
        { "key": "trending", "label": I18n.tr("TRENDING") },
        { "key": "popular", "label": I18n.tr("POPULAR") },
        { "key": "new", "label": I18n.tr("NEW") },
        { "key": "updated", "label": I18n.tr("UPDATED") }
    ]
    readonly property var featured: StoreLogic.featuredApps(items, 5, seed)
    readonly property real bodyTop: productGrid.y

    property alias grid: productGrid

    AppsHero {
        id: hero
        objectName: "ryostore-apps-hero"
        anchors {
            left: parent.left; leftMargin: Tokens.s6
            right: parent.right; rightMargin: Tokens.s6
            top: parent.top; topMargin: Tokens.s4
        }
        height: 192
        items: page.featured
        reducedMotion: page.reducedMotion
        onActivated: item => page.activated(item)
    }

    ProviderTabs {
        id: sortStrip
        objectName: "ryostore-apps-sort"
        anchors { left: parent.left; right: parent.right; top: hero.bottom }
        providers: page.sortTabs
        active: page.sortKey
        allLabel: ""
        trailingLabel: ""
        installableCount: 0
        reducedMotion: page.reducedMotion
        onPicked: key => page.sortRequested(key)
    }

    ProductGrid {
        id: productGrid
        objectName: "ryostore-apps-grid"
        anchors {
            left: parent.left; leftMargin: Tokens.s6
            right: parent.right; rightMargin: Tokens.s6
            top: sortStrip.bottom; topMargin: Tokens.s3
            bottom: parent.bottom
        }
        variant: "app"
        items: page.items
        selectedKey: page.selectedKey
        reducedMotion: page.reducedMotion
        contentKey: page.contentKey
        onSelectionRequested: item => page.selectionRequested(item)
        onActivated: item => page.activated(item)
    }
}
