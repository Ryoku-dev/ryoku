.pragma library

const catalog = [
    { id: "system", side: "left", tab: "controls", label: "Controls", glyph: "settings", source: "cards/SystemCard.qml" },
    { id: "notifications", side: "left", tab: "notices", label: "Notices", glyph: "notifications", source: "cards/NotificationsCard.qml" },
    { id: "weather", side: "left", tab: "weather", label: "Weather", glyph: "cloud", source: "cards/WeatherCard.qml" },
    { id: "media", side: "left", tab: "media", label: "Media", glyph: "play_circle", source: "cards/MediaCard.qml" },
    { id: "capture", side: "left", tab: "capture", label: "Capture", glyph: "photo_camera", source: "cards/CaptureCard.qml" },
    { id: "stage", side: "left", tab: "stage", label: "Stage", glyph: "graphic_eq", source: "cards/StageCard.qml" },
    { id: "usage", side: "right", tab: "overview", label: "Overview", glyph: "monitor_heart", source: "cards/UsageCard.qml" },
    { id: "tools", side: "right", tab: "tools", label: "Tools", glyph: "download", source: "cards/ToolsCard.qml" },
    { id: "chat", side: "right", tab: "chat", label: "Chat", glyph: "chat", source: "cards/ChatCard.qml" }
];

function entries(side) {
    return catalog.filter(function(entry) { return entry.side === side; });
}

function byId(id) {
    for (var i = 0; i < catalog.length; ++i) {
        if (catalog[i].id === id)
            return catalog[i];
    }
    return null;
}

function defaultTabs(side) {
    var result = [];
    var sideEntries = entries(side);
    for (var i = 0; i < sideEntries.length; ++i) {
        var entry = sideEntries[i];
        var group = null;
        for (var j = 0; j < result.length; ++j) {
            if (result[j].id === entry.tab) {
                group = result[j];
                break;
            }
        }
        if (!group) {
            group = { id: entry.tab, label: entry.label, glyph: entry.glyph, cards: [] };
            result.push(group);
        }
        group.cards.push(entry.id);
    }
    return result;
}
