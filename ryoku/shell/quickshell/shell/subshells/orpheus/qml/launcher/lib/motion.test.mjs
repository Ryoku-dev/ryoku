import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { ease, rippleProgress, rippleSpan } = require("./motion.js");

const decelerate = [0, 0, 0.2, 1, 1, 1];
const springFast = [0.42, 1.67, 0.21, 0.9, 1, 1];

test("a curve starts at rest and lands exactly", () => {
  assert.equal(ease(decelerate, 0), 0);
  assert.equal(ease(decelerate, 1), 1);
});

test("a decelerating curve covers most of the way early", () => {
  assert.ok(ease(decelerate, 0.3) > 0.5);
  assert.ok(ease(decelerate, 0.6) > ease(decelerate, 0.3));
});

test("Material's spring overshoots before it lands", () => {
  let peak = 0;
  for (let x = 0; x <= 1; x += 0.02) peak = Math.max(peak, ease(springFast, x));
  assert.ok(peak > 1);
});

test("rows ripple in one after another, never all at once", () => {
  const at = 40;
  const first = rippleProgress(at, 0, 14, 180, decelerate);
  const third = rippleProgress(at, 2, 14, 180, decelerate);
  const sixth = rippleProgress(at, 5, 14, 180, decelerate);
  assert.ok(first > third);
  assert.equal(sixth, 0);
});

test("every row has settled once the ripple's span has passed", () => {
  const span = rippleSpan(9, 14, 180);
  for (let i = 0; i < 9; i++) assert.equal(rippleProgress(span, i, 14, 180, decelerate), 1);
});

test("with motion off the rows simply appear", () => {
  assert.equal(rippleProgress(0, 0, 14, 0, decelerate), 1);
});
