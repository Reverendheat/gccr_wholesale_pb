import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");

test("staff layout uses an off-canvas mobile navigation", () => {
  const css = read("src/components/StaffLayout.css");
  const layout = read("src/components/StaffLayout.tsx");
  assert.match(css, /@media \(max-width: 700px\)/);
  assert.match(css, /\.staff-shell\s*\{[^}]*grid-template-columns:\s*1fr/);
  assert.match(css, /\.staff-sidebar\s*\{[\s\S]*?position:\s*fixed[\s\S]*?transform:\s*translateX\(-105%\)/);
  assert.match(css, /\.staff-sidebar\.open\s*\{[^}]*transform:\s*translateX\(0\)/);
  assert.match(css, /\.staff-nav-backdrop\.open\s*\{[^}]*position:\s*fixed/);
  assert.match(layout, /className="staff-mobile-menu"/);
  assert.match(layout, /aria-expanded=\{navigationOpen\}/);
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
