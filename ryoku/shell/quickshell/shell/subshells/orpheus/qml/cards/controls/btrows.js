// found devices keep their place and new ones go last, so a name arriving never moves a row
function order(current, entries) {
    var pos = {};
    for (var i = 0; i < current.length; i++)
        pos[current[i]] = i;
    return entries.slice().sort(function(a, b) {
        if (a.rank !== b.rank)
            return a.rank - b.rank;
        if (a.rank < 2) {
            var byName = String(a.label).localeCompare(String(b.label));
            return byName !== 0 ? byName : String(a.address).localeCompare(String(b.address));
        }
        var left = pos[a.address];
        var right = pos[b.address];
        if (left !== undefined && right !== undefined)
            return left - right;
        if (left !== undefined)
            return -1;
        if (right !== undefined)
            return 1;
        return String(a.address).localeCompare(String(b.address));
    }).map(function(entry) { return entry.address; });
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { order };
