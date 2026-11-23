import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const L = require("./list.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

const sections = [
    { id: "a", title: "First", rows: [{ id: "a1" }, { id: "a2", size: "two" }] },
    { id: "empty", title: "Empty", rows: [] },
    { id: "b", title: "", rows: [{ id: "b1", size: "calc" }] },
    { id: "c", title: "Third", rows: [{ id: "c1" }, { id: "c2", disabled: true }, { id: "c3" }] }
];
const plain = L.build(sections, { scale: 1 });
equal(plain.heads.map(h => [h.section.id, h.y, h.h]), [["a", 6, 30], ["c", 196, 30]], "only titled sections with rows get a head");
equal(plain.rows.map(r => [r.row.id, r.y, r.h, r.section]), [["a1", 36, 40, 0], ["a2", 76, 52, 0], ["b1", 136, 60, 2], ["c1", 226, 40, 3], ["c2", 266, 40, 3], ["c3", 306, 40, 3]], "rows stack by their size, an untitled section after a gap");
equal(plain.height, 354, "the height closes with the bottom pad");
equal(plain.boxes, [], "no boxes unless asked");

equal(plain.rows.map(r => r.last), [false, true, true, false, false, true], "the last row of each section is marked");
const scaled = L.build(sections, { scale: 1.5 });
equal([scaled.rows[0].y, scaled.rows[0].h, scaled.height], [54, 60, 531], "the scale applies to every measure");

const boxed = L.build(sections.slice(0, 1), { scale: 1, boxed: true });
equal(boxed.boxes, [{ y: 36, h: 100 }], "a boxed section wraps its rows with padding");
equal(boxed.rows.map(r => r.y), [40, 80], "boxed rows sit inside the padding");
equal(L.build([], {}), { heads: [], rows: [], boxes: [], height: 0 }, "no sections, no height");
equal(L.build([{ id: "x", title: "X", rows: [] }], {}).height, 0, "only empty sections, no height");

const rows = plain.rows;
equal([L.indexOf(rows, "b1"), L.indexOf(rows, "nope")], [2, -1], "rows are found by id");
equal([L.move(rows, 0, 1), L.move(rows, 3, 1), L.move(rows, 5, 1), L.move(rows, 0, -1)], [1, 5, 5, 0], "moving skips a disabled row and stops at the ends");
equal([L.move(rows, -1, 1), L.move(rows, -1, -1)], [0, 5], "with nothing chosen, down takes the first and up the last");
equal(L.move([], 0, 1), -1, "an empty list has no selection");
equal(L.first([{ row: { disabled: true } }, { row: {} }]), 1, "the first choosable row");
const slots = [{ row: { id: "x" } }, { row: { id: "y", disabled: true } }, { row: { id: "z" } }];
equal(L.near(slots, 0), 0, "the row that moved into a slot takes it");
equal(L.near(slots, 1), 2, "a slot that cannot be chosen passes to the next row");
equal(L.near(slots, 5), 2, "a slot past the end falls back to the last row");
equal(L.near([{ row: { id: "x" } }, { row: { id: "y", disabled: true } }], 1), 0, "with nothing after it, the row before");
equal(L.near([], 0), -1, "an empty list has no slot");

equal([L.jump(rows, 0, 1), L.jump(rows, 2, 1), L.jump(rows, 5, 1)], [2, 3, 5], "a section jump lands on the next section's first row");
equal([L.jump(rows, 5, -1), L.jump(rows, 3, -1), L.jump(rows, 1, -1), L.jump(rows, 0, -1)], [3, 2, 0, 0], "back goes to this section's start, then the one before");

equal([L.page(rows, 0, 1, 150), L.page(rows, 5, -1, 150), L.page(rows, 0, -1, 150)], [2, 3, 0], "a page moves about a viewport");
equal([L.hit(rows, 36), L.hit(rows, 75.9), L.hit(rows, 76), L.hit(rows, 10), L.hit(rows, 200), L.hit(rows, 999)], [0, 0, 1, -1, -1, -1], "a point finds its row, heads and gaps find none");

equal(L.reveal(rows, 5, 0, 200, plain.height, 30), 146, "a row below scrolls just into view");
equal(L.reveal(rows, 3, 250, 200, plain.height, 30), 154, "a section's first row keeps its head in sight (clamped to the end)");
equal(L.reveal(rows, 0, 80, 200, plain.height, 30), 6, "the first row shows its head");
equal(L.reveal(rows, 1, 40, 200, plain.height, 30), 40, "a visible row moves nothing");
