import { useEffect, useState } from "react";
import {
  fetchStaffOrders,
  sendInvoice,
  sendOrderEvent,
  type CustomerRecord,
  type Order,
  type OrderEvent,
  type StaffOrderResult,
} from "../../lib/api";
import StaffOrderModal from "./StaffOrderModal";
import "./Orders.css";

function formatDate(iso: string): string {
  if (!iso) return "—";
  return new Date(iso.replace(" ", "T")).toLocaleDateString();
}

function formatMoney(cents?: number, currency = "USD"): string {
  if (cents == null) return "—";
  return new Intl.NumberFormat("en-US", { style: "currency", currency }).format(cents / 100);
}

function availableActions(status: string): { event: OrderEvent; label: string; danger?: boolean }[] {
  switch (status) {
    case "pending":
      return [
        { event: "staff_confirm", label: "Confirm" },
        { event: "staff_cancel", label: "Cancel", danger: true },
      ];
    case "confirmed":
      return [
        { event: "staff_mark_delivered", label: "Mark delivered" },
        { event: "staff_cancel", label: "Cancel", danger: true },
      ];
    case "delivered":
    case "needs_review":
      return [{ event: "staff_cancel", label: "Cancel", danger: true }];
    default:
      return [];
  }
}

function OrderDrawer({
  order,
  onClose,
  onOrderUpdate,
  onEdit,
}: {
  order: Order;
  onClose: () => void;
  onOrderUpdate: (updated: Order) => void;
  onEdit: () => void;
}) {
  const [savingEvent, setSavingEvent] = useState<OrderEvent | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [invoicing, setInvoicing] = useState(false);
  const customer = order.expand?.customer;
  const actions = availableActions(order.status);

  async function handleOrderEvent(event: OrderEvent) {
    setSavingEvent(event);
    setSaveError(null);
    try {
      const updated = await sendOrderEvent(order.id, event);
      onOrderUpdate(updated);
    } catch (e: unknown) {
      setSaveError(e instanceof Error ? e.message : "Failed to update status");
    } finally {
      setSavingEvent(null);
    }
  }

  async function handleSendInvoice() {
    setInvoicing(true);
    setSaveError(null);
    try {
      const result = await sendInvoice(order.id);
      onOrderUpdate(result.order);
    } catch (e: unknown) {
      setSaveError(e instanceof Error ? e.message : "Failed to send invoice");
    } finally {
      setInvoicing(false);
    }
  }

  const canSendInvoice =
    !order.squareInvoiceId &&
    !["paid", "cancelled", "needs_review"].includes(order.status);
  const canEdit =
    ["pending", "confirmed"].includes(order.status) &&
    !order.squareOrderId &&
    !order.squareInvoiceId;

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
              <div className="meta-row">
                <span className="meta-label">Placed by</span>
                <span>
                  {order.placedBy?.name || order.submittedBy?.name || "Unknown"}
                  {order.placedBy?.type === "staff" && " (staff)"}
                  {order.placementReason && <><br /><span className="meta-sub">{order.placementReason}</span></>}
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
                  <span className={`status-badge status-${order.status}`}>
                    {order.status.replace("_", " ")}
                  </span>
                  {saveError && (
                    <span className="staff-error">{saveError}</span>
                  )}
                </span>
              </div>
            </div>
          </section>

          {/* Fulfillment */}
          <section className="drawer-section">
            <h4 className="drawer-section-title">Fulfillment</h4>
            {order.fulfillment?.method === "delivery" ? (
              <div className="fulfillment-detail">
                <strong>Delivery</strong>
                <span>{order.fulfillment.recipient_name}</span>
                <span>{order.fulfillment.recipient_phone}</span>
                <address>
                  {order.fulfillment.address_line_1}<br />
                  {order.fulfillment.address_line_2 && <>{order.fulfillment.address_line_2}<br /></>}
                  {order.fulfillment.city}, {order.fulfillment.state} {order.fulfillment.postal_code}
                </address>
                <span>
                  {(order.fulfillment.distance_miles ?? 0).toFixed(1)} driving miles · {order.fulfillment.fee_cents
                    ? `${formatMoney(order.fulfillment.fee_cents, order.fulfillment.currency ?? "USD")} delivery fee`
                    : "Free delivery"}
                </span>
                {order.fulfillment.instructions && (
                  <p><strong>Instructions:</strong> {order.fulfillment.instructions}</p>
                )}
              </div>
            ) : (
              <p className="drawer-notes">Pickup</p>
            )}
          </section>

          {/* Line Items */}
          <section className="drawer-section">
            <h4 className="drawer-section-title">Line Items</h4>
            <table className="drawer-items-table">
              <thead>
                <tr>
                  <th>Item</th>
                  <th>Qty</th>
                  <th>Unit Price</th>
                  <th>Note</th>
                </tr>
              </thead>
              <tbody>
                {order.lineItems.map((li, i) => (
                  <tr key={i}>
                    <td data-label="Item">
                      {li.name || li.variation_id}
                      {li.grind && ` · ${li.grind}`}
                    </td>
                    <td data-label="Quantity">{li.quantity}</td>
                    <td data-label="Unit Price">{formatMoney(li.unit_price_cents, li.currency)}</td>
                    <td data-label="Note">{li.note || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>

          {order.staffEditHistory && order.staffEditHistory.length > 0 && (
            <section className="drawer-section">
              <h4 className="drawer-section-title">Edit history</h4>
              {order.staffEditHistory.map((edit, index) => (
                <p className="drawer-notes" key={`${edit.edited_at}-${index}`}>
                  <strong>{edit.actor.name}</strong> · {formatDate(edit.edited_at)}<br />
                  {edit.reason}
                </p>
              ))}
            </section>
          )}

          {/* Notes */}
          {order.notes && (
            <section className="drawer-section">
              <h4 className="drawer-section-title">Order Notes</h4>
              <p className="drawer-notes">{order.notes}</p>
            </section>
          )}

          {actions.length > 0 && (
            <section className="drawer-section">
              <h4 className="drawer-section-title">Actions</h4>
              <div className="workflow-actions">
                {canEdit && (
                  <button className="btn-action" onClick={onEdit} disabled={savingEvent !== null}>
                    Edit order
                  </button>
                )}
                {actions.map((action) => (
                  <button
                    key={action.event}
                    className={action.danger ? "btn-action btn-action-danger" : "btn-action"}
                    onClick={() => handleOrderEvent(action.event)}
                    disabled={savingEvent !== null}
                  >
                    {savingEvent === action.event ? "Saving…" : action.label}
                  </button>
                ))}
              </div>
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
                {order.status === "paid"
                  ? "Order is already paid."
                  : order.status === "cancelled"
                    ? "Cannot invoice a cancelled order."
                    : order.status === "needs_review"
                      ? "Review this order before invoicing."
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
  const [editing, setEditing] = useState<Order | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  useEffect(() => {
    fetchStaffOrders()
      .then(setOrders)
      .catch((e: unknown) =>
        setError(e instanceof Error ? e.message : "Failed to load orders")
      )
      .finally(() => setLoading(false));
  }, []);

  function handleOrderUpdate(updated: Order) {
    setOrders((prev) => prev.map((o) => (o.id === updated.id ? updated : o)));
    setSelected(updated);
  }

  function handleStaffEdit(result: StaffOrderResult) {
    handleOrderUpdate(result.order);
    setSuccess(result.notification_sent
      ? "Order updated and customer notified."
      : "Order updated, but customer notification email failed.");
    window.setTimeout(() => setSuccess(null), 6000);
  }

  if (loading) return <p className="muted">Loading orders…</p>;
  if (error) return <p className="staff-error">{error}</p>;

  return (
    <div className="staff-view">
      <div className="staff-page-heading">
        <div>
          <p className="eyebrow">Wholesale operations</p>
          <h1>Orders.</h1>
          <p className="staff-page-lead">
            Review incoming orders, update fulfillment status, and send invoices.
          </p>
        </div>
      </div>
      {success && <p className="staff-success">{success}</p>}
      {orders.length === 0 ? (
        <div className="staff-empty">No orders yet.</div>
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
                  <td className="mono" data-label="Order #">{o.id.slice(0, 8)}</td>
                  <td data-label="Date">{formatDate(o.created)}</td>
                  <td data-label="Customer">
                    {customer ? (
                      customer.name
                    ) : (
                      <span className="mono">{o.customer.slice(0, 8)}</span>
                    )}
                  </td>
                  <td data-label="Items">{o.lineItems.length} item(s)</td>
                  <td className="mono" data-label="Square Order">
                    {o.squareOrderId
                      ? o.squareOrderId.slice(0, 10) + "…"
                      : "—"}
                  </td>
                  <td data-label="Status">
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
          onOrderUpdate={handleOrderUpdate}
          onEdit={() => setEditing(selected)}
        />
      )}

      {editing?.expand?.customer && (
        <StaffOrderModal
          customer={{
            id: editing.expand.customer.id,
            name: editing.expand.customer.name,
            email: editing.expand.customer.email,
            phone: editing.expand.customer.phone,
            squareCustomerId: "",
            company: editing.company,
            created: "",
          } satisfies CustomerRecord}
          order={editing}
          onClose={() => setEditing(null)}
          onUpdated={handleStaffEdit}
        />
      )}
    </div>
  );
}
