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

  const audienceCopy = tab === "staff"
    ? {
        eyebrow: "Staff access",
        title: "Staff sign in",
        lead: "Manage wholesale orders, customers, and invoices.",
      }
    : {
        eyebrow: "Customer access",
        title: "Customer sign in",
        lead: "Place orders and manage your wholesale coffee account.",
      };

  return (
    <main className="login-screen">
      <section className="login-context" aria-label="Ground Control Wholesale">
        <div className="login-brand">
          <img src="/logo.png" alt="" className="login-brand-logo" />
          <span>Ground Control <span>/ Wholesale</span></span>
        </div>

        <div className="login-context-copy">
          <p className="eyebrow">Ground Control Coffee</p>
          <h1>Wholesale<br />Coffee Portal</h1>
          <p className="login-context-lead">
            Place orders and manage your wholesale coffee account.
          </p>
        </div>

        <span className="login-location">Farmington, MI</span>
      </section>

      <section className="login-form-wrap">
        <div className="login-panel">
          <div className="login-tabs" role="tablist" aria-label="Sign-in audience">
            <button
              type="button"
              role="tab"
              aria-selected={tab === "staff"}
              className={tab === "staff" ? "active" : ""}
              onClick={() => resetFlow("staff")}
            >
              Staff
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={tab === "customer"}
              className={tab === "customer" ? "active" : ""}
              onClick={() => resetFlow("customer")}
            >
              Customer
            </button>
          </div>

          <p className="eyebrow">{audienceCopy.eyebrow}</p>
          <h2>{audienceCopy.title}</h2>
          <p className="login-form-lead">{audienceCopy.lead}</p>

          {error && <p className="login-error" role="alert">{error}</p>}

          <form onSubmit={handleSubmit} className="login-form">
            {step === "code" && (
              <p className="login-help">
                Enter the one-time code sent to <strong>{email}</strong>.
              </p>
            )}
            <label>
              Email address
              <input
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="name@company.com"
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
                  onChange={(event) => setCode(event.target.value.trim())}
                  placeholder="Enter your code"
                  required
                  autoComplete="one-time-code"
                  autoFocus
                />
              </label>
            )}
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
      </section>
    </main>
  );
}
