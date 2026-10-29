import { useEffect, useRef } from "react";
import type { MouseEvent } from "react";
import { CircleAlert } from "lucide-react";
import { cn } from "cn";
import { fieldId } from "@/shared/components/ui/form-field";

export interface FieldErrorItem {
  field: string;
  message: string;
}

/** What a form holds after a 422: apiFieldErrors' map, or the API's own list. */
export type FieldErrors = Record<string, string> | FieldErrorItem[];

/** One list from either shape, in the order given (the form's order when it is a map built in it). */
export function fieldErrorList(errors: FieldErrors | null | undefined): FieldErrorItem[] {
  if (!errors) return [];
  if (Array.isArray(errors)) return errors.filter((e) => e && typeof e.field === "string" && e.message);
  return Object.entries(errors)
    .filter(([, message]) => Boolean(message))
    .map(([field, message]) => ({ field, message }));
}

function prefersReducedMotion(): boolean {
  return typeof window !== "undefined" && window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
}

/** Scrolls a control into view and focuses it; false when no element has that id. */
export function focusField(name: string): boolean {
  const el = document.getElementById(fieldId(name));
  if (!el) return false;
  el.scrollIntoView({ block: "center", behavior: prefersReducedMotion() ? "auto" : "smooth" });
  el.focus({ preventScroll: true });
  return true;
}

/**
 * Focuses the first field in error (its FormField needs `name`). For a form that shows no
 * summary — a short dialog — call it after the 422 lands. Returns false when none was found.
 */
export function focusFirstError(errors: FieldErrors | null | undefined): boolean {
  return fieldErrorList(errors).some((e) => focusField(e.field));
}

export interface ErrorSummaryProps {
  errors: FieldErrors | null | undefined;
  /** Field name → the label the person saw; a field without one shows its name. */
  labels?: Record<string, string>;
  title?: string;
  className?: string;
}

/**
 * The list of everything wrong with a form, above it (GOV.UK Design System › Error summary):
 * takes focus when errors arrive, so a screen reader reads it, and each line links to its field.
 * The message beside the control is FormField's `error`; this is the same list in one place.
 * Renders nothing while there is no error. Give each FormField its `name` so the links land.
 */
export function ErrorSummary({ errors, labels, title = "Periksa kembali isian berikut", className }: ErrorSummaryProps) {
  const list = fieldErrorList(errors);
  const ref = useRef<HTMLDivElement>(null);
  const key = list.map((e) => `${e.field}:${e.message}`).join("\n");

  useEffect(() => {
    if (key) ref.current?.focus();
  }, [key]);

  if (list.length === 0) return null;

  function go(e: MouseEvent<HTMLAnchorElement>, field: string) {
    if (focusField(field)) e.preventDefault();
  }

  return (
    <div
      ref={ref}
      role="alert"
      tabIndex={-1}
      className={cn(
        "rounded-lg border border-destructive/40 border-l-4 border-l-destructive bg-destructive-soft px-4 py-3.5 text-foreground outline-none focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
        className,
      )}
    >
      <div className="flex items-start gap-2.5">
        <CircleAlert size={18} aria-hidden className="mt-0.5 shrink-0 text-destructive" />
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-destructive">{title}</h2>
          <ul className="mt-1.5 flex list-disc flex-col gap-1 pl-5 text-sm">
            {list.map((e) => (
              <li key={e.field}>
                <a
                  href={`#${fieldId(e.field)}`}
                  onClick={(ev) => go(ev, e.field)}
                  className="rounded-sm font-medium text-destructive underline underline-offset-2 hover:text-foreground focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                >
                  {labels?.[e.field] ?? e.field}
                </a>
                : {e.message}
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  );
}
