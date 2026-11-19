.pragma library

// The Material Symbols name for an OsdSource icon (a freedesktop symbolic
// name). The round looks draw the font; Adwaita keeps the symbolic SVGs.
function symbol(name) {
    switch (name) {
    case "audio-volume-muted": return "volume_off";
    case "audio-volume-high": return "volume_up";
    case "audio-volume-medium": return "volume_down";
    case "audio-volume-low": return "volume_mute";
    case "microphone-sensitivity-muted": return "mic_off";
    case "brightness-high": return "brightness_high";
    case "brightness-medium": return "brightness_medium";
    case "brightness-low": return "brightness_low";
    case "input-keyboard": return "keyboard";
    }
    return String(name).indexOf("microphone") === 0 ? "mic" : "volume_up";
}
