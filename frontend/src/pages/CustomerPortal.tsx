import { useEffect, useState } from "react";
import { useAuth } from "../context/AuthContext";
import {
  fetchWholesaleCatalog,
  submitOrder,
  updateOrder,
  submitScheduledOrder,
  fetchOrders,
  fetchScheduledOrders,
  cancelScheduledOrder,
  fetchFulfillmentOptions,
  quoteFulfillment,
  type CatalogItem,
  type CatalogVariation,
  type Order,
  type ScheduledOrder,
  type ScheduleFrequency,
  type Fulfillment,
  type FulfillmentOptions,
  type FulfillmentQuoteResult,
} from "../lib/api";
import "./CustomerPortal.css";

type CartItem = {
  variation: CatalogVariation;
  itemName: string;
  quantity: number;
};

const FREQUENCY_LABELS: Record<ScheduleFrequency, string> = {
  weekly: "Weekly",
  biweekly: "Every two weeks",
  monthly: "Monthly",
  quarterly: "Quarterly",
};

function formatPrice(amount?: number): string {
  if (amount == null) return "Contact for price";
  return `$${(amount / 100).toFixed(2)}`;
}

function formatDate(iso: string): string {
  if (!iso) return "—";
  // PocketBase returns "2006-01-02 15:04:05.000Z" — replace the space with T
  // so the JS Date constructor parses it correctly across all browsers.
  return new Date(iso.replace(" ", "T")).toLocaleDateString();
}

function validateFulfillment(fulfillment: Fulfillment): string | null {
  if (fulfillment.method === "pickup") return null;
  if (
    !fulfillment.recipient_name?.trim() ||
    !fulfillment.recipient_phone?.trim() ||
    !fulfillment.address_line_1?.trim() ||
    !fulfillment.city?.trim() ||
    !fulfillment.state?.trim() ||
    !fulfillment.postal_code?.trim()
  ) {
    return "Complete all required delivery fields.";
  }
  if (!/^[A-Za-z]{2}$/.test(fulfillment.state.trim())) {
    return "Use a two-letter state code.";
  }
  if (!/^\d{5}(-\d{4})?$/.test(fulfillment.postal_code.trim())) {
    return "Use a valid US ZIP code.";
  }
  return null;
}

function VariationTag({
  variation,
  onAdd,
}: {
  variation: CatalogVariation;
  onAdd: () => void;
}) {
  const { name, price_money } = variation.item_variation_data;
  return (
    <div className="variation">
      <span className="variation-name">{name}</span>
      <span className="variation-price">{formatPrice(price_money?.amount)}</span>
      <button className="btn-add" onClick={onAdd}>
        + Add
      </button>
    </div>
  );
}

export default function CustomerPortal() {
  const { user, logout } = useAuth();

  const [catalog, setCatalog] = useState<CatalogItem[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [scheduledOrders, setScheduledOrders] = useState<ScheduledOrder[]>([]);
  const [fulfillmentOptions, setFulfillmentOptions] = useState<FulfillmentOptions | null>(null);
  const [cart, setCart] = useState<CartItem[]>([]);
  const [notes, setNotes] = useState("");
  const [fulfillment, setFulfillment] = useState<Fulfillment>({
    method: "pickup",
    recipient_name: String(user?.name ?? ""),
    recipient_phone: String(user?.phone ?? ""),
    address_line_1: "",
    address_line_2: "",
    city: "",
    state: "",
    postal_code: "",
    country: "US",
    instructions: "",
  });
  const [tab, setTab] = useState<"catalog" | "orders" | "scheduled">("catalog");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [cancelling, setCancelling] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [expandedOrder, setExpandedOrder] = useState<string | null>(null);
  const [editingOrderId, setEditingOrderId] = useState<string | null>(null);
  const [deliveryQuote, setDeliveryQuote] = useState<{ key: string; result: FulfillmentQuoteResult } | null>(null);
  const [quotingDeliveryKey, setQuotingDeliveryKey] = useState<string | null>(null);
  const [deliveryQuoteError, setDeliveryQuoteError] = useState<{ key: string; message: string } | null>(null);

  // Schedule mode state
  const [scheduleMode, setScheduleMode] = useState(false);
  const [frequency, setFrequency] = useState<ScheduleFrequency>("weekly");

  useEffect(() => {
    async function load() {
      try {
        const [items, orderList, scheduledList, options] = await Promise.all([
          fetchWholesaleCatalog(),
          fetchOrders(),
          fetchScheduledOrders(),
          fetchFulfillmentOptions(),
        ]);
        setCatalog(items);
        setOrders(orderList);
        setScheduledOrders(scheduledList);
        setFulfillmentOptions(options);
      } catch (e: unknown) {
        setError(e instanceof Error ? e.message : "Failed to load data");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  function addToCart(item: CatalogItem, variation: CatalogVariation) {
    setCart((prev) => {
      const existing = prev.find((c) => c.variation.id === variation.id);
      if (existing) {
        return prev.map((c) =>
          c.variation.id === variation.id
            ? { ...c, quantity: c.quantity + 1 }
            : c
        );
      }
      return [...prev, { variation, itemName: item.item_data.name, quantity: 1 }];
    });
  }

  function updateQty(variationId: string, qty: number) {
    if (qty <= 0) {
      setCart((prev) => prev.filter((c) => c.variation.id !== variationId));
    } else {
      setCart((prev) =>
        prev.map((c) =>
          c.variation.id === variationId ? { ...c, quantity: qty } : c
        )
      );
    }
  }


  const lineItemsFromCart = () =>
    cart.map((c) => ({
      variation_id: c.variation.id,
      quantity: c.quantity,
    }));

  function updateFulfillment(field: keyof Fulfillment, value: string) {
    setFulfillment((current) => ({ ...current, [field]: value }));
  }

  function startEditingOrder(order: Order) {
    const nextCart: CartItem[] = [];
    for (const lineItem of order.lineItems) {
      let match: CartItem | null = null;
      for (const item of catalog) {
        const variation = item.item_data.variations.find(
          (candidate) => candidate.id === lineItem.variation_id,
        );
        if (variation) {
          match = {
            variation,
            itemName: item.item_data.name,
            quantity: lineItem.quantity,
          };
          break;
        }
      }
      if (!match) {
        setError("This order contains an item no longer available in the wholesale catalog.");
        return;
      }
      nextCart.push(match);
    }

    setCart(nextCart);
    setNotes(order.notes ?? "");
    setFulfillment(order.fulfillment ?? {
      method: "pickup",
      recipient_name: String(user?.name ?? ""),
      recipient_phone: String(user?.phone ?? ""),
      country: "US",
    });
    setEditingOrderId(order.id);
    setScheduleMode(false);
    setExpandedOrder(null);
    setError(null);
    setTab("catalog");
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function cancelEditingOrder() {
    setEditingOrderId(null);
    setCart([]);
    setNotes("");
    setError(null);
    setTab("orders");
  }

  async function handleSubmitOrder() {
    if (cart.length === 0) return;
    const fulfillmentError = validateFulfillment(fulfillment);
    if (fulfillmentError) {
      setError(fulfillmentError);
      return;
    }
    if (fulfillment.method === "delivery" && !activeDeliveryQuote) {
      setError(activeDeliveryQuoteError ?? "Wait for a valid delivery quote before submitting.");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      if (editingOrderId) {
        const order = await updateOrder(
          editingOrderId,
          lineItemsFromCart(),
          notes,
          fulfillment,
        );
        setOrders((prev) => prev.map((existing) => existing.id === order.id ? order : existing));
        setEditingOrderId(null);
        setSuccess("Order updated successfully!");
      } else {
        const order = await submitOrder(lineItemsFromCart(), notes, fulfillment);
        setOrders((prev) => [order, ...prev]);
        setSuccess("Order placed successfully!");
      }
      setCart([]);
      setNotes("");
      setTab("orders");
      setTimeout(() => setSuccess(null), 4000);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Order submission failed");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleSubmitScheduledOrder() {
    if (cart.length === 0) return;
    const fulfillmentError = validateFulfillment(fulfillment);
    if (fulfillmentError) {
      setError(fulfillmentError);
      return;
    }
    if (fulfillment.method === "delivery" && !activeDeliveryQuote) {
      setError(activeDeliveryQuoteError ?? "Wait for a valid delivery quote before scheduling.");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const { order, scheduledOrder } = await submitScheduledOrder(
        lineItemsFromCart(),
        notes,
        frequency,
        fulfillment,
      );
      setOrders((prev) => [order, ...prev]);
      setScheduledOrders((prev) => [scheduledOrder, ...prev]);
      setCart([]);
      setNotes("");
      setScheduleMode(false);
      setSuccess(
        `Order placed and recurring ${FREQUENCY_LABELS[frequency].toLowerCase()} schedule created!`
      );
      setTab("scheduled");
      setTimeout(() => setSuccess(null), 5000);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Scheduled order failed");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleCancel(id: string) {
    setCancelling(id);
    setError(null);
    try {
      await cancelScheduledOrder(id);
      setScheduledOrders((prev) =>
        prev.filter((s) => s.id !== id)
      );
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Could not cancel schedule");
    } finally {
      setCancelling(null);
    }
  }

  const cartTotal = cart.reduce((sum, c) => {
    const price = c.variation.item_variation_data.price_money?.amount ?? 0;
    return sum + price * c.quantity;
  }, 0);

  const deliveryQuoteKey = JSON.stringify({
    items: cart.map((item) => [item.variation.id, item.quantity]),
    fulfillment,
  });
  const activeDeliveryQuote = deliveryQuote?.key === deliveryQuoteKey ? deliveryQuote.result : null;
  const activeDeliveryQuoteError = deliveryQuoteError?.key === deliveryQuoteKey ? deliveryQuoteError.message : null;
  const quotingDelivery = quotingDeliveryKey === deliveryQuoteKey;

  useEffect(() => {
    if (fulfillment.method !== "delivery" || cart.length === 0 || validateFulfillment(fulfillment)) return;

    const requestKey = deliveryQuoteKey;
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setQuotingDeliveryKey(requestKey);
      try {
        const quote = await quoteFulfillment(
          cart.map((item) => ({ variation_id: item.variation.id, quantity: item.quantity })),
          fulfillment,
          controller.signal,
        );
        setDeliveryQuote({ key: requestKey, result: quote });
      } catch (quoteError: unknown) {
        if (quoteError instanceof DOMException && quoteError.name === "AbortError") return;
        setDeliveryQuoteError({
          key: requestKey,
          message: quoteError instanceof Error ? quoteError.message : "Could not calculate delivery quote",
        });
      } finally {
        if (!controller.signal.aborted) {
          setQuotingDeliveryKey((current) => current === requestKey ? null : current);
        }
      }
    }, 600);

    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [cart, deliveryQuoteKey, fulfillment]);

  // Build a lookup from variation ID → readable name for the orders tab.
  const variationNames: Record<string, string> = {};
  for (const item of catalog) {
    for (const v of item.item_data.variations) {
      variationNames[v.id] = `${item.item_data.name} — ${v.item_variation_data.name}`;
    }
  }

  return (
    <div className="portal-shell">
      <header className="portal-header">
        <img src="/logo.png" alt="Logo" className="portal-logo" />
        <div className="portal-header-right">
          <span className="portal-user">{user?.email}</span>
          <button onClick={logout} className="portal-logout">
            Sign out
          </button>
        </div>
      </header>

      <div className="portal-tabs">
        <button
          className={tab === "catalog" ? "active" : ""}
          onClick={() => setTab("catalog")}
        >
          Order
        </button>
        <button
          className={tab === "orders" ? "active" : ""}
          onClick={() => setTab("orders")}
        >
          Account Orders
        </button>
        <button
          className={tab === "scheduled" ? "active" : ""}
          onClick={() => setTab("scheduled")}
        >
          Scheduled
          {scheduledOrders.length > 0 && (
            <span className="tab-badge">{scheduledOrders.length}</span>
          )}
        </button>
      </div>

      <main className="portal-main">
        {loading && <p className="muted">Loading…</p>}
        {error && <p className="portal-error">{error}</p>}
        {success && <p className="portal-success">{success}</p>}

        {/* CATALOG + CART */}
        {tab === "catalog" && !loading && (
          <div className="catalog-layout">
            <section className="catalog-grid">
              <h2>Wholesale Catalog</h2>
              {catalog.length === 0 && (
                <p className="muted">No items available.</p>
              )}
              {catalog.map((item) => (
                <div key={item.id} className="catalog-card">
                  <div className="catalog-card-name">{item.item_data.name}</div>
                  {item.item_data.description && (
                    <p className="catalog-card-desc">
                      {item.item_data.description}
                    </p>
                  )}
                  <div className="catalog-card-variations">
                    {item.item_data.variations.map((v) => (
                      <VariationTag
                        key={v.id}
                        variation={v}
                        onAdd={() => addToCart(item, v)}
                      />
                    ))}
                  </div>
                </div>
              ))}
            </section>

            <aside className="cart">
              <h2>{editingOrderId ? "Edit Order" : "Your Order"}</h2>
              {editingOrderId && (
                <p className="muted">Changes are allowed while this order is pending.</p>
              )}
              {cart.length === 0 ? (
                <p className="muted">Add items from the catalog.</p>
              ) : (
                <>
                  <ul className="cart-list">
                    {cart.map((c) => (
                      <li key={c.variation.id} className="cart-item">
                        <div className="cart-item-info">
                          <span className="cart-item-name">{c.itemName}</span>
                          <span className="cart-item-variation">
                            {c.variation.item_variation_data.name}
                          </span>
                        </div>
                        <div className="cart-item-controls">
                          <button
                            onClick={() =>
                              updateQty(c.variation.id, c.quantity - 1)
                            }
                          >
                            −
                          </button>
                          <span>{c.quantity}</span>
                          <button
                            onClick={() =>
                              updateQty(c.variation.id, c.quantity + 1)
                            }
                          >
                            +
                          </button>
                        </div>
                        <span className="cart-item-subtotal">
                          {formatPrice(
                            (c.variation.item_variation_data.price_money
                              ?.amount ?? 0) * c.quantity
                          )}
                        </span>
                      </li>
                    ))}
                  </ul>

                  {cartTotal > 0 && (
                    <div className="cart-total">
                      <div>Merchandise subtotal: {formatPrice(cartTotal)}</div>
                      {activeDeliveryQuote && (
                        <>
                          <div>Delivery fee: {formatPrice(activeDeliveryQuote.fulfillment.fee_cents ?? 0)}</div>
                          <div>Estimated total: {formatPrice(activeDeliveryQuote.total_cents)}</div>
                        </>
                      )}
                      {fulfillment.method === "pickup" && <div>Estimated total: {formatPrice(cartTotal)}</div>}
                    </div>
                  )}

                  <section className="fulfillment-picker">
                    <h3>Fulfillment</h3>
                    <div className="fulfillment-methods">
                      <label>
                        <input
                          type="radio"
                          name="fulfillment-method"
                          checked={fulfillment.method === "pickup"}
                          onChange={() => updateFulfillment("method", "pickup")}
                        />
                        Pickup
                      </label>
                      <label>
                        <input
                          type="radio"
                          name="fulfillment-method"
                          checked={fulfillment.method === "delivery"}
                          onChange={() => updateFulfillment("method", "delivery")}
                        />
                        Delivery{fulfillmentOptions && ` (within ${fulfillmentOptions.max_miles} driving miles)`}
                      </label>
                    </div>

                    {fulfillment.method === "delivery" && (
                      <div className="delivery-fields">
                        <label>
                          Recipient name *
                          <input
                            value={fulfillment.recipient_name ?? ""}
                            onChange={(e) => updateFulfillment("recipient_name", e.target.value)}
                            autoComplete="name"
                          />
                        </label>
                        <label>
                          Recipient phone *
                          <input
                            type="tel"
                            value={fulfillment.recipient_phone ?? ""}
                            onChange={(e) => updateFulfillment("recipient_phone", e.target.value)}
                            autoComplete="tel"
                          />
                        </label>
                        <label>
                          Address line 1 *
                          <input
                            value={fulfillment.address_line_1 ?? ""}
                            onChange={(e) => updateFulfillment("address_line_1", e.target.value)}
                            autoComplete="address-line1"
                          />
                        </label>
                        <label>
                          Address line 2
                          <input
                            value={fulfillment.address_line_2 ?? ""}
                            onChange={(e) => updateFulfillment("address_line_2", e.target.value)}
                            autoComplete="address-line2"
                          />
                        </label>
                        <label>
                          City *
                          <input
                            value={fulfillment.city ?? ""}
                            onChange={(e) => updateFulfillment("city", e.target.value)}
                            autoComplete="address-level2"
                          />
                        </label>
                        <div className="delivery-region-row">
                          <label>
                            State *
                            <input
                              value={fulfillment.state ?? ""}
                              onChange={(e) => updateFulfillment("state", e.target.value.toUpperCase())}
                              maxLength={2}
                              autoComplete="address-level1"
                            />
                          </label>
                          <label>
                            ZIP code *
                            <input
                              value={fulfillment.postal_code ?? ""}
                              onChange={(e) => updateFulfillment("postal_code", e.target.value)}
                              autoComplete="postal-code"
                            />
                          </label>
                        </div>
                        <label>
                          Delivery instructions
                          <textarea
                            value={fulfillment.instructions ?? ""}
                            onChange={(e) => updateFulfillment("instructions", e.target.value)}
                            rows={2}
                          />
                        </label>
                        <p className="delivery-country">
                          United States addresses only{fulfillmentOptions && ` · within ${fulfillmentOptions.max_miles} driving miles`}
                        </p>
                        {quotingDelivery && <p className="delivery-quote pending">Calculating driving distance…</p>}
                        {activeDeliveryQuoteError && <p className="delivery-quote error">{activeDeliveryQuoteError}</p>}
                        {activeDeliveryQuote && (
                          <p className="delivery-quote success">
                            {activeDeliveryQuote.fulfillment.distance_miles?.toFixed(1)} driving miles · {activeDeliveryQuote.fulfillment.fee_cents
                              ? `${formatPrice(activeDeliveryQuote.fulfillment.fee_cents)} delivery fee`
                              : "Free delivery"}
                          </p>
                        )}
                      </div>
                    )}
                  </section>

                  <textarea
                    className="cart-notes"
                    placeholder="Order notes or special requests…"
                    value={notes}
                    onChange={(e) => setNotes(e.target.value)}
                    rows={3}
                  />

                  {/* Schedule toggle */}
                  {!editingOrderId && (
                    <label className="schedule-toggle">
                      <input
                        type="checkbox"
                        checked={scheduleMode}
                        onChange={(e) => setScheduleMode(e.target.checked)}
                      />
                      <span>Set as a recurring order</span>
                    </label>
                  )}

                  {!editingOrderId && scheduleMode && (
                    <div className="schedule-frequency">
                      <p className="schedule-frequency-label">Repeat every:</p>
                      <div className="schedule-frequency-options">
                        {(
                          [
                            "weekly",
                            "biweekly",
                            "monthly",
                            "quarterly",
                          ] as ScheduleFrequency[]
                        ).map((f) => (
                          <label key={f} className="frequency-option">
                            <input
                              type="radio"
                              name="frequency"
                              value={f}
                              checked={frequency === f}
                              onChange={() => setFrequency(f)}
                            />
                            {FREQUENCY_LABELS[f]}
                          </label>
                        ))}
                      </div>
                    </div>
                  )}

                  {!editingOrderId && scheduleMode ? (
                    <button
                      className="cart-submit cart-submit-schedule"
                      onClick={handleSubmitScheduledOrder}
                      disabled={submitting || (fulfillment.method === "delivery" && (!activeDeliveryQuote || quotingDelivery))}
                    >
                      {submitting
                        ? "Scheduling…"
                        : `Schedule ${FREQUENCY_LABELS[frequency]} Order`}
                    </button>
                  ) : (
                    <>
                      <button
                        className="cart-submit"
                        onClick={handleSubmitOrder}
                        disabled={submitting || (fulfillment.method === "delivery" && (!activeDeliveryQuote || quotingDelivery))}
                      >
                        {submitting
                          ? editingOrderId ? "Saving changes…" : "Placing order…"
                          : editingOrderId ? "Save Changes" : "Place Order"}
                      </button>
                      {editingOrderId && (
                        <button
                          className="link-btn cart-cancel-edit"
                          onClick={cancelEditingOrder}
                          disabled={submitting}
                        >
                          Cancel editing
                        </button>
                      )}
                    </>
                  )}
                </>
              )}
            </aside>
          </div>
        )}

        {/* ORDERS */}
        {tab === "orders" && !loading && (
          <div>
            <h2>Account Orders</h2>
            {orders.length === 0 ? (
              <p className="muted">No orders yet.</p>
            ) : (
              <table className="orders-table">
                <thead>
                  <tr>
                    <th>Order #</th>
                    <th>Date</th>
                    <th>Submitted By</th>
                    <th>Items</th>
                    <th>Status</th>
                    <th>Invoice</th>
                  </tr>
                </thead>
                <tbody>
                  {orders.map((o) => (
                    <>
                      <tr
                        key={o.id}
                        className="order-row-clickable"
                        onClick={() =>
                          setExpandedOrder(expandedOrder === o.id ? null : o.id)
                        }
                      >
                        <td className="mono">{o.id.slice(0, 8)}</td>
                        <td>{formatDate(o.created)}</td>
                        <td>
                          {o.submittedBy?.name || "Unknown"}
                          {o.submittedBy?.type === "staff" && " (GCCR staff)"}
                        </td>
                        <td>{o.lineItems.length} item(s)</td>
                        <td>
                          <span className={`status-badge status-${o.status}`}>
                            {o.status}
                          </span>
                        </td>
                        <td>
                          {o.squareInvoiceUrl ? (
                            <a
                              className="invoice-link"
                              href={o.squareInvoiceUrl}
                              target="_blank"
                              rel="noopener noreferrer"
                              onClick={(e) => e.stopPropagation()}
                            >
                              View invoice
                            </a>
                          ) : (
                            <span className="muted">—</span>
                          )}
                        </td>
                      </tr>
                      {expandedOrder === o.id && (
                        <tr key={`${o.id}-detail`} className="order-detail-row">
                          <td colSpan={6}>
                            <div className="order-detail">
                              <ul className="order-detail-items">
                                <li>
                                  <strong>Placed by:</strong>{" "}
                                  {o.placedBy?.type === "staff" ? `${o.placedBy.name} (GCCR staff)` : (o.submittedBy?.name || "Customer")}
                                </li>
                                <li>
                                  <strong>Fulfillment:</strong>{" "}
                                  {o.fulfillment?.method === "delivery" ? "Delivery" : "Pickup"}
                                </li>
                                {o.fulfillment?.method === "delivery" && o.fulfillment.address_line_1 && (
                                  <>
                                    <li>
                                      {o.fulfillment.address_line_1}
                                      {o.fulfillment.address_line_2 ? `, ${o.fulfillment.address_line_2}` : ""}, {o.fulfillment.city}, {o.fulfillment.state} {o.fulfillment.postal_code}
                                    </li>
                                    <li>
                                      {(o.fulfillment.distance_miles ?? 0).toFixed(1)} driving miles · {o.fulfillment.fee_cents
                                        ? `${formatPrice(o.fulfillment.fee_cents)} delivery fee`
                                        : "Free delivery"}
                                    </li>
                                  </>
                                )}
                                {o.lineItems.map((li, i) => (
                                  <li key={i}>
                                    <span className="order-detail-name">
                                      {li.name ??
                                        variationNames[li.variation_id] ??
                                        li.variation_id}
                                    </span>
                                    <span className="order-detail-qty">
                                      × {li.quantity}
                                    </span>
                                  </li>
                                ))}
                              </ul>
                              {o.notes && (
                                <p className="order-detail-notes">
                                  <strong>Notes:</strong> {o.notes}
                                </p>
                              )}
                              {o.customer === user?.id && o.status === "pending" && (o.placedBy?.type ?? "customer") === "customer" && (
                                <button
                                  className="btn-edit-order"
                                  onClick={() => startEditingOrder(o)}
                                >
                                  Edit order
                                </button>
                              )}
                            </div>
                          </td>
                        </tr>
                      )}
                    </>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}

        {/* SCHEDULED ORDERS */}
        {tab === "scheduled" && !loading && (
          <div>
            <h2>Account Schedules</h2>
            <p className="muted schedule-info">
              Account members share schedule visibility. Scheduled orders run automatically;
              only schedule creator can cancel one.
            </p>
            {scheduledOrders.length === 0 ? (
              <p className="muted">
                No active schedules.{" "}
                <button
                  className="link-btn"
                  onClick={() => setTab("catalog")}
                >
                  Create one from the catalog.
                </button>
              </p>
            ) : (
              <table className="orders-table">
                <thead>
                  <tr>
                    <th>Schedule #</th>
                    <th>Created</th>
                    <th>Submitted By</th>
                    <th>Items</th>
                    <th>Frequency</th>
                    <th>Fulfillment</th>
                    <th>Next Order</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {scheduledOrders.map((s) => (
                    <tr key={s.id}>
                      <td className="mono">{s.id.slice(0, 8)}</td>
                      <td>{formatDate(s.created)}</td>
                      <td>{s.submittedBy?.name || "Unknown"}</td>
                      <td>{s.lineItems.length} item(s)</td>
                      <td>
                        <span className="frequency-badge">
                          {FREQUENCY_LABELS[s.frequency]}
                        </span>
                      </td>
                      <td>{s.fulfillment?.method === "delivery" ? "Delivery" : "Pickup"}</td>
                      <td>{formatDate(s.next_run_at)}</td>
                      <td>
                        {s.customer === user?.id ? (
                          <button
                            className="btn-cancel"
                            onClick={() => handleCancel(s.id)}
                            disabled={cancelling === s.id}
                          >
                            {cancelling === s.id ? "Cancelling…" : "Cancel"}
                          </button>
                        ) : (
                          <span className="muted">Creator only</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
