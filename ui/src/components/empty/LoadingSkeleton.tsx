import { Skeleton } from "@/components/ui/skeleton";

interface LoadingSkeletonProps {
  label: string;
  rows?: number;
}

export function LoadingSkeleton({ label, rows = 3 }: LoadingSkeletonProps) {
  const count = Math.max(1, rows);
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={label}
      className="state-card state-card--loading"
      data-testid="loading-skeleton"
    >
      <div className="state-card__bars">
        {Array.from({ length: count }).map((_, i) => (
          <Skeleton key={i} data-testid="skeleton-row" className="state-card__bar" />
        ))}
      </div>
      <span className="sr-only">{label}</span>
    </div>
  );
}
