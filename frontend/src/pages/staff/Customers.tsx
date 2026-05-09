import { useEffect, useRef, useState, type FormEvent } from "react";
import { fetchCustomers, inviteCustomer, type CustomerRecord } from "../../lib/api";
import "./Orders.css";

function formatDate(iso: string): string {
  if (!iso) return "—";
  return new Date(iso.replace(" ", "T")).toLocaleDateString();
}

function InviteModal({
  onClose,
  onInvited,
}: {
  onClose: () => void;
  onInvited: (c: CustomerRecord) => void;
}) {
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const result = await inviteCustomer(email.trim());
      onInvited({
        id: result.id,
        name: result.name,
        email: result.email,
        phone: "",
        squareCustomerId: "",
        created: new Date().toISOString(),
      });
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Invite failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <h3>Invite Customer</h3>
        <p className="muted modal-desc">
          Enter the customer's email address. We'll look them up in Square,
          create their account, and email them a link to set their password.
        </p>
        <form onSubmit={handleSubmit} className="invite-form">
          <label>
            Email address
            <input
              ref={inputRef}
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="customer@example.com"
              required
              autoComplete="off"
            />
          </label>
          {error && <p className="staff-error modal-error">{error}</p>}
          <div className="modal-actions">
            <button type="button" className="btn-secondary" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" disabled={submitting}>
              {submitting ? "Sending invite…" : "Send invite"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default function Customers() {
  const [customers, setCustomers] = useState<CustomerRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showInvite, setShowInvite] = useState(false);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  useEffect(() => {
    fetchCustomers()
      .then(setCustomers)
      .catch((e: unknown) =>
        setError(e instanceof Error ? e.message : "Failed to load customers")
      )
      .finally(() => setLoading(false));
  }, []);

  function handleInvited(c: CustomerRecord) {
    setCustomers((prev) => [c, ...prev]);
    setSuccessMsg(`Invite sent to ${c.email} — they'll receive a link to set their password.`);
    setTimeout(() => setSuccessMsg(null), 6000);
  }

  if (loading) return <p className="muted">Loading customers…</p>;
  if (error) return <p className="staff-error">{error}</p>;

  return (
    <div>
      <div className="section-header">
        <h2>Customers</h2>
        <button onClick={() => setShowInvite(true)}>+ Invite Customer</button>
      </div>

      {successMsg && <p className="staff-success">{successMsg}</p>}

      {customers.length === 0 ? (
        <p className="muted">No customers yet. Use the button above to invite one.</p>
      ) : (
        <table className="orders-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Email</th>
              <th>Phone</th>
              <th>Square ID</th>
              <th>Registered</th>
            </tr>
          </thead>
          <tbody>
            {customers.map((c) => (
              <tr key={c.id}>
                <td>{c.name || "—"}</td>
                <td>{c.email}</td>
                <td>{c.phone || "—"}</td>
                <td className="mono">
                  {c.squareCustomerId
                    ? c.squareCustomerId.slice(0, 12) + "…"
                    : "—"}
                </td>
                <td>{formatDate(c.created)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {showInvite && (
        <InviteModal
          onClose={() => setShowInvite(false)}
          onInvited={handleInvited}
        />
      )}
    </div>
  );
}
