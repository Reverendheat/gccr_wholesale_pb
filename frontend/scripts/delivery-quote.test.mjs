import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");

test("checkout requests authoritative delivery quote", () => {
  const api = read("src/lib/api.ts");
  const portal = read("src/pages/CustomerPortal.tsx");

  assert.match(api, /\/fulfillment\/quote/);
  assert.match(api, /\/fulfillment\/options/);
  assert.match(portal, /quoteFulfillment\(/);
  assert.match(portal, /Calculating driving distance/);
  assert.match(portal, /activeDeliveryQuote\.total_cents/);
});

test("delivery submission waits for valid quote", () => {
  const portal = read("src/pages/CustomerPortal.tsx");
  assert.match(portal, /fulfillment\.method === "delivery" && !activeDeliveryQuote/);
  assert.match(portal, /fulfillmentOptions\.max_miles/);
  assert.match(portal, /within.*driving miles/i);
});
