import { useEffect, useState } from "react";
import { useAuth } from "../context/AuthContext";
import {
  fetchWholesaleCatalog,
  submitOrder,
  submitScheduledOrder,
  fetchOrders,
  fetchScheduledOrders,
  cancelScheduledOrder,
  type CatalogItem,
  type CatalogVariation,
  type Order,
  type ScheduledOrder,
  type ScheduleFrequency,
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
  const [cart, setCart] = useState<CartItem[]>([]);
  const [notes, setNotes] = useState("");
  const [tab, setTab] = useState<"catalog" | "orders" | "scheduled">("catalog");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [cancelling, setCancelling] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Schedule mode state
  const [scheduleMode, setScheduleMode] = useState(false);
  const [frequency, setFrequency] = useState<ScheduleFrequency>("weekly");

  useEffect(() => {
    async function load() {
      try {
        const [items, orderList, scheduledList] = await Promise.all([
          fetchWholesaleCatalog(),
          fetchOrders(),
          fetchScheduledOrders(),
        ]);
        setCatalog(items);
        setOrders(orderList);
        setScheduledOrders(scheduledList);
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

  async function handleSubmitOrder() {
    if (cart.length === 0) return;
    setSubmitting(true);
    setError(null);
    try {
      const order = await submitOrder(lineItemsFromCart(), notes);
      setOrders((prev) => [order, ...prev]);
      setCart([]);
      setNotes("");
      setSuccess("Order placed successfully!");
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
    setSubmitting(true);
    setError(null);
    try {
      const { order, scheduled_order } = await submitScheduledOrder(
        lineItemsFromCart(),
        notes,
        frequency,
      );
      setOrders((prev) => [order, ...prev]);
      setScheduledOrders((prev) => [scheduled_order, ...prev]);
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
          My Orders
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
              <h2>Your Order</h2>
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
                      Estimated total: {formatPrice(cartTotal)}
                    </div>
                  )}

                  <textarea
                    className="cart-notes"
                    placeholder="Delivery notes or special requests…"
                    value={notes}
                    onChange={(e) => setNotes(e.target.value)}
                    rows={3}
                  />

                  {/* Schedule toggle */}
                  <label className="schedule-toggle">
                    <input
                      type="checkbox"
                      checked={scheduleMode}
                      onChange={(e) => setScheduleMode(e.target.checked)}
                    />
                    <span>Set as a recurring order</span>
                  </label>

                  {scheduleMode && (
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

                  {scheduleMode ? (
                    <button
                      className="cart-submit cart-submit-schedule"
                      onClick={handleSubmitScheduledOrder}
                      disabled={submitting}
                    >
                      {submitting
                        ? "Scheduling…"
                        : `Schedule ${FREQUENCY_LABELS[frequency]} Order`}
                    </button>
                  ) : (
                    <button
                      className="cart-submit"
                      onClick={handleSubmitOrder}
                      disabled={submitting}
                    >
                      {submitting ? "Placing order…" : "Place Order"}
                    </button>
                  )}
                </>
              )}
            </aside>
          </div>
        )}

        {/* ORDERS */}
        {tab === "orders" && !loading && (
          <div>
            <h2>My Orders</h2>
            {orders.length === 0 ? (
              <p className="muted">No orders yet.</p>
            ) : (
              <table className="orders-table">
                <thead>
                  <tr>
                    <th>Order #</th>
                    <th>Date</th>
                    <th>Items</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {orders.map((o) => (
                    <tr key={o.id}>
                      <td className="mono">{o.id.slice(0, 8)}</td>
                      <td>{formatDate(o.created)}</td>
                      <td>{o.line_items.length} item(s)</td>
                      <td>
                        <span className={`status-badge status-${o.status}`}>
                          {o.status}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}

        {/* SCHEDULED ORDERS */}
        {tab === "scheduled" && !loading && (
          <div>
            <h2>Scheduled Orders</h2>
            <p className="muted schedule-info">
              Scheduled orders are placed automatically on the next due date.
              Your first order is placed immediately when you create a schedule.
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
                    <th>Items</th>
                    <th>Frequency</th>
                    <th>Next Order</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {scheduledOrders.map((s) => (
                    <tr key={s.id}>
                      <td className="mono">{s.id.slice(0, 8)}</td>
                      <td>{formatDate(s.created)}</td>
                      <td>{s.line_items.length} item(s)</td>
                      <td>
                        <span className="frequency-badge">
                          {FREQUENCY_LABELS[s.frequency]}
                        </span>
                      </td>
                      <td>{formatDate(s.next_run_at)}</td>
                      <td>
                        <button
                          className="btn-cancel"
                          onClick={() => handleCancel(s.id)}
                          disabled={cancelling === s.id}
                        >
                          {cancelling === s.id ? "Cancelling…" : "Cancel"}
                        </button>
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
