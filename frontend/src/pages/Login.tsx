import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import "./Login.css";

type Tab = "staff" | "customer";

export default function Login() {
  const { loginAsStaff, loginAsCustomer, role } = useAuth();
  const navigate = useNavigate();

  const [tab, setTab] = useState<Tab>("staff");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Already logged in — redirect away
  if (role === "staff") { navigate("/staff", { replace: true }); return null; }
  if (role === "customer") { navigate("/portal", { replace: true }); return null; }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      if (tab === "staff") {
        await loginAsStaff(email, password);
        navigate("/staff");
      } else {
        await loginAsCustomer(email, password);
        navigate("/portal");
      }
    } catch {
      setError("Invalid email or password.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="login-logo">
          <img src="/logo.png" alt="Ground Control" />
          <span>Wholesale</span>
        </div>

        <div className="login-tabs" role="tablist">
          <button
            role="tab"
            aria-selected={tab === "staff"}
            className={tab === "staff" ? "active" : ""}
            onClick={() => { setTab("staff"); setError(null); }}
          >
            Staff
          </button>
          <button
            role="tab"
            aria-selected={tab === "customer"}
            className={tab === "customer" ? "active" : ""}
            onClick={() => { setTab("customer"); setError(null); }}
          >
            Customer
          </button>
        </div>

        <form onSubmit={handleSubmit} className="login-form">
          <label>
            Email
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
              autoFocus
            />
          </label>
          <label>
            Password
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="current-password"
            />
          </label>
          {error && <p className="login-error" role="alert">{error}</p>}
          <button type="submit" disabled={loading} className="login-submit">
            {loading ? "Signing in…" : "Sign in"}
          </button>
        </form>
      </div>
    </div>
  );
}
