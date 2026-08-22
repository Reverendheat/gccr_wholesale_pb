import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");

test("customer order updates use the protected PATCH endpoint", () => {
  const api = read("src/lib/api.ts");
  assert.match(api, /fetch\(`\$\{BASE\}\/orders\/\$\{id\}`,[\s\S]*?method: "PATCH"/);
});

test("only the creator sees edit controls for pending orders", () => {
  const portal = read("src/pages/CustomerPortal.tsx");
  assert.match(portal, /o\.customer === user\?\.id && o\.status === "pending"/);
  assert.match(portal, /startEditingOrder\(o\)/);
});

test("customers can cancel their own pending order through workflow endpoint", () => {
  const api = read("src/lib/api.ts");
  const portal = read("src/pages/CustomerPortal.tsx");
  assert.match(api, /function cancelOrder[\s\S]*?orders\/\$\{id\}\/events[\s\S]*?customer_cancel/);
  assert.match(portal, /o\.customer === user\?\.id && o\.status === "pending"[\s\S]*?Cancel order/);
  assert.match(portal, /window\.confirm[\s\S]*?await cancelOrder[\s\S]*?status: "cancelled"/);
});

test("editing preloads and saves the existing order", () => {
  const portal = read("src/pages/CustomerPortal.tsx");
  assert.match(portal, /function startEditingOrder[\s\S]*?setCart\(nextCart\)[\s\S]*?setEditingOrderId\(order\.id\)/);
  assert.match(portal, /if \(editingOrderId\)[\s\S]*?await updateOrder/);
});
