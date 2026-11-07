// Pure weather display maps: WMO weather codes to the shell's glyph names,
// Material symbols, Nerd Font glyphs and short labels. The daemon (weather.go)
// owns the fetch, units and formatting; this file only names a code, so node can
// test it without QML.

// WMO weather code -> one of the shell's existing weather glyph names
// (sun | cloud | fog | rain | snow | storm), matching what GlyphIcon ships.
function glyphFor(code) {
    if (code === 0) return "sun";
    if (code <= 3) return "cloud";
    if (code === 45 || code === 48) return "fog";
    if (code >= 95) return "storm";
    if ((code >= 71 && code <= 77) || code === 85 || code === 86) return "snow";
    if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82)) return "rain";
    return "cloud";
}

// WMO weather code -> a Material Symbols weather glyph name (the set the shell's
// MaterialIcon renders). Day/night split for clear and partly cloudy; an unknown
// code falls back to a neutral cloud, never the brand mark.
function symbolFor(code, isDay) {
    var day = isDay === undefined ? true : !!isDay;
    if (code === 0) return day ? "clear_day" : "clear_night";
    if (code === 1 || code === 2) return day ? "partly_cloudy_day" : "partly_cloudy_night";
    if (code === 3) return "cloud";
    if (code === 45 || code === 48) return "foggy";
    if (code >= 95) return "thunderstorm";
    if ((code >= 71 && code <= 77) || code === 85 || code === 86) return "weather_snowy";
    if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82)) return "rainy";
    return "cloud";
}

// WMO weather code -> a Nerd Font weather glyph (the mono set a bar widget draws
// inline with its clock, where a Material icon would not share the text run).
// Day/night split where the font has both faces; an unknown code falls back to
// the neutral cloud.
function nerdFor(code, isDay) {
    var day = isDay === undefined ? true : !!isDay;
    if (code === 0) return day ? "\ue30d" : "\ue32b";
    if (code === 1 || code === 2) return day ? "\ue302" : "\ue32e";
    if (code === 3) return "\ue33d";
    if (code === 45 || code === 48) return "\ue313";
    if (code >= 95 && code <= 99) return "\ue31d";
    if (code === 85 || code === 86) return "\ue31a";
    if (code >= 71 && code <= 77) return "\ue3ad";
    if (code >= 80 && code <= 82) return day ? "\ue308" : "\ue333";
    if (code === 66 || code === 67) return "\ue318";
    if (code >= 61 && code <= 65) return day ? "\ue30a" : "\ue327";
    if (code >= 51 && code <= 57) return day ? "\ue308" : "\ue333";
    return "\ue33d";
}

// Short condition word for a WMO code (the display text, replacing wttr.in's %C).
function labelFor(code) {
    if (code === 0) return "Clear";
    if (code <= 3) return "Cloudy";
    if (code === 45 || code === 48) return "Fog";
    if (code >= 95) return "Thunder";
    if ((code >= 71 && code <= 77) || code === 85 || code === 86) return "Snow";
    if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82)) return "Rain";
    return "Cloudy";
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { glyphFor, labelFor, symbolFor, nerdFor };
}
