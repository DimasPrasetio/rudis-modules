import { useCallback, useEffect } from "react";
import { useBlocker } from "react-router-dom";

export interface UnsavedChanges {
  /** A navigation inside the app is waiting for the person's answer. */
  pending: boolean;
  /** Let the pending navigation happen. */
  leave: () => void;
  /** Cancel the pending navigation and stay. */
  stay: () => void;
}

/**
 * Keeps a form's unsaved changes from being lost: while `when` is true, closing or reloading
 * the tab asks the browser's own confirmation (`beforeunload`; the text is the browser's, not
 * `message` — browsers no longer show custom text), and a navigation inside the app is held
 * (`pending`) until `leave()` or `stay()` — UnsavedChangesGuard draws that question.
 *
 * In-app blocking uses react-router's `useBlocker`, which needs the data router the kernel
 * creates (`createBrowserRouter`); the hook must render inside it, as every page does. A
 * navigation to the same path with other search params is not blocked (filters, tabs).
 */
export function useUnsavedChanges(when: boolean, message = "Perubahan belum disimpan. Tinggalkan halaman ini?"): UnsavedChanges {
  const blocker = useBlocker(({ currentLocation, nextLocation }) => when && currentLocation.pathname !== nextLocation.pathname);

  useEffect(() => {
    if (!when) return;
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      // Older browsers read returnValue; current ones show their own text.
      e.returnValue = message;
      return message;
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [when, message]);

  // A blocker left "blocked" after the form became clean (saved, then the person clicks a link
  // that was blocked a moment ago) would hold the navigation forever.
  useEffect(() => {
    if (blocker.state === "blocked" && !when) blocker.proceed();
  }, [blocker, when]);

  const leave = useCallback(() => {
    if (blocker.state === "blocked") blocker.proceed();
  }, [blocker]);
  const stay = useCallback(() => {
    if (blocker.state === "blocked") blocker.reset();
  }, [blocker]);

  return { pending: blocker.state === "blocked", leave, stay };
}
