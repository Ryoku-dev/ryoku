// The edits that turn one ordered list of keys into another: removals first,
// then moves and inserts in order. A row whose key stays is only ever moved,
// so a view keeps its delegate and can slide it instead of rebuilding it.

function editScript(from, to) {
    var current = Array.isArray(from) ? from.slice() : [];
    var target = Array.isArray(to) ? to : [];
    var ops = [];
    var wanted = {};
    for (var i = 0; i < target.length; i++)
        wanted["$" + target[i]] = (wanted["$" + target[i]] || 0) + 1;
    // keep as many of each key as the target holds, drop the rest from the end
    var kept = {};
    for (var j = 0; j < current.length; j++) {
        var key = "$" + current[j];
        kept[key] = (kept[key] || 0) + 1;
    }
    for (var k = current.length - 1; k >= 0; k--) {
        var name = "$" + current[k];
        if (kept[name] > (wanted[name] || 0)) {
            kept[name] -= 1;
            ops.push({ op: "remove", index: k });
            current.splice(k, 1);
        }
    }
    for (var n = 0; n < target.length; n++) {
        if (current[n] === target[n])
            continue;
        var at = current.indexOf(target[n], n + 1);
        if (at >= 0) {
            ops.push({ op: "move", from: at, to: n });
            current.splice(n, 0, current.splice(at, 1)[0]);
        } else {
            ops.push({ op: "insert", index: n, key: target[n] });
            current.splice(n, 0, target[n]);
        }
    }
    return ops;
}

// Applies an edit script to a plain array, the way the view's model takes it.
function apply(list, ops) {
    var out = Array.isArray(list) ? list.slice() : [];
    for (var i = 0; i < ops.length; i++) {
        var op = ops[i];
        if (op.op === "remove")
            out.splice(op.index, 1);
        else if (op.op === "move")
            out.splice(op.to, 0, out.splice(op.from, 1)[0]);
        else if (op.op === "insert")
            out.splice(op.index, 0, op.key);
    }
    return out;
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { editScript: editScript, apply: apply };
}
