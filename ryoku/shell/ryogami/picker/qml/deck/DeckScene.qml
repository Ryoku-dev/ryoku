import QtQuick

// The kwepaperpicker deck, ported into Ryogami's picker. CardField remains
// the state and input owner; this scene only draws the hand over it.
Item {
    id: deck

    property QtObject controller: null
    property QtObject field: null
    property QtObject keymap: null

    readonly property var view: controller ? controller.view : null
    readonly property bool active: !!controller && controller.mode === "deck" && controller.collection === "wallpapers"
    readonly property int count: view ? view.count : 0

    readonly property real u: Math.max(0.6, Math.min(width / 1920, height / 1200))
    readonly property int side: 7
    readonly property int slots: 2 * side + 3
    readonly property real cardW: 212 * u
    readonly property real pivotX: width / 2
    readonly property real pivotY: height + 1650 * u
    readonly property real handR: pivotY - (height - 196 * u)
    readonly property real lift: 66 * u
    readonly property real frontScale: 1.22
    readonly property real step1: 204 * u / handR
    readonly property real stepN: 132 * u / handR

    property SettingValue navSetting: SettingValue { key: "components.wallpaperSelector.deckNavigationMs" }
    property SettingValue swatchesSetting: SettingValue { key: "components.wallpaperSelector.deckSwatches" }
    property SettingValue shadowsSetting: SettingValue { key: "components.wallpaperSelector.deckShadows" }

    readonly property int navMs: {
        const v = Number(deck.navSetting.value)
        return isNaN(v) ? 360 : Math.max(1, Math.min(3000, v))
    }
    readonly property bool showSwatches: !(deck.swatchesSetting.value === false)
    readonly property bool showShadows: !(deck.shadowsSetting.value === false)

    property real pos: field ? field.currentIndex : 0
    property real deal: 0
    property bool dragging: false
    property real dragStart: 0

    Behavior on pos {
        NumberAnimation { duration: deck.navMs; easing.type: Easing.OutCubic }
    }
    Behavior on deal {
        NumberAnimation { duration: 560; easing.type: Easing.OutCubic }
    }
    Behavior on opacity {
        NumberAnimation { duration: 200 }
    }

    DeckCard {
        id: gauge
        u: deck.u
        showShadow: false
        visible: false
    }
    readonly property real cardH: gauge.implicitHeight
    readonly property real frontTop: pivotY - handR - lift - cardH * frontScale / 2

    opacity: active ? 1 : 0
    visible: opacity > 0.01

    function clamp(value, low, high) {
        return Math.max(low, Math.min(high, value))
    }

    readonly property int frontIndex: count > 0 ? deck.clamp(Math.round(deck.pos), 0, count - 1) : -1

    function baseName(path) {
        if (!path)
            return ""
        const text = String(path)
        const cut = Math.max(text.lastIndexOf("/"), text.lastIndexOf("\\"))
        return cut >= 0 ? text.slice(cut + 1) : text
    }

    function pictureOf(entry) {
        if (!entry)
            return ""
        const path = entry.fullImage || entry.thumb || entry.path || ""
        return path.charAt(0) === "/" ? "file://" + path : path
    }

    function fileName(entry) {
        if (!entry)
            return ""
        return deck.baseName(entry.name || deck.baseName(entry.path))
    }

    function extensionOf(entry) {
        const name = deck.fileName(entry)
        const dot = name.lastIndexOf(".")
        return dot > 0 ? name.slice(dot + 1).toUpperCase() : ""
    }

    function bytes(n) {
        const value = Number(n) || 0
        if (value <= 0)
            return ""
        return value >= 1048576
            ? (value / 1048576).toFixed(1) + " MB"
            : Math.max(1, Math.round(value / 1024)) + " KB"
    }

    function titleOf(entry) {
        const name = deck.fileName(entry)
        const dot = name.lastIndexOf(".")
        return dot > 0 ? name.slice(0, dot) : name
    }

    function metaOf(entry) {
        if (!entry)
            return ""
        const parts = []
        const ext = deck.extensionOf(entry)
        if (ext)
            parts.push(ext)
        const w = Number(entry.width) || 0
        const h = Number(entry.height) || 0
        if (w > 0 && h > 0)
            parts.push(w + " \u00d7 " + h)
        const size = deck.bytes(entry.filesize)
        if (size)
            parts.push(size)
        return parts.join("  \u00b7  ")
    }

    function requestPalettes() {
        if (!view || count <= 0)
            return
        const low = Math.max(0, Math.floor(pos) - side - 2)
        const high = Math.min(count - 1, Math.ceil(pos) + side + 2)
        const wanted = []
        for (let i = low; i <= high; ++i) {
            const entry = view.get(i)
            if (entry && entry.key)
                wanted.push({
                    key: entry.key,
                    path: entry.path,
                    thumb: entry.thumb,
                    fullImage: entry.fullImage,
                    mtime: entry.mtime
                })
        }
        DeckPalettes.ensure(wanted)
    }

    function choose(index) {
        if (!field || index < 0 || index >= count)
            return
        if (index === field.currentIndex)
            keymap.mouseGesture(false, Qt.NoModifier, index)
        else
            field.currentIndex = index
    }

    function apply(index) {
        if (!field || index < 0 || index >= count)
            return
        field.currentIndex = index
        keymap.mouseGesture(false, Qt.NoModifier, index)
    }

    function applyWheel(angleY, pixelY) {
        if (!field || count <= 0)
            return
        const amount = pixelY !== 0 ? pixelY / 50 : angleY / 120
        const steps = Math.round(amount)
        if (steps !== 0)
            field.currentIndex = clamp(field.currentIndex + steps, 0, count - 1)
    }

    function applyDrag(dx, released) {
        if (!field || count <= 0)
            return
        if (!deck.dragging) {
            deck.dragging = true
            deck.dragStart = deck.pos
        }
        if (released) {
            deck.dragging = false
            field.currentIndex = clamp(Math.round(deck.pos), 0, count - 1)
            return
        }
        const stride = stepN * handR
        deck.pos = clamp(deck.dragStart - dx / stride, -0.45, count - 0.55)
    }

    function schemeOf(entry) {
        DeckPalettes.revision
        return DeckPalettes.scheme(entry)
    }

    readonly property var ribbonColors: {
        DeckPalettes.revision
        const out = []
        if (!view)
            return out
        for (let i = 0; i < count; ++i)
            out.push(DeckPalettes.swatch(view.get(i)) || "")
        return out
    }

    readonly property var ribbonFavourites: {
        if (!view)
            return []
        const out = []
        for (let i = 0; i < count; ++i) {
            const entry = view.get(i)
            if (entry && entry.favourite)
                out.push(i)
        }
        return out
    }

    readonly property var ribbonThumbs: {
        if (!view)
            return []
        const out = []
        for (let i = 0; i < count; ++i)
            out.push(pictureOf(view.get(i)))
        return out
    }

    readonly property var ribbonNames: {
        if (!view)
            return []
        const out = []
        for (let i = 0; i < count; ++i)
            out.push(titleOf(view.get(i)))
        return out
    }

    readonly property int ribbonActive: {
        if (!view)
            return -1
        for (let i = 0; i < count; ++i) {
            const entry = view.get(i)
            if (entry && entry.applied)
                return i
        }
        return -1
    }

    Connections {
        target: field
        function onCurrentIndexChanged() {
            if (!deck.dragging)
                deck.pos = field.currentIndex
            deck.requestPalettes()
        }
    }

    Connections {
        target: view
        function onCountChanged() { deck.requestPalettes() }
        function onGenerationChanged() { deck.requestPalettes() }
    }

    onActiveChanged: {
        if (active) {
            deck.deal = 0
            Qt.callLater(() => deck.deal = 1)
            deck.requestPalettes()
        }
    }

    Component.onCompleted: {
        if (active) {
            deck.deal = 1
            deck.requestPalettes()
        }
    }

    Repeater {
        id: hand
        model: deck.slots

        Item {
            id: slot
            required property int index

            readonly property int idx: index + deck.slots * Math.round((deck.pos - index) / deck.slots)
            readonly property real d: idx - deck.pos
            readonly property real ad: Math.abs(d)
            readonly property bool live: idx >= 0 && idx < deck.count && ad < deck.side + 1
            readonly property var entry: live ? deck.view.get(idx) : null
            readonly property real near: Math.max(0, 1 - ad)
            readonly property real dealt: {
                const t = Math.max(0, Math.min(1, deck.deal * (1 + 0.09 * deck.side) - 0.09 * ad))
                return 1 - Math.pow(1 - t, 3)
            }
            readonly property real theta: (d < 0 ? -1 : 1)
                * (Math.min(ad, 1) * deck.step1 + Math.max(0, ad - 1) * deck.stepN)
            property real hoverLift: card.hovered && near < 0.5 ? 20 * deck.u : 0
            readonly property real radius: deck.handR + deck.lift * near + hoverLift
            readonly property real lower: (1 - dealt) * (deck.cardH * 1.35 + 140 * deck.u)

            width: deck.cardW
            height: deck.cardH
            x: deck.pivotX + radius * Math.sin(theta) - width / 2
            y: deck.pivotY - radius * Math.cos(theta) - height / 2 + lower
            rotation: (theta + (1 - dealt) * (d < 0 ? -0.1 : 0.1)) * 180 / Math.PI
            scale: 1 + (deck.frontScale - 1) * near
            z: 100 - ad
            opacity: Math.max(0, Math.min(1, deck.side + 1 - ad))
            visible: slot.live && deck.active && opacity > 0.01 && y < deck.height

            Behavior on hoverLift {
                NumberAnimation { duration: 170; easing.type: Easing.OutCubic }
            }

            DeckCard {
                id: card
                anchors.fill: parent
                u: deck.u
                entry: slot.entry
                picture: deck.pictureOf(slot.entry)
                front: slot.near
                dim: Math.min(0.3, Math.max(0, slot.ad - 0.6) * 0.065)
                scheme: deck.schemeOf(slot.entry)
                showSwatches: deck.showSwatches
                showShadow: deck.showShadows
                current: !!slot.entry && !!slot.entry.applied
                favourite: !!slot.entry && !!slot.entry.favourite
                saved: !!slot.entry && ((Number(slot.entry.badges) & 16) !== 0)
                title: deck.titleOf(slot.entry)
                meta: deck.metaOf(slot.entry)

                onClicked: deck.choose(slot.idx)
                onDoubleClicked: deck.apply(slot.idx)
                onWheeled: (angleY, pixelY) => deck.applyWheel(angleY, pixelY)
                onDragged: (dx, dy, released) => deck.applyDrag(dx, released)
            }
        }
    }

    DeckActions {
        id: actions
        row: deck.frontIndex
        entry: deck.frontIndex >= 0 && deck.view ? deck.view.get(deck.frontIndex) : null
        u: deck.u
        reveal: deck.deal
        dress: DeckPalettes.dressOf(deck.schemeOf(entry))
        favourite: !!entry && !!entry.favourite
        active: !!entry && !!entry.applied
        x: (deck.width - width) / 2
        y: deck.frontTop - height - 20 * deck.u
        z: 130

        onFavouriteClicked: {
            if (keymap && keymap.state && deck.frontIndex >= 0)
                keymap.state.toggleFavourite(deck.frontIndex)
        }
        onApplyClicked: deck.apply(deck.frontIndex)
    }

    DeckRibbon {
        x: 40 * deck.u
        y: Math.max(90 * deck.u, (field ? field.barReserve.height : 0) + 12 * deck.u)
        width: deck.width - 96 * deck.u
        u: deck.u
        pos: deck.pos
        span: deck.side
        accent: Theme.primary
        ink: "white"
        colors: deck.ribbonColors
        favourites: deck.ribbonFavourites
        thumbs: deck.ribbonThumbs
        names: deck.ribbonNames
        activeIndex: deck.ribbonActive
        z: 120

        onScrub: value => deck.pos = value
        onSettle: index => {
            if (deck.field)
                deck.field.currentIndex = deck.clamp(index, 0, deck.count - 1)
        }
    }
}
