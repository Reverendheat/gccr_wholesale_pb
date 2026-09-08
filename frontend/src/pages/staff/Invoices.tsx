import { useEffect, useState } from "react";
import { fetchStaffOrders, type Order } from "../../lib/api";
import "./Orders.css";

function formatDate(iso: string): string {
  if (!iso) return "—";
  return new Date(iso.replace(" ", "T")).toLocaleDateString();
}

function InvoiceDeliverySummary({ order }: { order: Order }) {
  const recipients = order.invoiceRecipients ?? [];
  if (recipients.length === 0) {
    return (
      <div className="invoice-delivery-summary">
        <strong>Legacy · Square delivery</strong>
        <span className="meta-sub">Original Square-email invoice; no portal recipient snapshot or retry.</span>
      </div>
    );
  }
  const sent = new Set((order.invoiceSentRecipients ?? []).map((email) => email.toLowerCase()));
  const pending = recipients.filter((email) => !sent.has(email.toLowerCase()));
  return (
    <div className="invoice-delivery-summary">
      <strong className={pending.length === 0 ? "invoice-sent-label" : undefined}>
        {pending.length === 0 ? "Sent · Portal email" : order.squareInvoiceUrl ? "Pending · Portal email" : "Invoice creation pending"}
      </strong>
      {recipients.map((email) => (
        <span key={email}>{email} — {sent.has(email.toLowerCase()) ? "Sent" : "Pending"}</span>
      ))}
      {pending.length === 0 && order.invoiceEmailSentAt && (
        <span className="meta-sub">Sent {new Date(order.invoiceEmailSentAt.replace(" ", "T")).toLocaleString()}</span>
      )}
      {order.invoiceEmailError && <span className="staff-error">{order.invoiceEmailError}</span>}
      {pending.length > 0 && (
        <span className="meta-sub">Open the order to review and {order.squareInvoiceUrl ? "retry unsent emails" : "resume the invoice"}.</span>
      )}
    </div>
  );
}

export default function Invoices() {
  const [invoiced, setInvoiced] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchStaffOrders()
      .then((orders) =>
        setInvoiced(orders.filter((o) => o.squareInvoiceId || o.invoiceRecipients?.length))
      )
      .catch((e: unknown) =>
        setError(e instanceof Error ? e.message : "Failed to load invoices")
      )
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <p className="muted">Loading invoices…</p>;
  if (error) return <p className="staff-error">{error}</p>;

  return (
    <div className="staff-view">
      <div className="staff-page-heading">
        <div>
          <p className="eyebrow">Wholesale operations</p>
          <h1>Invoices.</h1>
          <p className="staff-page-lead">
            Open Square invoices and track invoice email delivery.
          </p>
        </div>
      </div>
      {invoiced.length === 0 ? (
        <div className="staff-empty">No invoices created yet.</div>
      ) : (
        <table className="orders-table">
          <thead>
            <tr>
              <th>Square Invoice ID</th>
              <th>Order #</th>
              <th>Date</th>
              <th>Customer</th>
              <th>Status</th>
              <th>Email delivery</th>
              <th>Invoice</th>
            </tr>
          </thead>
          <tbody>
            {invoiced.map((o) => (
              <tr key={o.id}>
                <td className="mono" data-label="Square Invoice">{o.squareInvoiceId || "Not created yet"}</td>
                <td className="mono" data-label="Order #">{o.id.slice(0, 8)}</td>
                <td data-label="Date">{formatDate(o.created)}</td>
                <td data-label="Customer">{o.expand?.customer?.name ?? <span className="mono">{o.customer.slice(0, 8)}</span>}</td>
                <td data-label="Status">
                  <span className={`status-badge status-${o.status}`}>
                    {o.status}
                  </span>
                </td>
                <td data-label="Email delivery"><InvoiceDeliverySummary order={o} /></td>
                <td data-label="Invoice">
                  {o.squareInvoiceUrl ? (
                    <a
                      href={o.squareInvoiceUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="invoice-link"
                    >
                      View invoice
                    </a>
                  ) : (
                    <span className="muted">—</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
