import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");

test("customer lookup tolerates a null suggested_accounts response", () => {
  const customers = read("src/pages/staff/Customers.tsx");
  assert.match(customers, /result\.suggested_accounts\s*\?\?\s*\[\]/);
});
