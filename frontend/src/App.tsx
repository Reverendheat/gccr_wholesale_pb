import { Navigate, Route, BrowserRouter as Router, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./context/AuthContext";
import type { UserRole } from "./context/AuthContext";
import Login from "./pages/Login";
import StaffLayout from "./components/StaffLayout";
import Orders from "./pages/staff/Orders";
import Invoices from "./pages/staff/Invoices";
import Customers from "./pages/staff/Customers";
import CustomerPortal from "./pages/CustomerPortal";
import InstallPrompt from "./components/InstallPrompt";

// Redirects to /login if not authenticated, or wrong role
function RequireRole({ role, children }: { role: UserRole; children: React.ReactNode }) {
  const { user, role: userRole, isLoading } = useAuth();

  if (isLoading) return null;
  if (!user) return <Navigate to="/login" replace />;
  if (userRole !== role) return <Navigate to="/login" replace />;

  return <>{children}</>;
}

function AppRoutes() {
  const { user, role, isLoading } = useAuth();

  if (isLoading) return null;

  return (
    <Routes>
      <Route
        path="/login"
        element={
          // Already logged in — send to the right place
          user && role === "staff" ? <Navigate to="/staff/orders" replace /> :
          user && role === "customer" ? <Navigate to="/portal" replace /> :
          <Login />
        }
      />

      {/* Staff routes */}
      <Route
        path="/staff"
        element={
          <RequireRole role="staff">
            <StaffLayout />
          </RequireRole>
        }
      >
        <Route index element={<Navigate to="orders" replace />} />
        <Route path="orders" element={<Orders />} />
        <Route path="invoices" element={<Invoices />} />
        <Route path="customers" element={<Customers />} />
      </Route>

      {/* Customer routes */}
      <Route
        path="/portal"
        element={
          <RequireRole role="customer">
            <CustomerPortal />
          </RequireRole>
        }
      />

      {/* Default */}
      <Route path="*" element={<Navigate to="/login" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <Router>
      <AuthProvider>
        <AppRoutes />
        <InstallPrompt />
      </AuthProvider>
    </Router>
  );
}
