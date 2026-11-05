function chooseMusicSink(playbackSink, defaultSink, previousSink, previousDefaultSink, previousKnown) {
    var playback = playbackSink || "";
    var currentDefault = defaultSink || "";
    var previous = previousSink || "";
    var previousDefault = previousDefaultSink || "";

    if (playback) {
        var defaultChanged = currentDefault && previousDefault && currentDefault !== previousDefault;
        var staleDirectLink = playback === previous && previous === previousDefault;
        return defaultChanged && staleDirectLink ? currentDefault : playback;
    }
    if (previousKnown && previous && previous !== previousDefault)
        return previous;
    if (currentDefault)
        return currentDefault;
    return previousKnown ? previous : "";
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { chooseMusicSink };
