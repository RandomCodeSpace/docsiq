import { AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

interface ErrorStateProps {
  title: string;
  message: string;
  onRetry?: () => void;
}

const MAX_MESSAGE_LEN = 500;

function sanitize(raw: string): string {
  const trimmed = raw.trim().replace(/\s+/g, " ");
  if (trimmed.length <= MAX_MESSAGE_LEN) return trimmed;
  return `${trimmed.slice(0, MAX_MESSAGE_LEN)}…`;
}

export function ErrorState({ title, message, onRetry }: ErrorStateProps) {
  return (
    <div
      role="alert"
      className="state-card state-card--error"
      data-testid="error-state"
    >
      <div className="state-card__icon state-card__icon--danger" aria-hidden="true">
        <AlertTriangle className="size-6" />
      </div>
      <h3 className="state-card__title">{title}</h3>
      <p className="state-card__description" data-testid="error-message">
        {sanitize(message)}
      </p>
      {onRetry ? (
        <div className="state-card__action">
          <Button type="button" size="sm" variant="outline" onClick={onRetry}>
            Retry
          </Button>
        </div>
      ) : null}
    </div>
  );
}
