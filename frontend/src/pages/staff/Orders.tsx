import { useEffect, useState } from "react";
import { fetchStaffOrders, updateOrderStatus, sendInvoice, type Order } from "../../lib/api";
import "./Orders.css";

const ORDER_STATUSES = [
  "pending",
  "confirmed",
  "delivered",
  "invoiced",
  "paid",
  "cancelled",
] as const;

function formatDate(iso: string): string {
  if (!iso) return "—";
  return new Date(iso.replace(" ", "T")).toLocaleDateString();
}

function OrderDrawer({
  order,
  onClose,
  onStatusChange,
  onOrderUpdate,
}: {
  order: Order;
  onClose: () => void;
  onStatusChange: (id: string, status: string) => void;
  onOrderUpdate: (updated: Order) => void;
}) {
  const [status, setStatus] = useState(order.status);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [invoicing, setInvoicing] = useState(false);
  const customer = order.expand?.customer;

  async function handleStatusChange(newStatus: string) {
    setSaving(true);
    setSaveError(null);
    try {
      await updateOrderStatus(order.id, newStatus);
      setStatus(newStatus);
      onStatusChange(order.id, newStatus);
    } catch (e: unknown) {
      setSaveError(e instanceof Error ? e.message : "Failed to update status");
    } finally {
      setSaving(false);
    }
  }

  async function handleSendInvoice() {
    setInvoicing(true);
    setSaveError(null);
    try {
      const result = await sendInvoice(order.id);
      setStatus("invoiced");
      onStatusChange(order.id, "invoiced");
      onOrderUpdate(result.order);
    } catch (e: unknown) {
      setSaveError(e instanceof Error ? e.message : "Failed to send invoice");
    } finally {
      setInvoicing(false);
    }
  }

  const canSendInvoice =
    order.squareOrderId &&
    !order.squareInvoiceId &&
    status !== "paid" &&
    status !== "cancelled";

  return (
    <>
      <div className="drawer-backdrop" onClick={onClose} />
      <aside className="order-drawer">
        <div className="drawer-header">
          <div>
            <h3 className="drawer-title">Order Detail</h3>
            <span className="drawer-id mono">{order.id}</span>
          </div>
          <button className="drawer-close" onClick={onClose} aria-label="Close">
            ✕
          </button>
        </div>

        <div className="drawer-body">
          {/* Meta */}
          <section className="drawer-section">
            <div className="drawer-meta-grid">
              <div className="meta-row">
                <span className="meta-label">Date</span>
                <span>{formatDate(order.created)}</span>
              </div>
              <div className="meta-row">
                <span className="meta-label">Customer</span>
                <span>
                  {customer ? (
                    <>
                      <strong>{customer.name}</strong>
                      <br />
                      <span className="meta-sub">{customer.email}</span>
                      {customer.phone && (
                        <>
                          <br />
                          <span className="meta-sub">{customer.phone}</span>
                        </>
                      )}
                    </>
                  ) : (
                    <span className="mono">{order.customer.slice(0, 8)}</span>
                  )}
                </span>
              </div>
              {order.squareOrderId && (
                <div className="meta-row">
                  <span className="meta-label">Square Order</span>
                  <span className="mono">{order.squareOrderId}</span>
                </div>
              )}
              <div className="meta-row">
                <span className="meta-label">Status</span>
                <span className="drawer-status-control">
                  <select
                    value={status}
                    onChange={(e) => handleStatusChange(e.target.value)}
                    disabled={saving}
                    className="status-select"
                  >
                    {ORDER_STATUSES.map((s) => (
                      <option key={s} value={s}>
                        {s.charAt(0).toUpperCase() + s.slice(1)}
                      </option>
                    ))}
                  </select>
                  {saving && <span className="meta-sub">Saving…</span>}
                  {saveError && (
                    <span className="staff-error">{saveError}</span>
                  )}
                </span>
              </div>
            </div>
          </section>

          {/* Line Items */}
          <section className="drawer-section">
            <h4 className="drawer-section-title">Line Items</h4>
            <table className="drawer-items-table">
              <thead>
                <tr>
                  <th>Variation ID</th>
                  <th>Qty</th>
                  <th>Note</th>
                </tr>
              </thead>
              <tbody>
                {order.lineItems.map((li, i) => (
                  <tr key={i}>
                    <td className="mono">{li.variation_id}</td>
                    <td>{li.quantity}</td>
                    <td>{li.note || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>

          {/* Notes */}
          {order.notes && (
            <section className="drawer-section">
              <h4 className="drawer-section-title">Order Notes</h4>
              <p className="drawer-notes">{order.notes}</p>
            </section>
          )}

          {/* Invoice */}
          <section className="drawer-section">
            <h4 className="drawer-section-title">Invoice</h4>
            {order.squareInvoiceId ? (
              <div className="invoice-sent">
                <span className="invoice-sent-label">Invoice sent</span>
                <span className="mono invoice-id">{order.squareInvoiceId}</span>
                {order.squareInvoiceUrl && (
                  <a
                    href={order.squareInvoiceUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="invoice-link"
                  >
                    View invoice ↗
                  </a>
                )}
              </div>
            ) : canSendInvoice ? (
              <div className="invoice-action">
                <p className="meta-sub">
                  Sends a payment request email to the customer via Square.
                  Due date is set to 30 days from today.
                </p>
                <button
                  className="btn-send-invoice"
                  onClick={handleSendInvoice}
                  disabled={invoicing}
                >
                  {invoicing ? "Sending…" : "Send Invoice"}
                </button>
              </div>
            ) : (
              <p className="muted">
                {status === "paid"
                  ? "Order is already paid."
                  : status === "cancelled"
                    ? "Cannot invoice a cancelled order."
                    : "No Square order linked."}
              </p>
            )}
          </section>
        </div>
      </aside>
    </>
  );
}

export default function Orders() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Order | null>(null);

  useEffect(() => {
    fetchStaffOrders()
      .then(setOrders)
      .catch((e: unknown) =>
        setError(e instanceof Error ? e.message : "Failed to load orders")
      )
      .finally(() => setLoading(false));
  }, []);

  function handleStatusChange(id: string, status: string) {
    setOrders((prev) =>
      prev.map((o) => (o.id === id ? { ...o, status } : o))
    );
    setSelected((prev) => (prev?.id === id ? { ...prev, status } : prev));
  }

  function handleOrderUpdate(updated: Order) {
    setOrders((prev) => prev.map((o) => (o.id === updated.id ? updated : o)));
    setSelected(updated);
  }

  if (loading) return <p className="muted">Loading orders…</p>;
  if (error) return <p className="staff-error">{error}</p>;

  return (
    <div>
      <h2>Orders</h2>
      {orders.length === 0 ? (
        <p className="muted">No orders yet.</p>
      ) : (
        <table className="orders-table orders-table-clickable">
          <thead>
            <tr>
              <th>Order #</th>
              <th>Date</th>
              <th>Customer</th>
              <th>Items</th>
              <th>Square Order</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {orders.map((o) => {
              const customer = o.expand?.customer;
              return (
                <tr
                  key={o.id}
                  onClick={() => setSelected(o)}
                  className={selected?.id === o.id ? "row-selected" : ""}
                >
                  <td className="mono">{o.id.slice(0, 8)}</td>
                  <td>{formatDate(o.created)}</td>
                  <td>
                    {customer ? (
                      customer.name
                    ) : (
                      <span className="mono">{o.customer.slice(0, 8)}</span>
                    )}
                  </td>
                  <td>{o.lineItems.length} item(s)</td>
                  <td className="mono">
                    {o.squareOrderId
                      ? o.squareOrderId.slice(0, 10) + "…"
                      : "—"}
                  </td>
                  <td>
                    <span className={`status-badge status-${o.status}`}>
                      {o.status}
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      {selected && (
        <OrderDrawer
          order={selected}
          onClose={() => setSelected(null)}
          onStatusChange={handleStatusChange}
          onOrderUpdate={handleOrderUpdate}
        />
      )}
    </div>
  );
}
