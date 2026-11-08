// light and unmeasured stay bare so a mark always means it uses more than the rest

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
