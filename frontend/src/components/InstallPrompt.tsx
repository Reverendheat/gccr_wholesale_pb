import { useEffect, useState } from "react";
import "./InstallPrompt.css";

const DISMISSAL_KEY = "gccr-pwa-install-dismissed-at";
const DISMISSAL_DAYS = 30;

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed"; platform: string }>;
}

interface NavigatorWithPWAHints extends Navigator {
  standalone?: boolean;
  userAgentData?: { mobile?: boolean };
}

function isIOSDevice(): boolean {
  return /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
}

function isMobileDevice(): boolean {
  const mobileHint = (navigator as NavigatorWithPWAHints).userAgentData?.mobile;
  return Boolean(mobileHint) || /Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent) ||
    (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
}

function isStandalone(): boolean {
  return window.matchMedia("(display-mode: standalone)").matches ||
    Boolean((navigator as NavigatorWithPWAHints).standalone);
}

function wasRecentlyDismissed(): boolean {
  try {
    const dismissedAt = Number(localStorage.getItem(DISMISSAL_KEY));
    return dismissedAt > 0 && Date.now() - dismissedAt < DISMISSAL_DAYS * 24 * 60 * 60 * 1000;
  } catch {
    return false;
  }
}

function rememberDismissal() {
  try {
    localStorage.setItem(DISMISSAL_KEY, String(Date.now()));
  } catch {
    // Storage can be unavailable in private browsing. Hiding for this page is enough.
  }
}

export default function InstallPrompt() {
  const [visible, setVisible] = useState(false);
  const [showIOSHelp, setShowIOSHelp] = useState(false);
  const [installEvent, setInstallEvent] = useState<BeforeInstallPromptEvent | null>(null);
  const ios = typeof navigator !== "undefined" && isIOSDevice();

  useEffect(() => {
    if (isStandalone() || wasRecentlyDismissed()) return;

    let iosTimer: number | undefined;

    const handleInstallPrompt = (event: Event) => {
      if (!isMobileDevice()) return;
      event.preventDefault();
      setInstallEvent(event as BeforeInstallPromptEvent);
      setVisible(true);
    };
    const handleInstalled = () => {
      setVisible(false);
      setInstallEvent(null);
    };

    window.addEventListener("beforeinstallprompt", handleInstallPrompt);
    window.addEventListener("appinstalled", handleInstalled);

    if (isIOSDevice()) {
      iosTimer = window.setTimeout(() => setVisible(true), 1500);
    }

    return () => {
      if (iosTimer) window.clearTimeout(iosTimer);
      window.removeEventListener("beforeinstallprompt", handleInstallPrompt);
      window.removeEventListener("appinstalled", handleInstalled);
    };
  }, []);

  function dismiss() {
    rememberDismissal();
    setVisible(false);
  }

  async function install() {
    if (ios) {
      setShowIOSHelp(true);
      return;
    }
    if (!installEvent) return;

    await installEvent.prompt();
    const { outcome } = await installEvent.userChoice;
    setInstallEvent(null);
    if (outcome === "accepted") {
      setVisible(false);
    } else {
      dismiss();
    }
  }

  if (!visible) return null;

  return (
    <div className="install-prompt" role="dialog" aria-labelledby="install-title">
      <button className="install-prompt-close" onClick={dismiss} aria-label="Dismiss install prompt">×</button>
      <img src="/logo.png" alt="" className="install-prompt-logo" />
      <div className="install-prompt-content">
        <h2 id="install-title">Add Ground Control Roasters to your Home Screen</h2>
        {showIOSHelp ? (
          <ol className="install-steps">
            <li>Tap <strong>Share</strong> <span className="share-symbol" aria-hidden="true">□↑</span> in your browser toolbar.</li>
            <li>Select <strong>Add to Home Screen</strong>.</li>
            <li>Enable <strong>Open as Web App</strong>, then tap <strong>Add</strong>.</li>
          </ol>
        ) : (
          <p>Open wholesale ordering like an app with one tap.</p>
        )}
        <div className="install-prompt-actions">
          {!showIOSHelp && (
            <button className="install-primary" onClick={install}>
              {ios ? "Show me how" : "Install app"}
            </button>
          )}
          <button className="install-secondary" onClick={dismiss}>
            {showIOSHelp ? "Done" : "Not now"}
          </button>
        </div>
      </div>
    </div>
  );
}
