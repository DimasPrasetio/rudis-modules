import type { ReactNode } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/shared/components/primitives/alert-dialog";
import { useUnsavedChanges } from "@/shared/hooks/use-unsaved-changes";

export interface UnsavedChangesDialogProps {
  open: boolean;
  /** Keep the changes and stay (also Esc and the overlay). */
  onStay: () => void;
  /** Drop the changes and go. */
  onLeave: () => void;
  title?: string;
  description?: ReactNode;
  stayLabel?: string;
  leaveLabel?: string;
}

/**
 * The question itself, for a flow that is not a route change — closing a FormSheet or a Modal
 * with a dirty form: open it from the sheet's onClose and call onLeave to really close.
 * "Tetap di halaman" has focus, so Enter is the safe choice.
 */
export function UnsavedChangesDialog({
  open,
  onStay,
  onLeave,
  title = "Perubahan belum disimpan",
  description = "Jika Anda meninggalkan halaman ini, perubahan yang belum disimpan akan hilang.",
  stayLabel = "Tetap di halaman",
  leaveLabel = "Tinggalkan",
}: UnsavedChangesDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={(next) => !next && onStay()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription className="text-base text-foreground">{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel size="sm" onClick={onStay}>
            {stayLabel}
          </AlertDialogCancel>
          <AlertDialogAction variant="destructive" size="sm" onClick={onLeave}>
            {leaveLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export interface UnsavedChangesGuardProps extends Omit<UnsavedChangesDialogProps, "open" | "onStay" | "onLeave"> {
  /** True while the form has changes that are not saved. */
  when: boolean;
  /** The browser's reload/close prompt (the browser shows its own text). */
  message?: string;
}

/**
 * Put once in a form page (or inside the FormSheet/Modal that holds the form): while `when` is
 * true, leaving the route asks "Perubahan belum disimpan" — Tinggalkan goes on, Tetap di halaman
 * cancels — and reloading or closing the tab asks the browser's prompt. Renders nothing until
 * a navigation is held.
 */
export function UnsavedChangesGuard({ when, message, ...dialog }: UnsavedChangesGuardProps) {
  const { pending, leave, stay } = useUnsavedChanges(when, message);
  return <UnsavedChangesDialog open={pending} onStay={stay} onLeave={leave} {...dialog} />;
}
