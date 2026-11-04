import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { fetchCompanies, type CompanyRecord, type OrderListFilters } from "../../lib/api";
import { defaultOrderFilters } from "../../lib/useStaffOrders";

const statuses = ["pending", "confirmed", "delivered", "invoiced", "paid", "cancelled", "needs_review"];
const sortLabels: Record<string, string> = {
  newest: "Newest first", oldest: "Oldest first", company: "Company A–Z",
  total_desc: "Total: high to low", total_asc: "Total: low to high",
};

export default function OrderListControls({ invoices = false, onApply }: {
  invoices?: boolean;
  onApply: (filters: OrderListFilters) => void;
}) {
  const [filters, setFilters] = useState({ ...defaultOrderFilters });
  const [expanded, setExpanded] = useState(false);
  const [companies, setCompanies] = useState<CompanyRecord[]>([]);
  const [error, setError] = useState<string | null>(null);
  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const panelId = useId();
  const quickStatuses = invoices
    ? [["", "All"], ["unpaid", "Unpaid"], ["paid", "Paid"], ["needs_review", "Needs review"]]
    : [["", "All"], ["pending", "Pending"], ["confirmed", "Confirmed"], ["paid", "Paid"]];
  const extraStatus = filters.status && !quickStatuses.some(([value]) => value === filters.status);
  const refinementCount = Number(!!filters.company) + Number(!!(filters.from || filters.to))
    + Number(filters.sort !== "newest") + Number(!!extraStatus);
  const hasFilters = filters.q || filters.status || refinementCount > 0;

  useEffect(() => {
    let active = true;
    fetchCompanies().then((result) => { if (active) setCompanies(result); })
      .catch(() => { if (active) setError("Company filter unavailable. Search by company name instead."); });
    return () => {
      active = false;
      if (searchTimer.current) clearTimeout(searchTimer.current);
    };
  }, []);

  function apply(next: OrderListFilters) {
    if (searchTimer.current) clearTimeout(searchTimer.current);
    setFilters(next);
    onApply(next);
  }

  function update(key: keyof OrderListFilters, value: string) {
    const next = { ...filters, [key]: value };
    if (key !== "q") { apply(next); return; }
    setFilters(next);
    if (searchTimer.current) clearTimeout(searchTimer.current);
    searchTimer.current = setTimeout(() => onApply(next), 300);
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    apply(filters);
  }

  return (
    <section className="order-list-controls" aria-label={invoices ? "Invoice filters" : "Order filters"}>
      <div className="order-search-row">
        <form className="order-search" role="search" onSubmit={submit}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5" /><path d="m16 16 4 4" /></svg>
          <input aria-label={invoices ? "Search invoices" : "Search orders"} type="search" maxLength={200} value={filters.q} onChange={(e) => update("q", e.target.value)} placeholder="Search company, customer, or ID…" />
        </form>
        <button type="button" className="order-filter-toggle" aria-expanded={expanded} aria-controls={panelId} onClick={() => setExpanded(!expanded)}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true"><path d="M4 7h9m4 0h3M4 17h3m4 0h9" /><circle cx="15" cy="7" r="2" /><circle cx="9" cy="17" r="2" /></svg>
          Filters{refinementCount > 0 && <span className="order-filter-count">{refinementCount}</span>}
        </button>
      </div>
      <div className="order-status-tabs" role="group" aria-label="Quick status filters">
        {quickStatuses.map(([value, label]) => (
          <button type="button" key={value} className="order-status-tab" aria-pressed={filters.status === value} onClick={() => update("status", value)}>{label}</button>
        ))}
      </div>
      {expanded && (
        <div className="order-filter-panel" id={panelId}>
          <label>Status<select value={filters.status} onChange={(e) => update("status", e.target.value)}>
            <option value="">All statuses</option>
            {invoices && <option value="unpaid">Unpaid (invoiced)</option>}
            {statuses.map((status) => <option key={status} value={status}>{status.replaceAll("_", " ")}</option>)}
          </select></label>
          <label>Company<select value={filters.company} onChange={(e) => update("company", e.target.value)}>
            <option value="">All companies</option>
            {companies.map((company) => <option key={company.id} value={company.id}>{company.name}</option>)}
          </select></label>
          <label>Sort by<select value={filters.sort} onChange={(e) => update("sort", e.target.value)}>
            {Object.entries(sortLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select></label>
          <div className="order-date-range">
            <label>From (UTC)<input type="date" value={filters.from} max={filters.to || undefined} onChange={(e) => {
              if (!e.target.value || !filters.to || e.target.value <= filters.to) update("from", e.target.value);
            }} /></label>
            <label>Through (UTC)<input type="date" value={filters.to} min={filters.from || undefined} onChange={(e) => {
              if (!e.target.value || !filters.from || e.target.value >= filters.from) update("to", e.target.value);
            }} /></label>
          </div>
          {error && <p className="staff-error" role="alert">{error}</p>}
        </div>
      )}
      {hasFilters && (
        <div className="order-active-filters" aria-label="Active filters">
          {extraStatus && <button type="button" onClick={() => update("status", "")}>{filters.status.replaceAll("_", " ")} <span aria-hidden="true">×</span><span className="order-sr-only">Remove status filter</span></button>}
          {filters.company && <button type="button" onClick={() => update("company", "")}>{companies.find((company) => company.id === filters.company)?.name || "Company"} <span aria-hidden="true">×</span><span className="order-sr-only">Remove company filter</span></button>}
          {(filters.from || filters.to) && <button type="button" onClick={() => apply({ ...filters, from: "", to: "" })}>{filters.from || "Any date"} – {filters.to || "No end date"} <span aria-hidden="true">×</span><span className="order-sr-only">Remove date filter</span></button>}
          {filters.sort !== "newest" && <button type="button" onClick={() => update("sort", "newest")}>{sortLabels[filters.sort]} <span aria-hidden="true">×</span><span className="order-sr-only">Reset sort</span></button>}
          <button type="button" className="order-clear-filters" onClick={() => apply({ ...defaultOrderFilters })}>Clear all</button>
        </div>
      )}
    </section>
  );
}

export function OrderPagination({ page, hasMore, loading, onPage }: {
  page: number; hasMore: boolean; loading: boolean; onPage: (page: number) => void;
}) {
  return <nav className="order-pagination" aria-label="Results pages">
    <button type="button" className="btn-secondary" disabled={loading || page <= 1} onClick={() => onPage(page - 1)}>Previous</button>
    <span aria-live="polite">Page {page}</span>
    <button type="button" className="btn-secondary" disabled={loading || !hasMore} onClick={() => onPage(page + 1)}>Next</button>
  </nav>;
}
