import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import ts from "typescript";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { orderTotalCents } from "../src/lib/orderTotal.ts";

const require = createRequire(import.meta.url);
const source = readFileSync(new URL("../src/pages/staff/OrderListControls.tsx", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { jsx: ts.JsxEmit.ReactJSX, module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
});
const component = {};
new Function("require", "exports", outputText)((name) => {
  if (name.endsWith("/api")) return { fetchCompanies: async () => [] };
  if (name.endsWith("/useStaffOrders")) return { defaultOrderFilters: { q: "", status: "", company: "", from: "", to: "", sort: "newest" } };
  return require(name);
}, component);

for (const invoices of [false, true]) {
  test(`${invoices ? "invoice" : "order"} controls keep advanced filters out of the mobile default view`, () => {
    const html = renderToStaticMarkup(React.createElement(component.default, { invoices, onApply() {} }));
    assert.match(html, /type="search"/);
    assert.match(html, /aria-pressed="true"/);
    assert.match(html, /aria-expanded="false"/);
    assert.doesNotMatch(html, /<select|type="date"|>Apply<|>Reset</);
    if (invoices) assert.match(html, />Unpaid</);
    else assert.match(html, />Pending</);
  });
}

test("order total includes quantity and delivery, including legacy missing prices", () => {
  assert.equal(orderTotalCents({ lineItems: [{ quantity: 2, unit_price_cents: 6000 }, { quantity: 1 }], fulfillment: { fee_cents: 500 } }), 12500);
  assert.equal(orderTotalCents({ lineItems: [], fulfillment: undefined }), 0);
});
