.pragma library

function position(i, n) {
    if (n <= 1) return "only";
    return i === 0 ? "first" : (i === n - 1 ? "last" : "middle");
}
