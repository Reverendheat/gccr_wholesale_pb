import type { Order } from "./api";

export function orderTotalCents(order: Pick<Order, "lineItems" | "fulfillment">): number {
  return order.lineItems.reduce((sum, item) => sum + item.quantity * (item.unit_price_cents ?? 0), 0)
    + (order.fulfillment?.fee_cents ?? 0);
}
