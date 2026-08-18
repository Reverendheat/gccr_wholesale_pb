import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const root = new URL("../", import.meta.url);
const read = (path) => readFileSync(new URL(path, root), "utf8");

test("tall desktop cart can reach submit controls", () => {
  const css = read("src/pages/CustomerPortal.css");
  const cartRule = css.match(/\.cart\s*\{([\s\S]*?)\}/)?.[1] ?? "";
  assert.match(cartRule, /max-height:\s*calc\(100dvh - 3rem\)/);
  assert.match(cartRule, /overflow-y:\s*auto/);
  assert.doesNotMatch(cartRule, /overscroll-behavior:\s*contain/);
});
