import type { ReactNode } from "react";
import { AlertTriangle, CheckCircle2, Info, X, XCircle } from "lucide-react";
import { cn } from "cn";
import { FOCUS_RING } from "@/shared/lib/focus";

export type BannerTone = "info" | "warning" | "danger" | "success";

const STYLE: Record<BannerTone, { wrap: string; accent: string; icon: typeof Info }> = {
  info: { wrap: "bg-info-soft text-info", accent: "border-l-info", icon: Info },
  warning: { wrap: "bg-warning-soft text-warning", accent: "border-l-warning", icon: AlertTriangle },
  danger: { wrap: "bg-destructive-soft text-destructive", accent: "border-l-destructive", icon: XCircle },
  success: { wrap: "bg-success-soft text-success", accent: "border-l-success", icon: CheckCircle2 },
};

export interface BannerProps {
  tone: BannerTone;
  title: string;
  children?: ReactNode;
  /** Buttons or links on the right (a `Button size="sm" variant="outline"`, a link). */
  actions?: ReactNode;
  /** Shows a "Tutup" button; the caller hides the banner. */
  onDismiss?: () => void;
  dismissLabel?: string;
  className?: string;
}

/**
 * A message that stays — for a page or the whole app: a period is closed, the trial ends in 3
 * days, the server is in maintenance. Full width with a colored edge; danger is announced at
 * once (role=alert), the rest politely (role=status). An inline `Notice` (ui-core) explains one
 * thing in one place; a toast reports something that just happened.
 */
export function Banner({ tone, title, children, actions, onDismiss, dismissLabel = "Tutup", className }: BannerProps) {
  const { wrap, accent, icon: Icon } = STYLE[tone];
  return (
    <div
      role={tone === "danger" ? "alert" : "status"}
      className={cn("flex w-full items-start gap-3 border-l-4 px-4 py-3 sm:items-center", wrap, accent, className)}
    >
      <Icon size={18} aria-hidden className="mt-0.5 shrink-0 sm:mt-0" />
      <div className="flex min-w-0 flex-1 flex-col gap-x-4 gap-y-1 sm:flex-row sm:items-center">
        <div className="min-w-0 flex-1">
          <p className="text-sm font-semibold">{title}</p>
          {children && <div className="text-sm text-foreground">{children}</div>}
        </div>
        {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
      </div>
      {onDismiss && (
        <button
          type="button"
          aria-label={dismissLabel}
          onClick={onDismiss}
          className={cn("-my-1 -mr-2 flex size-8 shrink-0 items-center justify-center rounded-md transition-colors hover:bg-foreground/10", FOCUS_RING)}
        >
          <X size={16} aria-hidden />
        </button>
      )}
    </div>
  );
}
