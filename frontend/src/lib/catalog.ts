import type { CatalogItem, WholesaleAudience } from "./api";

export const WHOLESALE_AUDIENCE_SECTIONS: ReadonlyArray<{
  audience: WholesaleAudience;
  label: string;
}> = [
  { audience: "grocery", label: "Grocery" },
  { audience: "cafe_restaurant", label: "Cafe / Restaurant" },
];

export function groupCatalogByAudience(
  items: CatalogItem[],
): Record<WholesaleAudience, CatalogItem[]> {
  const grouped: Record<WholesaleAudience, CatalogItem[]> = {
    grocery: [],
    cafe_restaurant: [],
  };

  for (const item of items) {
    if (item.wholesale_audiences.includes("grocery")) {
      grouped.grocery.push(item);
    }
    if (item.wholesale_audiences.includes("cafe_restaurant")) {
      grouped.cafe_restaurant.push(item);
    }
  }

  return grouped;
}
