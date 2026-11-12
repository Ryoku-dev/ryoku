.pragma library
.import "../../schema/app-rules.js" as Schema

// app rules as one list: each app's window-rule records and its look override
// read as one rule and are written back to the two stores; tested by rules.test.mjs

var props = Schema.rows.filter(function (r) { return r.kind !== undefined; });
var layerRows = Schema.rows.filter(function (r) { return r.layer === true; });

var lookDefaults = { opacity: -1, rounding: -1, borderSize: -1, blur: "inherit", shadow: "inherit",
                     dim: "inherit", anim: "inherit", opaque: "inherit" };

function clone(v) {
    return v === undefined ? undefined : JSON.parse(JSON.stringify(v));
}

function keyOf(cls, title) {
    return JSON.stringify([String(cls || ""), String(title || "")]);
}

function has(list, id) {
    return (list || []).indexOf(id) >= 0;
}

function isSet(v) {
    return v !== undefined && v !== null && v !== "" && v !== false;
}

// what the window manager applies: its window-rule actions and the look
// override fields it writes (Settings.windowRuleActions, Settings.appOverrideFields);
// no look fields means not known yet, so every look is offered
function vocabulary(actions, looks) {
    return { actions: actions || [], looks: looks || [] };
}

function writesLook(wm, field) {
    return wm.looks.length === 0 || has(wm.looks, field);
}

// the options of a mode or tri property this window manager can apply
function optionsOf(p, wm) {
    return (p.options || []).filter(function (o) {
        if (p.kind === "mode")
            return has(wm.actions, o.action);
        return !o.needsAction || has(wm.actions, o.action);
    });
}

// offered here: actions the window manager lists and look fields it writes
function available(p, wm) {
    switch (p.kind) {
    case "amount":
        return writesLook(wm, p.field);
    case "tri":
        return writesLook(wm, p.field) && optionsOf(p, wm).length > 0;
    case "mode":
        return optionsOf(p, wm).length > 0;
    case "flag":
        return has(wm.actions, p.action) || (!!p.field && writesLook(wm, p.field));
    }
    return has(wm.actions, p.action);
}

function offered(wm) {
    return props.filter(function (p) { return available(p, wm); });
}

// actions an offered property reads, so stored records of them are not set aside
function claimed(wm) {
    var out = {};
    offered(wm).forEach(function (p) {
        [p.action, p.zeroAction, p.valueAction].forEach(function (a) {
            if (a)
                out[a] = true;
        });
        var opts = p.kind === "mode" ? optionsOf(p, wm) : p.kind === "tri" ? (p.options || []) : [];
        opts.forEach(function (o) { out[o.action] = true; });
    });
    return out;
}

// [{ key, cls, title, rules, override }] in store order; rules holds window-rule
// indices and, as -1 - i, any further look override for the same app
function grouped(windowRules, appOverrides) {
    var out = [], at = {};
    function slot(rec) {
        var k = keyOf(rec["class"], rec.title);
        if (at[k] === undefined) {
            at[k] = out.length;
            out.push({ key: k, cls: String(rec["class"] || ""), title: String(rec.title || ""), rules: [], override: -1 });
        }
        return out[at[k]];
    }
    (windowRules || []).forEach(function (r, i) { slot(r || {}).rules.push(i); });
    (appOverrides || []).forEach(function (a, i) {
        var g = slot(a || {});
        if (g.override < 0)
            g.override = i;
        else
            g.rules.push(-1 - i);
    });
    return out;
}

// one group as an editable rule: { cls, title, values, extras, keep }
function itemOf(group, windowRules, appOverrides, wm) {
    var ao = group.override >= 0 ? (appOverrides[group.override] || {}) : {};
    var recs = group.rules.filter(function (i) { return i >= 0; }).map(function (i) { return windowRules[i] || {}; });
    var mine = claimed(wm);
    var values = {}, extras = [];
    function find(action) {
        for (var i = 0; i < recs.length; i++)
            if (recs[i].action === action)
                return recs[i];
        return null;
    }
    offered(wm).forEach(function (p) {
        switch (p.kind) {
        case "flag":
            if ((p.action && find(p.action)) || (p.field && ao[p.field] === p.fieldOn))
                values[p.id] = true;
            break;
        case "mode":
            optionsOf(p, wm).forEach(function (o) {
                if (values[p.id] === undefined && find(o.action))
                    values[p.id] = o.value;
            });
            break;
        case "pick":
            var pr = find(p.action);
            if (pr)
                values[p.id] = p.options.some(function (o) { return o.value === pr.value; }) ? pr.value : p.options[0].value;
            break;
        case "text":
            var tr = find(p.action);
            if (tr)
                values[p.id] = String(tr.value || "");
            break;
        case "amount":
            var n = Number(ao[p.field]);
            if (ao[p.field] !== undefined && ao[p.field] !== null && n >= 0)
                values[p.id] = n;
            else if (p.valueAction && find(p.valueAction))
                values[p.id] = Math.max(p.min, Math.min(p.max, parseFloat(find(p.valueAction).value) || p.max));
            else if (p.zeroAction && find(p.zeroAction))
                values[p.id] = 0;
            break;
        case "tri":
            if (ao[p.field] === "off" || ao[p.field] === "on")
                values[p.id] = ao[p.field];
            else
                (p.options || []).forEach(function (o) {
                    if (values[p.id] === undefined && find(o.action))
                        values[p.id] = o.value;
                });
            break;
        }
    });
    recs.forEach(function (r) {
        if (!mine[r.action])
            extras.push(clone(r));
    });
    // look fields this window manager does not write stay stored for the one that does
    var read = {};
    offered(wm).forEach(function (p) {
        if (p.field)
            read[p.field] = true;
    });
    var keep = {};
    for (var k in ao)
        if (k !== "class" && k !== "title" && !read[k]
                && (lookDefaults[k] === undefined || JSON.stringify(ao[k]) !== JSON.stringify(lookDefaults[k])))
            keep[k] = clone(ao[k]);
    return { cls: group.cls, title: group.title, values: values, extras: extras, keep: keep };
}

// the stored records of one rule: { rules, override (or null) }; a look
// switch the window manager offers as an action is written as that action
function recordsOf(item, wm) {
    var cls = String(item.cls || ""), title = String(item.title || "");
    var rules = [], look = clone(lookDefaults), touched = false;
    function rule(action, value) {
        rules.push({ "class": cls, "title": title, "action": action, "value": value === undefined ? "" : String(value) });
    }
    offered(wm).forEach(function (p) {
        var v = (item.values || {})[p.id];
        if (!isSet(v))
            return;
        switch (p.kind) {
        case "flag":
            if (p.action && has(wm.actions, p.action)) {
                rule(p.action);
            } else if (p.field) {
                look[p.field] = p.fieldOn;
                touched = true;
            }
            break;
        case "mode":
            optionsOf(p, wm).forEach(function (o) {
                if (o.value === v)
                    rule(o.action);
            });
            break;
        case "pick":
        case "text":
            rule(p.action, String(v).trim());
            break;
        case "amount":
            look[p.field] = p.unit === "px" ? Math.round(Number(v)) : Math.round(Number(v) * 100) / 100;
            touched = true;
            break;
        case "tri":
            look[p.field] = v;
            touched = true;
            break;
        }
    });
    (item.extras || []).forEach(function (r) {
        rules.push(Object.assign(clone(r), { "class": cls, "title": title }));
    });
    var override = null;
    if (touched || Object.keys(item.keep || {}).length > 0)
        override = Object.assign({ "class": cls, "title": title }, look, clone(item.keep || {}));
    return { rules: rules, override: override };
}

function dropsOf(group) {
    var rules = {}, looks = {};
    group.rules.forEach(function (i) {
        if (i >= 0)
            rules[i] = true;
        else
            looks[-1 - i] = true;
    });
    return { rules: rules, looks: looks };
}

// the stores after adding (index -1) or replacing one rule; its records take the
// place of the old ones, so the order of everything else holds
function withRule(windowRules, appOverrides, index, item, wm) {
    var old = index >= 0 ? grouped(windowRules, appOverrides)[index] : null;
    var made = recordsOf(item, wm);
    var drop = old ? dropsOf(old) : { rules: {}, looks: {} };
    var firsts = old ? old.rules.filter(function (i) { return i >= 0; }) : [];
    var ruleAt = firsts.length > 0 ? Math.min.apply(null, firsts) : -1;
    var lookAt = old ? old.override : -1;
    var wr = [], placed = false;
    (windowRules || []).forEach(function (r, i) {
        if (i === ruleAt) {
            wr = wr.concat(made.rules);
            placed = true;
        }
        if (!drop.rules[i])
            wr.push(clone(r));
    });
    if (!placed)
        wr = wr.concat(made.rules);
    var ao = [];
    placed = false;
    (appOverrides || []).forEach(function (a, i) {
        if (i === lookAt) {
            if (made.override)
                ao.push(made.override);
            placed = true;
        } else if (!drop.looks[i]) {
            ao.push(clone(a));
        }
    });
    if (!placed && made.override)
        ao.push(made.override);
    return { windowRules: wr, appOverrides: ao };
}

function withoutRule(windowRules, appOverrides, index) {
    var g = grouped(windowRules, appOverrides)[index];
    var drop = g ? dropsOf(g) : { rules: {}, looks: {} };
    if (g && g.override >= 0)
        drop.looks[g.override] = true;
    return {
        windowRules: (windowRules || []).filter(function (r, i) { return !drop.rules[i]; }).map(clone),
        appOverrides: (appOverrides || []).filter(function (a, i) { return !drop.looks[i]; }).map(clone)
    };
}

// what is wrong with a typed value: "" or the property's check
function problem(p, v) {
    var s = String(v === undefined || v === null ? "" : v).trim();
    if (s === "")
        return "";
    switch (p.check) {
    case "size":
        return /^\d+\s*[x×]\s*\d+$/i.test(s) ? "" : "size";
    case "point":
        return /^-?\d+\s*,\s*-?\d+$/.test(s) ? "" : "point";
    case "fraction":
        var f = Number(s);
        return !isNaN(f) && f >= 0.1 && f <= 1 ? "" : "fraction";
    case "positive":
        var g = Number(s);
        return !isNaN(g) && g > 0 && g <= 10 ? "" : "positive";
    }
    return "";
}

// the words for an action this page does not offer here
function actionWords(action) {
    for (var i = 0; i < props.length; i++) {
        var p = props[i];
        var opts = p.options || [];
        for (var j = 0; j < opts.length; j++)
            if (opts[j].action === action)
                return opts[j].title;
        if (p.action === action || p.zeroAction === action || p.valueAction === action)
            return p.label;
    }
    return String(action || "");
}

// the rule's short line, one phrase per change; tr translates source strings
// and unused words an action this window manager does not apply
function phrases(item, wm, tr, unused) {
    var out = [];
    offered(wm).forEach(function (p) {
        var v = (item.values || {})[p.id];
        if (!isSet(v))
            return;
        switch (p.kind) {
        case "flag":
            out.push(tr(p.title));
            break;
        case "text":
            out.push(tr(p.title).replace("%1", String(v).trim()));
            break;
        case "amount":
            out.push(tr(p.title).replace("%1", p.pct ? Math.round(Number(v) * 100) + "%" : Math.round(Number(v))));
            break;
        default:
            (p.options || []).forEach(function (o) {
                if (o.value === v)
                    out.push(tr(o.title));
            });
        }
    });
    (item.extras || []).forEach(function (r) {
        out.push(unused.replace("%1", tr(actionWords(r.action))));
    });
    return out;
}

// phrases as one line, lowercase after the first unless a short form like "PiP" opens one
function joined(list) {
    return (list || []).map(function (s, i) {
        s = String(s);
        var rest = s.split(" ")[0].slice(1);
        if (i === 0 || rest !== rest.toLowerCase())
            return s;
        return s.charAt(0).toLowerCase() + s.slice(1);
    }).join(", ");
}

function anySet(item) {
    var v = item.values || {};
    for (var k in v)
        if (isSet(v[k]))
            return true;
    return (item.extras || []).length > 0;
}

// the rule holding the stored record an unhonored key names, or -1
function ruleOfStoreKey(list, key) {
    var m = /^desktop\.(windowRules|appOverrides)\[(\d+)\]/.exec(String(key || ""));
    if (!m)
        return -1;
    var at = parseInt(m[2], 10);
    for (var i = 0; i < list.length; i++) {
        var g = list[i];
        if (m[1] === "windowRules" ? g.rules.indexOf(at) >= 0 : (g.override === at || g.rules.indexOf(-1 - at) >= 0))
            return i;
    }
    return -1;
}

// shell layers: the provider's list row either names each field, or its records
// carry one action each, listed by its "[i].action" row
function layerRow(name) {
    for (var i = 0; i < layerRows.length; i++)
        if (layerRows[i].name === name)
            return layerRows[i];
    return null;
}

function layerSpec(rows) {
    var list = null;
    (rows || []).forEach(function (r) {
        if (!list && r && r.ctl === "list" && r.key && String(r.key).indexOf("[") < 0)
            list = r;
    });
    if (!list)
        return null;
    var spec = { key: String(list.key).replace(/^desktop\./, ""), shape: "", props: [] };
    function described(name, fallback) {
        var w = layerRow(name);
        return { id: name, label: w ? w.label : (fallback || name), detail: w ? w.detail : "", rowId: w ? w.id : "" };
    }
    if (list.fields && list.fields.length) {
        spec.shape = "fields";
        list.fields.forEach(function (f) {
            if (f.name === "namespace")
                return;
            var p = described(f.name, f.label);
            if (f.ctl === "sw") {
                p.kind = "flag";
            } else if (f.ctl === "seg" || f.ctl === "chips" || f.ctl === "pick") {
                var w = layerRow(f.name);
                p.kind = "tri";
                p.options = (f.opts || []).filter(function (o) { return o !== "inherit"; }).map(function (o) {
                    var known = w ? (w.options || []).filter(function (x) { return x.value === o; })[0] : null;
                    return { value: o, label: known ? known.label : o };
                });
            } else {
                var lo = f.lo === undefined ? 0 : f.lo, hi = f.hi === undefined ? 1 : f.hi;
                var whole = Math.floor(lo) === lo && Math.floor(hi) === hi && hi > 1;
                p.kind = "amount";
                p.min = Math.max(0, lo);
                p.max = hi;
                p.step = whole ? 1 : 0.01;
                p.pct = !whole;
                p.unit = whole ? "px" : "";
                p.start = whole ? Math.round((p.min + p.max) / 4) : 0.9;
                p.unset = f.def !== undefined ? f.def : lo < 0 ? lo : -1;
            }
            spec.props.push(p);
        });
        return spec;
    }
    var acts = null;
    (rows || []).forEach(function (r) {
        if (r && r.key === list.key + "[i].action")
            acts = r.opts || [];
    });
    if (!acts)
        return null;
    spec.shape = "actions";
    acts.forEach(function (a) {
        var p = described(a);
        p.action = a;
        p.kind = "flag";
        if (a === "ignorealpha") {
            p.kind = "amount";
            p.min = 0;
            p.max = 1;
            p.step = 0.01;
            p.pct = false;
            p.start = 0.5;
        }
        spec.props.push(p);
    });
    return spec;
}

// layer rules as items: { namespace, values, at (record indices), keep, extras }
function layerItems(spec, records) {
    var out = [];
    if (!spec)
        return out;
    if (spec.shape === "fields") {
        (records || []).forEach(function (r, i) {
            r = r || {};
            var values = {};
            spec.props.forEach(function (p) {
                var v = r[p.id];
                if (p.kind === "flag" && v === true)
                    values[p.id] = true;
                else if (p.kind === "tri" && typeof v === "string" && v !== "inherit" && v !== "")
                    values[p.id] = v;
                else if (p.kind === "amount" && typeof v === "number" && v >= 0)
                    values[p.id] = v;
            });
            out.push({ namespace: String(r.namespace || ""), values: values, at: [i], keep: clone(r), extras: [] });
        });
        return out;
    }
    var at = {};
    (records || []).forEach(function (r, i) {
        r = r || {};
        var ns = String(r.namespace || "");
        if (at[ns] === undefined) {
            at[ns] = out.length;
            out.push({ namespace: ns, values: {}, at: [], keep: {}, extras: [] });
        }
        var it = out[at[ns]];
        it.at.push(i);
        var p = spec.props.filter(function (q) { return q.action === r.action; })[0];
        if (!p)
            it.extras.push(clone(r));
        else if (p.kind === "amount")
            it.values[p.id] = Math.max(p.min, Math.min(p.max, isNaN(parseFloat(r.value)) ? p.start : parseFloat(r.value)));
        else
            it.values[p.id] = true;
    });
    return out;
}

function layerRecords(spec, item) {
    var ns = String(item.namespace || "").trim();
    var v = item.values || {};
    if (spec.shape === "fields") {
        var rec = Object.assign(clone(item.keep || {}), { namespace: ns });
        spec.props.forEach(function (p) {
            var x = v[p.id];
            if (p.kind === "flag")
                rec[p.id] = x === true;
            else if (p.kind === "tri")
                rec[p.id] = x ? x : "inherit";
            else
                rec[p.id] = x === undefined || x === null ? p.unset : (p.unit === "px" ? Math.round(x) : Math.round(x * 100) / 100);
        });
        return [rec];
    }
    var out = [];
    spec.props.forEach(function (p) {
        var x = v[p.id];
        if (!isSet(x))
            return;
        out.push({ namespace: ns, action: p.action, value: p.kind === "amount" ? String(Math.round(x * 100) / 100) : "" });
    });
    (item.extras || []).forEach(function (r) { out.push(Object.assign(clone(r), { namespace: ns })); });
    return out;
}

// the layer records after adding (index -1), replacing, or removing (item null) one
function withLayer(spec, records, index, item) {
    var old = index >= 0 ? layerItems(spec, records)[index] : null;
    var drop = {}, at = -1;
    if (old) {
        old.at.forEach(function (i) { drop[i] = true; });
        at = Math.min.apply(null, old.at);
    }
    var made = item ? layerRecords(spec, item) : [];
    var out = [], placed = false;
    (records || []).forEach(function (r, i) {
        if (i === at) {
            out = out.concat(made);
            placed = true;
        }
        if (!drop[i])
            out.push(clone(r));
    });
    if (!placed)
        out = out.concat(made);
    return out;
}

// the layer rule's short line; pair joins a label and its value
function layerPhrases(spec, item, tr, pair, unused) {
    var out = [];
    spec.props.forEach(function (p) {
        var x = (item.values || {})[p.id];
        if (!isSet(x))
            return;
        if (p.kind === "flag") {
            out.push(tr(p.label));
        } else if (p.kind === "tri") {
            var o = (p.options || []).filter(function (q) { return q.value === x; })[0];
            out.push(pair.replace("%1", tr(p.label)).replace("%2", tr(o ? o.label : x)));
        } else {
            var shown = p.pct ? Math.round(x * 100) + "%" : p.unit === "px" ? Math.round(x) + " px" : String(Math.round(x * 100) / 100);
            out.push(pair.replace("%1", tr(p.label)).replace("%2", shown));
        }
    });
    (item.extras || []).forEach(function (r) {
        out.push(unused.replace("%1", String(r.action || "")));
    });
    return out;
}

function layerAnySet(item) {
    var v = item.values || {};
    for (var k in v)
        if (isSet(v[k]))
            return true;
    return (item.extras || []).length > 0;
}

// a record with its keys in one order, so a record read back from disk equals the one written
function canonical(v) {
    if (Array.isArray(v))
        return v.map(canonical);
    if (v === null || typeof v !== "object")
        return v;
    var out = {};
    Object.keys(v).sort().forEach(function (k) { out[k] = canonical(v[k]); });
    return out;
}

// the rule store's value: one list of { windowRules: record } and { appOverrides: record },
// so an undo puts back only the records its change touched
function tagged(windowRules, appOverrides) {
    return (windowRules || []).map(function (r) { return { windowRules: canonical(r) }; })
        .concat((appOverrides || []).map(function (r) { return { appOverrides: canonical(r) }; }));
}

function untagged(list, name) {
    return (list || []).filter(function (x) { return x && x[name] !== undefined; })
        .map(function (x) { return clone(x[name]); });
}
