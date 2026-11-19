.pragma library

// the bubble and the card both read their audio from here so they never disagree:
// "on", "off", or "lost" for the half of a desktop+mic mix that fell back
function sources(phase, activeMode, fallback, wantSystem, wantMic) {
    if (phase === "arming" || phase === "starting")
        return { system: wantSystem ? "on" : "off", mic: wantMic ? "on" : "off" };
    var system = activeMode === "system" || activeMode === "both";
    var mic = activeMode === "microphone" || activeMode === "both";
    return {
        system: system ? "on" : fallback ? "lost" : "off",
        mic: mic ? "on" : fallback ? "lost" : "off"
    };
}
