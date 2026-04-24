import { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

interface RouteBoundaryProps { children: ReactNode }
interface RouteBoundaryState { error: Error | null }

const MAX_MESSAGE_LEN = 500;
const REPORT_EMAIL = "docsiq-support@example.invalid";

function sanitize(raw: string): string {
  const trimmed = raw.trim().replace(/\s+/g, " ");
  if (trimmed.length <= MAX_MESSAGE_LEN) return trimmed;
  return `${trimmed.slice(0, MAX_MESSAGE_LEN)}…`;
}

function buildMailto(err: Error): string {
  const subject = encodeURIComponent(`[docsiq] Render error: ${err.name}`);
  const bodyLines = [
    "What I was doing:",
    "",
    "",
    "Message:",
    sanitize(err.message),
    "",
    "Stack (truncated):",
    (err.stack ?? "").split("\n").slice(0, 10).join("\n"),
    "",
    `URL: ${typeof window !== "undefined" ? window.location.href : ""}`,
    `UA:  ${typeof navigator !== "undefined" ? navigator.userAgent : ""}`,
  ].join("\n");
  const body = encodeURIComponent(bodyLines);
  return `mailto:${REPORT_EMAIL}?subject=${subject}&body=${body}`;
}

export class RouteBoundary extends Component<RouteBoundaryProps, RouteBoundaryState> {
  state: RouteBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): RouteBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // slog-equivalent: keep one structured line; never dump user PII
    // eslint-disable-next-line no-console
    console.error("[RouteBoundary]", error.name, sanitize(error.message), info.componentStack);
  }

  private reset = () => this.setState({ error: null });

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;
    return (
      <div role="alert" className="state-card state-card--error" data-testid="route-boundary">
        <div className="state-card__icon state-card__icon--danger" aria-hidden="true">
          <AlertTriangle className="size-6" />
        </div>
        <h3 className="state-card__title">Something went wrong</h3>
        <p className="state-card__description" data-testid="boundary-message">
          {sanitize(error.message || "Unknown render error")}
        </p>
        <div className="state-card__action flex gap-2">
          <Button type="button" size="sm" variant="outline" onClick={this.reset}>
            Reload this view
          </Button>
          <a
            className="inline-flex items-center justify-center rounded-md border px-3 py-1.5 text-sm hover:bg-accent"
            href={buildMailto(error)}
            rel="noopener noreferrer"
          >
            Report
          </a>
        </div>
      </div>
    );
  }
}
