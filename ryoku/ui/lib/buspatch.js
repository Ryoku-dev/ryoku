// Applies a daemon delta frame to the cached settings object. Ops are
// {"p": [...keys], "v": value} (set) or {"p": [...keys], "x": true} (delete);
// arrays arrive whole at their path. Every subscriber in the process shares
// the cached object, so nothing is mutated: the objects along each patched
// path are copied once and untouched subtrees keep their identity, which is
// what lets a binding on an unrelated key stay asleep.

function shallow(v) {
    if (Array.isArray(v))
        return v.slice();
    var out = {};
    for (var k in v)
        out[k] = v[k];
    return out;
}

function isObject(v) {
    return v !== null && typeof v === "object" && !Array.isArray(v);
}

function apply(root, ops) {
    if (!ops || !ops.length)
        return root;
    var next = isObject(root) ? shallow(root) : {};
    // Objects already copied by this patch; a second op under the same parent
    // writes into the copy instead of copying again.
    var fresh = [next];
    function own(parent, key) {
        var child = parent[key];
        if (fresh.indexOf(child) >= 0)
            return child;
        var copy = isObject(child) ? shallow(child) : {};
        parent[key] = copy;
        fresh.push(copy);
        return copy;
    }
    for (var i = 0; i < ops.length; i++) {
        var op = ops[i];
        var p = op && Array.isArray(op.p) ? op.p : null;
        if (!p)
            continue;
        if (p.length === 0) {
            if (op.x === true)
                next = {};
            else
                next = isObject(op.v) ? shallow(op.v) : {};
            fresh = [next];
            continue;
        }
        var parent = next;
        var missing = false;
        for (var j = 0; j < p.length - 1; j++) {
            if (op.x === true && !isObject(parent[p[j]])) {
                missing = true;
                break;
            }
            parent = own(parent, p[j]);
        }
        if (missing)
            continue;
        var last = p[p.length - 1];
        if (op.x === true)
            delete parent[last];
        else
            parent[last] = op.v;
    }
    return next;
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { apply };
