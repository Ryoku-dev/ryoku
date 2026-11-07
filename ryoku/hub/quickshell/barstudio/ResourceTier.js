// The catalogue measures every subshell and bar style against Orpheus and
// publishes metadata.resourceTier (light, standard or heavy; see docs/store.md).
// Bar Studio marks the costlier two in each tile's corner and leaves light and
// unmeasured items bare, so the mark always means "uses more than the rest".

function tierOf(value) {
    const tier = String(value || "");
    return tier === "light" || tier === "standard" || tier === "heavy" ? tier : "";
}

function markOf(tier) {
    if (tier === "heavy")
        return "▲";
    if (tier === "standard")
        return "△";
    return "";
}

if (typeof module !== "undefined" && module.exports) module.exports = { tierOf, markOf };
