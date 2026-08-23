import assert from "node:assert/strict";
import test from "node:test";

import { groupCatalogByAudience } from "../src/lib/catalog.ts";

function item(id, audiences) {
  return {
    id,
    type: "ITEM",
    item_data: { name: id, description: "", variations: [] },
    wholesale_audiences: audiences,
  };
}

test("partitions Grocery and Cafe offerings in API order", () => {
  const groceryFirst = item("grocery-first", ["grocery"]);
  const cafeFirst = item("cafe-first", ["cafe_restaurant"]);
  const grocerySecond = item("grocery-second", ["grocery"]);
  const cafeSecond = item("cafe-second", ["cafe_restaurant"]);

  const grouped = groupCatalogByAudience([
    groceryFirst,
    cafeFirst,
    grocerySecond,
    cafeSecond,
  ]);

  assert.deepEqual(grouped.grocery, [groceryFirst, grocerySecond]);
  assert.deepEqual(grouped.cafe_restaurant, [cafeFirst, cafeSecond]);
});

test("includes dual-audience items once in each section", () => {
  const dual = item("dual", ["grocery", "cafe_restaurant"]);
  const grouped = groupCatalogByAudience([dual]);

  assert.deepEqual(grouped.grocery, [dual]);
  assert.deepEqual(grouped.cafe_restaurant, [dual]);
});

test("does not duplicate an item within one section", () => {
  const duplicateFlag = item("duplicate-flag", ["grocery", "grocery"]);
  const grouped = groupCatalogByAudience([duplicateFlag]);

  assert.deepEqual(grouped.grocery, [duplicateFlag]);
  assert.deepEqual(grouped.cafe_restaurant, []);
});
