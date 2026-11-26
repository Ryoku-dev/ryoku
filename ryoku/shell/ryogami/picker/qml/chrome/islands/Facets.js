.pragma library

// The library's facets as the islands name them

function hueValues() {
    return [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 99]
}

function hueColor(bucket) {
    if (bucket === 99) return Qt.hsla(0, 0, 0.55, 1)
    return Qt.hsla(bucket / 12.0, 0.6, 0.55, 1)
}

function hueName(bucket, tr) {
    var names = ["Red", "Orange", "Yellow", "Lime", "Green", "Mint",
                 "Cyan", "Sky", "Blue", "Violet", "Purple", "Pink"]
    if (bucket === 99) return tr("Grey")
    return bucket >= 0 && bucket < names.length ? tr(names[bucket]) : ""
}

function types(tr) {
    return [
        { value: "", glyph: "apps", label: tr("Everything"), key: "filterBar.show.type.all" },
        { value: "static", glyph: "image", label: tr("Stills"), key: "filterBar.show.type.static" },
        { value: "video", glyph: "movie", label: tr("Video"), key: "filterBar.show.type.video" },
        { value: "we", glyph: "view_in_ar", label: tr("Scenes"), key: "filterBar.show.type.we" }
    ]
}

function typeName(value, tr) {
    var t = types(tr)
    for (var i = 0; i < t.length; ++i)
        if (t[i].value === value) return t[i].label
    return value
}

function orders(tr) {
    return [
        { value: "color", glyph: "palette", label: tr("By colour"), key: "filterBar.show.sort.color" },
        { value: "date", glyph: "schedule", label: tr("Newest"), key: "filterBar.show.sort.date" },
        { value: "recent", glyph: "history", label: tr("Recently applied"), key: "filterBar.show.sort.recent" },
        { value: "pop", glyph: "colorize", label: tr("Colour pop"), key: "filterBar.show.sort.pop" },
        { value: "richness", glyph: "gradient", label: tr("Colourful"), key: "filterBar.show.sort.richness" },
        { value: "minimalist", glyph: "crop_square", label: tr("Minimalist"), key: "filterBar.show.sort.minimalist" },
        { value: "res", glyph: "high_quality", label: tr("Highest resolution"), key: "filterBar.show.sort.res" }
    ]
}
