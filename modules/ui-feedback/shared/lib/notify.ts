import { toast } from "sonner";
import { apiErrorMessage } from "@/shared/services/api-error";

export interface UndoableOptions {
  /** Called when the person presses the undo button. */
  onUndo: () => void | Promise<void>;
  /** Called once the toast is gone without undo — the moment to really delete. */
  onExpire?: () => void;
  /** How long the undo stays offered, ms. */
  duration?: number;
  label?: string;
  /** A second line under the message. */
  description?: string;
}

/**
 * Short confirmations and failures as toasts. An error shows the API's own message.
 * Use a toast for something that happened; use a Notice for something the user must read.
 */
export const notify = {
  success: (message: string) => toast.success(message),
  info: (message: string) => toast.info(message),
  warning: (message: string) => toast.warning(message),
  error: (err: unknown, fallback?: string) => toast.error(typeof err === "string" ? err : apiErrorMessage(err, fallback)),

  /**
   * "Pelanggan diarsipkan — Urungkan": the toast carries an undo button for `duration` ms.
   * Two ways to use it: apply the change at once and revert it in `onUndo`; or hold a
   * destructive call until the toast is gone — do it in `onExpire`, which runs when the toast
   * closes by itself, by its close button, or by notify.dismiss(id), but not after undo.
   * Returns the toast id.
   */
  undoable(message: string, { onUndo, onExpire, duration = 8000, label = "Urungkan", description }: UndoableOptions) {
    let undone = false;
    const expire = () => {
      if (!undone) onExpire?.();
    };
    return toast(message, {
      description,
      duration,
      action: {
        label,
        onClick: () => {
          undone = true;
          void onUndo();
        },
      },
      onAutoClose: expire,
      onDismiss: expire,
    });
  },

  /** Closes one toast, or all of them. */
  dismiss: (id?: string | number) => toast.dismiss(id),
};
