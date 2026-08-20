import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");

test("staff order API targets an existing customer", () => {
  const api = read("src/lib/api.ts");
  assert.match(api, /customers\/\$\{customerId\}\/orders/);
  assert.match(api, /placementNote/);
});

test("customers page exposes staff order composer", () => {
  const customers = read("src/pages/staff/Customers.tsx");
  assert.match(customers, /Create order/);
  assert.match(customers, /StaffOrderModal/);
});

test("staff can edit pending or confirmed pre-invoice orders through protected API", () => {
  const api = read("src/lib/api.ts");
  const orders = read("src/pages/staff/Orders.tsx");
  const composer = read("src/pages/staff/StaffOrderModal.tsx");
  assert.match(api, /orders\/\$\{id\}\/staff/);
  assert.match(api, /editReason/);
  assert.match(orders, /Edit order/);
  assert.match(composer, /updateStaffOrder/);
});

test("staff order composer requires reason and supports delivery quotes", () => {
  const composer = read("src/pages/staff/StaffOrderModal.tsx");
  assert.match(composer, /Phone order, email request, or other authorization/);
  assert.match(composer, /quoteFulfillment/);
  assert.match(composer, /Customer will see this order immediately/);
  const portal = read("src/pages/CustomerPortal.tsx");
  assert.match(portal, /placedBy\?\.type.*customer/);
});
