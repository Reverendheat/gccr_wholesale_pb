import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import "./Login.css";

type Tab = "staff" | "customer";
type LoginStep = "email" | "code";

export default function Login() {
  const {
    requestStaffOTP,
    requestCustomerOTP,
    loginAsStaff,
    loginAsCustomer,
    role,
  } = useAuth();
  const navigate = useNavigate();

  const [tab, setTab] = useState<Tab>("staff");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [otpId, setOtpId] = useState("");
  const [step, setStep] = useState<LoginStep>("email");
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
      if (step === "email") {
        const id = tab === "staff"
          ? await requestStaffOTP(email)
          : await requestCustomerOTP(email);
        setOtpId(id);
        setCode("");
        setStep("code");
      } else {
        if (tab === "staff") {
          await loginAsStaff(otpId, code);
          navigate("/staff");
        } else {
          await loginAsCustomer(otpId, code);
          navigate("/portal");
        }
      }
    } catch {
      setError(step === "email" ? "Could not send a sign-in code." : "Invalid or expired code.");
    } finally {
      setLoading(false);
    }
  }

  function resetFlow(nextTab = tab) {
    setTab(nextTab);
    setCode("");
    setOtpId("");
    setStep("email");
    setError(null);
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
            onClick={() => resetFlow("staff")}
          >
            Staff
          </button>
          <button
            role="tab"
            aria-selected={tab === "customer"}
            className={tab === "customer" ? "active" : ""}
            onClick={() => resetFlow("customer")}
          >
            Customer
          </button>
        </div>

        <form onSubmit={handleSubmit} className="login-form">
          {step === "code" && (
            <p className="login-help">
              Enter the code sent to {email}.
            </p>
          )}
          <label>
            Email
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
              autoFocus
              disabled={step === "code"}
            />
          </label>
          {step === "code" && (
            <label>
              Sign-in code
              <input
                type="text"
                inputMode="numeric"
                value={code}
                onChange={(e) => setCode(e.target.value.trim())}
                required
                autoComplete="one-time-code"
                autoFocus
              />
            </label>
          )}
          {error && <p className="login-error" role="alert">{error}</p>}
          <button type="submit" disabled={loading} className="login-submit">
            {loading
              ? step === "email" ? "Sending code…" : "Signing in…"
              : step === "email" ? "Send sign-in code" : "Sign in"}
          </button>
          {step === "code" && (
            <button type="button" className="login-link" onClick={() => resetFlow()}>
              Use a different email
            </button>
          )}
        </form>
      </div>
    </div>
  );
}
