// Deduplicate the compositor's screen list to one surface set per physical
// output. Two situations make Quickshell.screens misrepresent the real outputs:
// QtWayland briefly exposes a nameless 0x0 placeholder before any output exists,
// and a monitor re-announce (a modeset -- e.g. a GPU-accelerated app such as
// EasyEffects initialising its render context on first launch) can leave two live
// ShellScreen objects for the same physical output at once. Every per-monitor
// surface (the bar, notification popups, the OSDs, the per-screen state slices) is
// fanned across this list, so an un-deduped duplicate maps two of each -- the
// visible symptom is two stacked bars, the rest is the desktop "tweaking out".
//
// Keep the first valid occurrence of each output name: one physical output always
// maps to exactly one surface, and holding the first object avoids rebuilding
// every per-monitor surface for a duplicate that the compositor is about to drop.
function uniqueByName(screens) {
    var out = [];
    var seen = [];
    var count = screens ? screens.length : 0;
    for (var i = 0; i < count; i++) {
        var s = screens[i];
        if (!s || s.name === "" || !(s.width > 0) || !(s.height > 0))
            continue;
        if (seen.indexOf(s.name) !== -1)
            continue;
        seen.push(s.name);
        out.push(s);
    }
    return out;
}

// Whether two sanitized screen lists hold exactly the same live objects. A
// monitor power-off/on makes QtWayland signal the screen list several times
// while the real outputs have not changed (a nameless placeholder is added,
// then removed); rebuilding every per-monitor surface for that churn is what
// crashed the shell (#312). A recreated output comes back as a NEW object with
// the same name, so identity, not just the name, must be compared: the same
// name with a different object is a genuine change and must flow through.
function sameOutputs(a, b) {
    if (!a || !b || a.length !== b.length)
        return false;
    for (var i = 0; i < a.length; i++)
        if (a[i] !== b[i])
            return false;
    return true;
}

// An output's stable identity is its NAME, never its ShellScreen object. A
// disabled and re-enabled output -- a laptop lid, a modeset -- comes back as a
// new object with the same name, so a per-monitor slice found by object identity
// resolves to nothing exactly when a surface is being rebuilt for it, and every
// binding reading that slice falls back to its stale value.
function sliceForName(instances, name) {
    if (!instances || !name)
        return null;
    for (var i = 0; i < instances.length; i++) {
        var inst = instances[i];
        if (inst && inst.modelData && inst.modelData.name === name)
            return inst;
    }
    return null;
}

function sliceForScreen(instances, screen) {
    return screen ? sliceForName(instances, screen.name) : null;
}

// A surface's monitor-derived size factor: the output's LOGICAL pixel height
// over 1080, capped. Quickshell reports the logical rectangle and the
// compositor scales the surface back up by the output scale, so the logical
// height alone already lands the chrome at the panel's physical height: a
// 1600 px panel at scale 1.25 arrives as height 1280 and the factor
// 1280/1080 renders at 1600/1080 physical. Multiplying the scale in as well
// (the old formula) scaled the chrome twice.
// `cap` is optional: without it the factor is uncapped.
function monitorScale(screen, cap) {
    if (!screen || !(screen.height > 0))
        return 1;
    var f = screen.height / 1080;
    return (typeof cap === "number" && cap > 0) ? Math.min(cap, f) : f;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { uniqueByName, sameOutputs, sliceForName, sliceForScreen, monitorScale };
