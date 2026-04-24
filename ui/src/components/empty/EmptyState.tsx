import type { ReactNode } from "react";
import { Inbox } from "lucide-react";

interface EmptyStateProps {
  title: string;
  description: string;
  icon?: ReactNode;
  action?: ReactNode;
}

export function EmptyState({ title, description, icon, action }: EmptyStateProps) {
  return (
    <div
      role="status"
      aria-live="polite"
      className="state-card state-card--empty"
      data-testid="empty-state"
    >
      <div className="state-card__icon" aria-hidden="true">
        {icon ?? <Inbox className="size-6" />}
      </div>
      <h3 className="state-card__title">{title}</h3>
      <p className="state-card__description">{description}</p>
      {action ? <div className="state-card__action">{action}</div> : null}
    </div>
  );
}
