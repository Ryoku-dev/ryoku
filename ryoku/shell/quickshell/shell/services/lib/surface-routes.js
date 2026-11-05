function styleOwns(surface, barStyle) {
    return surface === "cheatsheet" && barStyle === "iris";
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { styleOwns };
