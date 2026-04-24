import { ShieldAlert } from "lucide-react";
import { useAuthStore } from "@/stores/auth";

// AuthRequiredBanner renders a visible "Authentication required"
// affordance when the API has returned 401 anywhere in the app.
// Mounted inside <main id="main"> so smoke tests that scope to that
// landmark always find the copy.
//
// Copy keywords ("Sign in", "authentication required", "session") are
// intentionally aligned with ui/e2e/auth.spec.ts so the Playwright
// smoke can match without embedding brittle selectors.
export function AuthRequiredBanner() {
  const authRequired = useAuthStore((s) => s.authRequired);
  if (!authRequired) return null;
  return (
    <div
      role="alert"
      aria-live="assertive"
      data-testid="auth-required-banner"
      className="state-card state-card--error"
    >
      <div className="state-card__icon state-card__icon--danger" aria-hidden="true">
        <ShieldAlert className="size-6" />
      </div>
      <h3 className="state-card__title">Sign in required</h3>
      <p className="state-card__description">
        Your session has expired or is missing. Please sign in again —
        run <code>docsiq login</code> on the server, or reload the page
        after authentication is re-established.
      </p>
      <div className="state-card__action">
        <button
          type="button"
          className="inline-flex items-center justify-center rounded-md border px-3 py-1.5 text-sm hover:bg-accent"
          onClick={() => window.location.reload()}
        >
          Reload
        </button>
      </div>
    </div>
  );
}
