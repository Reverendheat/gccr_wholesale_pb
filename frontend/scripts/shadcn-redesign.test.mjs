import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");

const styles = [
  "src/App.css",
  "src/pages/Login.css",
  "src/pages/CustomerPortal.css",
  "src/components/StaffLayout.css",
  "src/pages/staff/Orders.css",
  "src/pages/staff/StaffOrderModal.css",
  "src/components/InstallPrompt.css",
].map(read).join("\n");

test("Shadcn semantic tokens drive shared UI", () => {
  const app = read("src/App.css");
  for (const token of [
    "--background:",
    "--foreground:",
    "--card:",
    "--muted:",
    "--muted-foreground:",
    "--border:",
    "--ring:",
    "--destructive:",
  ]) {
    assert.match(app, new RegExp(token), `missing ${token}`);
  }
  assert.doesNotMatch(styles, /\bInter\b|linear-gradient/i);
});

test("controls retain accessible targets and focus treatment", () => {
  const app = read("src/App.css");
  assert.match(app, /button[\s\S]*?min-height:\s*44px/);
  assert.match(app, /:focus-visible[\s\S]*?var\(--ring\)/);
  assert.match(app, /input[\s\S]*?min-height:\s*44px/);
});

test("responsive back-office and customer layouts remain intact", () => {
  assert.match(read("src/components/StaffLayout.css"), /@media \(max-width: 700px\)/);
  assert.match(read("src/pages/CustomerPortal.css"), /@media \(max-width: 650px\)/);
  assert.match(read("src/pages/staff/Orders.css"), /content: attr\(data-label\)/);
});

test("customer actions have visible spacing", () => {
  const customers = read("src/pages/staff/Customers.tsx");
  const modalStyles = read("src/pages/staff/StaffOrderModal.css");
  assert.match(customers, /className="customer-action-buttons"/);
  assert.match(modalStyles, /\.customer-action-buttons\s*\{[^}]*display:\s*inline-flex[^}]*gap:\s*0\.5rem[^}]*\}/s);
});
