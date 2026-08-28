import { useEffect, useState } from "react";
import { fetchStaffOrders, type Order } from "../../lib/api";
import "./Orders.css";

function formatDate(iso: string): string {
  if (!iso) return "—";
  return new Date(iso.replace(" ", "T")).toLocaleDateString();
}

export default function Invoices() {
  const [invoiced, setInvoiced] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchStaffOrders()
      .then((orders) =>
        setInvoiced(orders.filter((o) => o.squareInvoiceId))
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
            Open Square invoices and track the orders already sent for payment.
          </p>
        </div>
      </div>
      {invoiced.length === 0 ? (
        <div className="staff-empty">No invoices sent yet.</div>
      ) : (
        <table className="orders-table">
          <thead>
            <tr>
              <th>Square Invoice ID</th>
              <th>Order #</th>
              <th>Date</th>
              <th>Customer</th>
              <th>Status</th>
              <th>Invoice</th>
            </tr>
          </thead>
          <tbody>
            {invoiced.map((o) => (
              <tr key={o.id}>
                <td className="mono" data-label="Square Invoice">{o.squareInvoiceId}</td>
                <td className="mono" data-label="Order #">{o.id.slice(0, 8)}</td>
                <td data-label="Date">{formatDate(o.created)}</td>
                <td data-label="Customer">{o.expand?.customer?.name ?? <span className="mono">{o.customer.slice(0, 8)}</span>}</td>
                <td data-label="Status">
                  <span className={`status-badge status-${o.status}`}>
                    {o.status}
                  </span>
                </td>
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
