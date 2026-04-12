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
    <div>
      <h2>Invoices</h2>
      {invoiced.length === 0 ? (
        <p className="muted">No invoices sent yet.</p>
      ) : (
        <table className="orders-table">
          <thead>
            <tr>
              <th>Square Invoice ID</th>
              <th>Order #</th>
              <th>Date</th>
              <th>Customer</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {invoiced.map((o) => (
              <tr key={o.id}>
                <td className="mono">{o.squareInvoiceId}</td>
                <td className="mono">{o.id.slice(0, 8)}</td>
                <td>{formatDate(o.created)}</td>
                <td>{o.expand?.customer?.name ?? <span className="mono">{o.customer.slice(0, 8)}</span>}</td>
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
  );
}
