import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  assignCustomerAccount,
  fetchCompanies,
  fetchCustomers,
  inviteCustomer,
  previewCustomer,
  type CompanyRecord,
  type CustomerRecord,
  type SquareCustomerPreview,
} from "../../lib/api";
import "./Orders.css";

function formatDate(iso: string): string {
  if (!iso) return "—";
  return new Date(iso.replace(" ", "T")).toLocaleDateString();
}

function InviteModal({
  companies,
  onClose,
  onInvited,
}: {
  companies: CompanyRecord[];
  onClose: () => void;
  onInvited: (c: CustomerRecord) => void;
}) {
  const [email, setEmail] = useState("");
  const [preview, setPreview] = useState<SquareCustomerPreview | null>(null);
  const [companyId, setCompanyId] = useState("");
  const [newCompanyName, setNewCompanyName] = useState("");
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
      if (!preview) {
        const result = await previewCustomer(email.trim());
        setPreview(result.customer);
        if (result.suggested_accounts.length === 1) {
          setCompanyId(result.suggested_accounts[0].id);
        } else if (result.customer.company_name) {
          setNewCompanyName(result.customer.company_name);
        }
        return;
      }

      if (!companyId && !newCompanyName.trim()) {
        throw new Error("Select or create a wholesale account");
      }
      const result = await inviteCustomer(email.trim(), companyId
        ? { company_id: companyId }
        : { new_company_name: newCompanyName.trim() });
      onInvited({
        ...result,
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
          Match customer in Square, then confirm wholesale account membership.
        </p>
        <form onSubmit={handleSubmit} className="invite-form">
          <label>
            Email address
            <input
              ref={inputRef}
              type="email"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value);
                setPreview(null);
                setCompanyId("");
                setNewCompanyName("");
              }}
              placeholder="customer@example.com"
              required
              disabled={preview !== null}
              autoComplete="off"
            />
          </label>

          {preview && (
            <>
              <div className="modal-desc">
                <strong>{preview.name}</strong><br />
                <span className="muted">Square company: {preview.company_name || "Not set"}</span>
              </div>
              <label>
                Existing wholesale account
                <select
                  value={companyId}
                  onChange={(e) => {
                    setCompanyId(e.target.value);
                    if (e.target.value) setNewCompanyName("");
                  }}
                >
                  <option value="">Select account…</option>
                  {companies.map((company) => (
                    <option key={company.id} value={company.id}>{company.name}</option>
                  ))}
                </select>
              </label>
              <label>
                Or create wholesale account
                <input
                  value={newCompanyName}
                  onChange={(e) => {
                    setNewCompanyName(e.target.value);
                    if (e.target.value) setCompanyId("");
                  }}
                  placeholder="Business or account name"
                />
              </label>
            </>
          )}

          {error && <p className="staff-error modal-error">{error}</p>}
          <div className="modal-actions">
            <button type="button" className="btn-secondary" onClick={onClose}>Cancel</button>
            <button type="submit" disabled={submitting}>
              {submitting ? "Working…" : preview ? "Send invite" : "Look up in Square"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default function Customers() {
  const [customers, setCustomers] = useState<CustomerRecord[]>([]);
  const [companies, setCompanies] = useState<CompanyRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showInvite, setShowInvite] = useState(false);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [assigning, setAssigning] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([fetchCustomers(), fetchCompanies()])
      .then(([customerList, companyList]) => {
        setCustomers(customerList);
        setCompanies(companyList);
      })
      .catch((e: unknown) =>
        setError(e instanceof Error ? e.message : "Failed to load customers")
      )
      .finally(() => setLoading(false));
  }, []);

  function recordCompany(company?: CompanyRecord) {
    if (!company || companies.some((c) => c.id === company.id)) return;
    setCompanies((prev) => [...prev, company].sort((a, b) => a.name.localeCompare(b.name)));
  }

  function handleInvited(customer: CustomerRecord) {
    setCustomers((prev) => [customer, ...prev]);
    recordCompany(customer.expand?.company);
    setSuccessMsg(`Invite sent to ${customer.email}. They can sign in using a one-time code.`);
    setTimeout(() => setSuccessMsg(null), 6000);
  }

  async function saveAccountSelection(
    customer: CustomerRecord,
    selection: { company_id?: string; new_company_name?: string },
  ) {
    setAssigning(customer.id);
    setError(null);
    try {
      const updated = await assignCustomerAccount(customer.id, selection);
      setCustomers((prev) => prev.map((c) => c.id === updated.id ? updated : c));
      recordCompany(updated.expand?.company);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Could not assign account");
    } finally {
      setAssigning(null);
    }
  }

  async function handleAccountChange(customer: CustomerRecord, value: string) {
    if (value === "__new") {
      const name = window.prompt("New wholesale account name")?.trim();
      if (name) await saveAccountSelection(customer, { new_company_name: name });
    } else if (value) {
      await saveAccountSelection(customer, { company_id: value });
    }
  }

  async function handleSquareReconcile(customer: CustomerRecord) {
    setAssigning(customer.id);
    setError(null);
    try {
      const result = await previewCustomer(customer.email);
      const squareName = result.customer.company_name.trim();
      if (!squareName) throw new Error("Square customer has no company name");

      if (result.suggested_accounts.length === 1) {
        const account = result.suggested_accounts[0];
        if (!window.confirm(`Square company is “${squareName}”. Link to existing account “${account.name}”?`)) return;
        await saveAccountSelection(customer, { company_id: account.id });
      } else {
        if (!window.confirm(`Square company is “${squareName}”. Create this wholesale account?`)) return;
        await saveAccountSelection(customer, { new_company_name: squareName });
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Could not reconcile Square company");
    } finally {
      setAssigning(null);
    }
  }

  if (loading) return <p className="muted">Loading customers…</p>;

  return (
    <div>
      <div className="section-header">
        <h2>Customers</h2>
        <button onClick={() => setShowInvite(true)}>+ Invite Customer</button>
      </div>

      {error && <p className="staff-error">{error}</p>}
      {successMsg && <p className="staff-success">{successMsg}</p>}

      {customers.length === 0 ? (
        <p className="muted">No customers yet. Use button above to invite one.</p>
      ) : (
        <table className="orders-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Email</th>
              <th>Wholesale Account</th>
              <th>Phone</th>
              <th>Square ID</th>
              <th>Registered</th>
            </tr>
          </thead>
          <tbody>
            {customers.map((customer) => (
              <tr key={customer.id}>
                <td>{customer.name || "—"}</td>
                <td>{customer.email}</td>
                <td className="account-cell">
                  <select
                    value={customer.company || ""}
                    onChange={(e) => handleAccountChange(customer, e.target.value)}
                    disabled={assigning === customer.id}
                    aria-label={`Wholesale account for ${customer.name || customer.email}`}
                  >
                    <option value="">Unassigned</option>
                    {companies.map((company) => (
                      <option key={company.id} value={company.id}>{company.name}</option>
                    ))}
                    <option value="__new">+ Create new account…</option>
                  </select>
                  {!customer.company && (
                    <button
                      type="button"
                      className="account-reconcile"
                      onClick={() => handleSquareReconcile(customer)}
                      disabled={assigning === customer.id}
                    >
                      Use Square company
                    </button>
                  )}
                </td>
                <td>{customer.phone || "—"}</td>
                <td className="mono">
                  {customer.squareCustomerId ? `${customer.squareCustomerId.slice(0, 12)}…` : "—"}
                </td>
                <td>{formatDate(customer.created)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {showInvite && (
        <InviteModal
          companies={companies}
          onClose={() => setShowInvite(false)}
          onInvited={handleInvited}
        />
      )}
    </div>
  );
}
