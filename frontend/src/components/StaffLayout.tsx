import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import "./StaffLayout.css";

export default function StaffLayout() {
  const { user, logout } = useAuth();

  return (
    <div className="staff-shell">
      <aside className="staff-sidebar">
        <div className="sidebar-logo">
          <img src="/logo.png" alt="Logo" className="sidebar-logo-img" />
        </div>
        <nav className="sidebar-nav">
          <NavLink to="/staff/orders">Orders</NavLink>
          <NavLink to="/staff/invoices">Invoices</NavLink>
          <NavLink to="/staff/customers">Customers</NavLink>
        </nav>
        <div className="sidebar-footer">
          <span className="sidebar-user">{user?.email}</span>
          <button onClick={logout} className="sidebar-logout">Sign out</button>
        </div>
      </aside>
      <main className="staff-main">
        <Outlet />
      </main>
    </div>
  );
}
