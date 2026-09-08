import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  assignCustomerAccount,
  fetchCompanies,
  fetchCustomerAudienceAccess,
  fetchCustomers,
  inviteCustomer,
  previewCustomer,
  updateCompanyBilling,
  updateCustomerAudienceAccess,
  type CompanyRecord,
  type CustomerAudienceAccess,
  type CustomerRecord,
  type SquareCustomerPreview,
  type StaffOrderResult,
} from "../../lib/api";
import StaffOrderModal from "./StaffOrderModal";
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
        const suggestedAccounts = result.suggested_accounts ?? [];
        setPreview(result.customer);
        if (suggestedAccounts.length === 1) {
          setCompanyId(suggestedAccounts[0].id);
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

function AudienceAccessModal({
  customer,
  onClose,
  onSaved,
}: {
  customer: CustomerRecord;
  onClose: () => void;
  onSaved: (access: CustomerAudienceAccess) => void;
}) {
  const [access, setAccess] = useState<CustomerAudienceAccess | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchCustomerAudienceAccess(customer.id)
      .then(setAccess)
      .catch((cause: unknown) => {
        setError(cause instanceof Error ? cause.message : "Could not load catalog access");
      })
      .finally(() => setLoading(false));
  }, [customer.id]);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!access) return;
    setSaving(true);
    setError(null);
    try {
      onSaved(await updateCustomerAudienceAccess(customer.id, access));
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : "Could not update catalog access");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(event) => event.stopPropagation()}>
        <h3>Catalog access</h3>
        <p className="muted modal-desc">
          Enable the Square wholesale groups for {customer.name || customer.email}.
        </p>
        {loading ? <p className="muted">Loading access…</p> : (
          <form onSubmit={handleSubmit} className="invite-form audience-access-form">
            <div className="audience-options">
              <label className="audience-option">
                <input
                  type="checkbox"
                  checked={access?.grocery ?? false}
                  onChange={(event) => setAccess((current) => current
                    ? { ...current, grocery: event.target.checked }
                    : current)}
                />
                <span>
                  Grocery
                  <small>Seven case-pack coffee offerings</small>
                </span>
              </label>
              <label className="audience-option">
                <input
                  type="checkbox"
                  checked={access?.cafe_restaurant ?? false}
                  onChange={(event) => setAccess((current) => current
                    ? { ...current, cafe_restaurant: event.target.checked }
                    : current)}
                />
                <span>
                  Cafe / Restaurant
                  <small>Nine bulk coffee and cold brew offerings</small>
                </span>
              </label>
            </div>
            <p className="muted modal-desc">
              Customers with neither group see no standard catalog items.
            </p>
            {error && <p className="staff-error modal-error">{error}</p>}
            <div className="modal-actions">
              <button type="button" className="btn-secondary" onClick={onClose}>Cancel</button>
              <button type="submit" disabled={saving || !access}>
                {saving ? "Saving…" : "Save access"}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}

function BillingModal({
  company,
  onClose,
  onSaved,
}: {
  company: CompanyRecord;
  onClose: () => void;
  onSaved: (company: CompanyRecord) => void;
}) {
  const [emails, setEmails] = useState<string[]>(company.billingEmails ?? []);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const cardRef = useRef<HTMLDivElement>(null);
  const nextFocus = useRef<number | null>(null);

  useEffect(() => {
    const trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    cardRef.current?.querySelector<HTMLElement>("input, button")?.focus();
    return () => {
      document.body.style.overflow = previousOverflow;
      trigger?.focus();
    };
  }, []);

  useEffect(() => {
    if (nextFocus.current === null) return;
    const inputs = cardRef.current?.querySelectorAll<HTMLInputElement>("input");
    const target = inputs?.[Math.min(nextFocus.current, inputs.length - 1)];
    (target ?? cardRef.current?.querySelector<HTMLButtonElement>("[data-add-email]"))?.focus();
    nextFocus.current = null;
  }, [emails.length]);

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    const normalized = emails.map((email) => email.trim()).filter(Boolean);
    if (normalized.some((email) => !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email))) {
      setError("Enter a valid email address in each row, or remove it.");
      return;
    }
    const seen = new Set<string>();
    const recipients = normalized.filter((email) => {
      const key = email.toLowerCase();
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
    cardRef.current?.focus();
    setSaving(true);
    setError(null);
    try {
      onSaved(await updateCompanyBilling(company.id, recipients));
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : "Could not update billing emails");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={() => { if (!saving) onClose(); }}>
      <div
        ref={cardRef}
        className="modal-card billing-modal"
        role="dialog"
        tabIndex={-1}
        aria-modal="true"
        aria-labelledby="billing-title"
        aria-describedby="billing-description"
        onClick={(event) => event.stopPropagation()}
        onKeyDown={(event) => {
          if (event.key === "Escape" && !saving) {
            event.stopPropagation();
            onClose();
          }
          if (event.key !== "Tab") return;
          const focusable = cardRef.current?.querySelectorAll<HTMLElement>("input:not(:disabled), button:not(:disabled)");
          if (!focusable?.length) {
            event.preventDefault();
            return;
          }
          const first = focusable[0];
          const last = focusable[focusable.length - 1];
          if (document.activeElement === cardRef.current) {
            event.preventDefault();
            (event.shiftKey ? last : first).focus();
          } else if (event.shiftKey && document.activeElement === first) {
            event.preventDefault();
            last.focus();
          } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first.focus();
          }
        }}
      >
        <h3 id="billing-title">Manage billing · {company.name}</h3>
        <p id="billing-description" className="muted modal-desc">
          Invoices go only to these addresses. Leave blank to use the buyer’s account email.
          Changes apply to new invoices for this company.
        </p>
        <form onSubmit={handleSubmit} className="invite-form" noValidate aria-busy={saving}>
          {emails.map((email, index) => (
            <div className="billing-email-row" key={index}>
              <label>
                Billing email {index + 1}
                <input
                  type="email"
                  autoComplete="off"
                  value={email}
                  disabled={saving}
                  onChange={(event) => setEmails((current) => current.map((value, i) => i === index ? event.target.value : value))}
                />
              </label>
              <button
                type="button"
                className="btn-secondary"
                aria-label={`Remove billing email ${index + 1}`}
                disabled={saving}
                onClick={() => {
                  nextFocus.current = index;
                  setEmails((current) => current.filter((_, i) => i !== index));
                }}
              >
                Remove
              </button>
            </div>
          ))}
          <button
            type="button"
            className="btn-secondary"
            data-add-email
            disabled={saving}
            onClick={() => {
              nextFocus.current = emails.length;
              setEmails((current) => [...current, ""]);
            }}
          >
            Add billing email
          </button>
          {error && <p className="staff-error modal-error" role="alert">{error}</p>}
          <div className="modal-actions">
            <button type="button" className="btn-secondary" onClick={onClose} disabled={saving}>Cancel</button>
            <button type="submit" disabled={saving}>{saving ? "Saving…" : "Save billing emails"}</button>
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
  const [orderingFor, setOrderingFor] = useState<CustomerRecord | null>(null);
  const [audienceFor, setAudienceFor] = useState<CustomerRecord | null>(null);
  const [billingFor, setBillingFor] = useState<CompanyRecord | null>(null);

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
    setAudienceFor(customer);
    setSuccessMsg(`Invite sent to ${customer.email}. They can sign in using a one-time code.`);
    setTimeout(() => setSuccessMsg(null), 6000);
  }

  function handleOrderCreated(result: StaffOrderResult) {
    const customerName = orderingFor?.name || orderingFor?.email || "customer";
    setSuccessMsg(result.notification_sent
      ? `Order created for ${customerName}; confirmation email sent.`
      : `Order created for ${customerName}, but confirmation email could not be sent.`);
    setTimeout(() => setSuccessMsg(null), 7000);
  }

  function handleAudienceSaved(access: CustomerAudienceAccess) {
    const customerName = audienceFor?.name || audienceFor?.email || "customer";
    const enabled = [
      access.grocery ? "Grocery" : "",
      access.cafe_restaurant ? "Cafe / Restaurant" : "",
    ].filter(Boolean);
    setSuccessMsg(enabled.length > 0
      ? `${customerName} catalog access: ${enabled.join(" and ")}.`
      : `${customerName} has no standard catalog access.`);
    setAudienceFor(null);
    setTimeout(() => setSuccessMsg(null), 6000);
  }

  function handleBillingSaved(company: CompanyRecord) {
    setCompanies((current) => current.map((item) => item.id === company.id ? company : item));
    setCustomers((current) => current.map((customer) => customer.company === company.id
      ? { ...customer, expand: { ...customer.expand, company } }
      : customer));
    setBillingFor(null);
    setSuccessMsg(`Billing emails updated for ${company.name}. Existing invoice recipients are unchanged.`);
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

      const suggestedAccounts = result.suggested_accounts ?? [];
      if (suggestedAccounts.length === 1) {
        const account = suggestedAccounts[0];
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
    <div className="staff-view">
      <div className="staff-page-heading">
        <div>
          <p className="eyebrow">Account directory</p>
          <h1>Customers.</h1>
          <p className="staff-page-lead">
            Invite buyers, manage wholesale accounts, and control catalog access.
          </p>
        </div>
        <button type="button" onClick={() => setShowInvite(true)}>Invite customer</button>
      </div>

      {error && <p className="staff-error">{error}</p>}
      {successMsg && <p className="staff-success">{successMsg}</p>}

      {customers.length === 0 ? (
        <div className="staff-empty">No customers yet. Invite the first wholesale buyer.</div>
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
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {customers.map((customer) => (
              <tr key={customer.id}>
                <td data-label="Name">{customer.name || "—"}</td>
                <td data-label="Email">{customer.email}</td>
                <td className="account-cell" data-label="Wholesale Account">
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
                  {customer.company && (
                    <button
                      type="button"
                      className="account-reconcile"
                      disabled={assigning === customer.id || !companies.some((company) => company.id === customer.company)}
                      onClick={() => setBillingFor(companies.find((company) => company.id === customer.company) ?? null)}
                      aria-label={`Manage billing for ${companies.find((company) => company.id === customer.company)?.name || "wholesale account"}`}
                    >
                      Manage billing
                    </button>
                  )}
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
                <td data-label="Phone">{customer.phone || "—"}</td>
                <td className="mono" data-label="Square ID">
                  {customer.squareCustomerId ? `${customer.squareCustomerId.slice(0, 12)}…` : "—"}
                </td>
                <td data-label="Registered">{formatDate(customer.created)}</td>
                <td className="customer-actions" data-label="Actions">
                  <div className="customer-action-buttons">
                    <button type="button" onClick={() => setOrderingFor(customer)} disabled={!customer.squareCustomerId}>
                      Create order
                    </button>
                    <button type="button" onClick={() => setAudienceFor(customer)} disabled={!customer.squareCustomerId}>
                      Catalog access
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {billingFor && (
        <BillingModal
          company={billingFor}
          onClose={() => setBillingFor(null)}
          onSaved={handleBillingSaved}
        />
      )}

      {orderingFor && (
        <StaffOrderModal
          customer={orderingFor}
          onClose={() => setOrderingFor(null)}
          onCreated={handleOrderCreated}
        />
      )}

      {audienceFor && (
        <AudienceAccessModal
          customer={audienceFor}
          onClose={() => setAudienceFor(null)}
          onSaved={handleAudienceSaved}
        />
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
