import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dir = path.dirname(fileURLToPath(import.meta.url));
const P = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(dir, "progression.js"), "utf8")
    .replace(/^(\.pragma|\.import) .*$/gm, ""), P);

test("a patient collector can fund the whole shop without circular rewards", () => {
    const independent = new Set([
        ...P.achievements.filter(a => a.category === "Ryoku").map(a => a.id),
        "welcome", "explorer", "identity", "portrait", "layout", "showcase",
        "card-complete", "card-tapestry",
        "profile-curator", "profile-sticker", "profile-self", "profile-mosaic",
        "profile-gallery", "profile-sanctuary", "profile-variety",
        "profile-curation", "profile-week",
        "collection-firstclaim", "collection-fifth", "collection-tenth",
        "collection-twentieth", "collection-thirty", "collection-forty",
        "secret-signal", "secret-sevenfold", "secret-keepsake",
        "secret-empty", "secret-twelve", "secret-timekeeper",
        "secret-many-faces", "secret-nightshift", "secret-rabbit"
    ]);
    const earned = Object.fromEntries([...independent].map(id => [id, true]));
    let state = P.clean({ version: 4, earned, claimed: earned });
    const seededBalance = state.balance;
    const shop = [...P.cosmetics].sort((a, b) => a.price - b.price);
    function collect() {
        for (let again = true; again;) {
            again = false;
            for (const badge of P.achievements) {
                if (state.earned[badge.id] && !state.claimed[badge.id]) {
                    const claimed = P.claim(state, badge.id);
                    state = claimed.state;
                    again ||= claimed.changed;
                }
            }
        }
    }
    collect();
    let count = 0;
    for (const item of shop) {
        const result = P.purchase(state, item.id);
        assert.equal(result.changed, true,
            item.id + " needs " + item.price + " credits, available " + state.balance +
            ", purchases completed " + count);
        state = result.state;
        count++;
        if (item.type === "palette") state = P.equip(state, item.id).state;
        if (item.type === "font") {
            for (const role of ["name", "hostname", "tagline"])
                state = P.font(state, role, item.id).state;
        }
        if (item.type === "background") state = P.background(state, item.id).state;
        if (item.type === "sticker") state = P.addSticker(state, item.id).state;
        collect();
    }
    assert.equal(count, P.cosmetics.length);
    assert.equal(Object.keys(state.owned).length, shop.length);
    assert.equal(state.earned["collection-everything"], true);
    assert.ok(seededBalance > 0);
    assert.ok(state.balance >= 0);
});
