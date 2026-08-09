import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");

test("staff layout switches from sidebar to mobile header", () => {
  const css = read("src/components/StaffLayout.css");
  assert.match(css, /@media \(max-width: 700px\)/);
  assert.match(css, /\.staff-shell\s*\{\s*flex-direction: column/);
  assert.match(css, /\.staff-sidebar\s*\{[\s\S]*?width: 100%/);
  assert.match(css, /\.sidebar-nav\s*\{[\s\S]*?grid-template-columns: repeat\(3, 1fr\)/);
});

test("staff tables become labeled cards on mobile", () => {
  const css = read("src/pages/staff/Orders.css");
  assert.match(css, /@media \(max-width: 700px\)/);
  assert.match(css, /\.orders-table thead[\s\S]*?display: none/);
  assert.match(css, /content: attr\(data-label\)/);

  for (const path of [
    "src/pages/staff/Orders.tsx",
    "src/pages/staff/Invoices.tsx",
    "src/pages/staff/Customers.tsx",
  ]) {
    assert.match(read(path), /data-label=/, `${path} needs mobile cell labels`);
  }
});

test("order drawer and invite modal fit phone viewport", () => {
  const css = read("src/pages/staff/Orders.css");
  assert.match(css, /\.order-drawer\s*\{[\s\S]*?width: 100vw/);
  assert.match(css, /\.modal-card\s*\{[\s\S]*?max-height: calc\(100dvh - 2rem\)/);
});
