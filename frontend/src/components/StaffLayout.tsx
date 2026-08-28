import { useEffect, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import "./StaffLayout.css";

const STAFF_TITLES: Record<string, string> = {
  "/staff/orders": "Orders",
  "/staff/invoices": "Invoices",
  "/staff/customers": "Customers",
};

export default function StaffLayout() {
  const { user, logout } = useAuth();
  const location = useLocation();
  const [navigationOpen, setNavigationOpen] = useState(false);
  const pageTitle = STAFF_TITLES[location.pathname] ?? "Wholesale";

  useEffect(() => {
    setNavigationOpen(false);
  }, [location.pathname]);

  return (
    <div className="staff-shell">
      <aside className={`staff-sidebar${navigationOpen ? " open" : ""}`}>
        <div className="sidebar-brand">
          <img src="/logo.png" alt="" className="sidebar-brand-logo" />
          <span className="sidebar-brand-copy">
            <strong>Ground Control</strong>
            <span>Wholesale staff</span>
          </span>
        </div>

        <nav className="sidebar-nav" aria-label="Staff navigation">
          <p className="sidebar-nav-label">Workspace</p>
          <NavLink to="/staff/orders">Orders</NavLink>
          <NavLink to="/staff/invoices">Invoices</NavLink>
          <NavLink to="/staff/customers">Customers</NavLink>
        </nav>

        <div className="sidebar-footer">
          <strong>Staff account</strong>
          <span className="sidebar-user">{user?.email}</span>
          <button type="button" onClick={logout} className="sidebar-logout">
            Sign out
          </button>
        </div>
      </aside>

      <div
        className={`staff-nav-backdrop${navigationOpen ? " open" : ""}`}
        onClick={() => setNavigationOpen(false)}
        aria-hidden="true"
      />

      <main className="staff-main">
        <header className="staff-topbar">
          <div className="staff-topbar-title">
            <button
              type="button"
              className="staff-mobile-menu"
              aria-label={navigationOpen ? "Close navigation" : "Open navigation"}
              aria-expanded={navigationOpen}
              onClick={() => setNavigationOpen((open) => !open)}
            >
              <svg viewBox="0 0 20 20" aria-hidden="true">
                <path d="M3 5h14M3 10h14M3 15h14" />
              </svg>
            </button>
            <h2>{pageTitle}</h2>
          </div>
          <span className="staff-environment">Wholesale operations</span>
        </header>
        <div className="staff-content">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
