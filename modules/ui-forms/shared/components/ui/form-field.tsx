import { cloneElement, isValidElement, useId } from "react";
import type { ReactElement, ReactNode } from "react";

export interface FormFieldProps {
  label: string;
  /** One control; it gets id, aria-invalid, and aria-describedby wired. */
  children: ReactElement<{ id?: string; "aria-invalid"?: boolean; "aria-describedby"?: string }> | ReactNode;
  required?: boolean;
  /** Help under the control. */
  hint?: ReactNode;
  /** Replaces the hint while set. */
  error?: string;
  /**
   * The field's name as the API knows it ("email", "customer_id"). Gives the control the stable id
   * `field-<name>`, which ErrorSummary links to and focusFirstError focuses; without it the id is
   * generated. Also sets the control's `name` when it has none.
   */
  name?: string;
  /** For a control that is not the direct child (e.g. a composite). */
  htmlFor?: string;
  /** Extra beside the label, e.g. a FieldHint or a "Lupa kata sandi?" link. Drawn next to the label,
   *  never inside it, so the control's accessible name stays the label alone. */
  labelAside?: ReactNode;
}

/** The id a FormField with `name` gives its control: "field-<name>". */
export function fieldId(name: string): string {
  return `field-${name}`;
}

/** Label, control, and its hint or error. Required is a calm "*", read as "wajib diisi". */
export function FormField({ label, children, required, hint, error, name, htmlFor, labelAside }: FormFieldProps) {
  const autoId = useId();
  const messageId = `${autoId}-message`;
  const single = isValidElement(children) ? (children as ReactElement<{ id?: string; name?: string }>) : null;
  const controlId = htmlFor ?? single?.props.id ?? (name ? fieldId(name) : single ? autoId : undefined);
  const control = single
    ? cloneElement(single as ReactElement<Record<string, unknown>>, {
        id: controlId,
        name: single.props.name ?? name,
        "aria-invalid": error ? true : undefined,
        "aria-describedby": error || hint ? messageId : undefined,
      })
    : children;

  return (
    <div className="min-w-0">
      <div className="mb-1.5 flex items-center gap-1">
        <label htmlFor={controlId} className="flex items-center gap-1 text-sm font-medium text-foreground">
          {label}
          {required && (
            <>
              <span aria-hidden className="text-destructive">
                *
              </span>
              <span className="sr-only">(wajib diisi)</span>
            </>
          )}
        </label>
        {labelAside}
      </div>
      {control}
      {error ? (
        <p id={messageId} role="alert" className="mt-1.5 text-xs font-medium text-destructive">
          {error}
        </p>
      ) : hint ? (
        <p id={messageId} className="mt-1.5 text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
