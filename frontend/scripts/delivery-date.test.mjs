import assert from "node:assert/strict";
import test from "node:test";

import {
  deliveryPreviewNow,
  estimatedDeliveryDate,
  formatDeliveryDate,
} from "../src/lib/deliveryDate.ts";

test("Monday orders target the same week's Thursday", () => {
  assert.equal(
    estimatedDeliveryDate(new Date("2026-08-24T23:59:59-04:00")),
    "2026-08-27",
  );
});

test("orders after Monday night target the Thursday after next", () => {
  assert.equal(
    estimatedDeliveryDate(new Date("2026-08-25T00:00:00-04:00")),
    "2026-09-03",
  );
  assert.equal(
    estimatedDeliveryDate(new Date("2026-08-26T12:00:00-04:00")),
    "2026-09-03",
  );
});

test("weekend orders target the next Thursday", () => {
  assert.equal(
    estimatedDeliveryDate(new Date("2026-08-30T12:00:00-04:00")),
    "2026-09-03",
  );
});

test("cutoff uses Eastern time rather than the browser time zone", () => {
  assert.equal(
    estimatedDeliveryDate(new Date("2026-08-25T03:30:00Z")),
    "2026-08-27",
  );
  assert.equal(formatDeliveryDate("2026-09-03"), "Thursday, September 3");
});

test("local preview override is explicit and development-only", () => {
  const fallback = new Date("2026-08-22T12:00:00-04:00");
  const enabled = deliveryPreviewNow(
    "?deliveryNow=2026-08-25T12%3A00%3A00-04%3A00",
    true,
    fallback,
  );
  assert.equal(enabled.now.toISOString(), "2026-08-25T16:00:00.000Z");
  assert.equal(enabled.overridden, true);

  const disabled = deliveryPreviewNow("?deliveryNow=2026-08-25", false, fallback);
  assert.equal(disabled.now, fallback);
  assert.equal(disabled.overridden, false);
});
