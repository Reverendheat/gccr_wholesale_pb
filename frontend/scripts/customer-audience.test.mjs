import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");

test("staff API reads and patches customer audience access", () => {
  const api = read("src/lib/api.ts");
  assert.match(api, /fetchCustomerAudienceAccess[\s\S]*customers\/\$\{customerId\}\/audiences/);
  assert.match(api, /updateCustomerAudienceAccess[\s\S]*method:\s*"PATCH"/);
  assert.match(api, /body:\s*JSON\.stringify\(access\)/);
});

test("customers page exposes independent Grocery and Cafe controls", () => {
  const customers = read("src/pages/staff/Customers.tsx");
  assert.match(customers, />\s*Catalog access\s*</);
  assert.match(customers, /checked=\{access\?\.grocery/);
  assert.match(customers, /checked=\{access\?\.cafe_restaurant/);
  assert.match(customers, /setAudienceFor\(customer\)/);
  assert.match(customers, /Customers with neither group see no standard catalog items/);
});
