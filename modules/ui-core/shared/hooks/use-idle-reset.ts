import { useEffect, useRef } from "react";

export interface IdleResetOptions {
  /** What counts as activity (default: pointer, key, touch, wheel, and scroll events). */
  events?: readonly string[];
  /** false pauses the timer (a dialog that must stay open). */
  enabled?: boolean;
}

const DEFAULT_EVENTS: readonly string[] = ["pointerdown", "pointermove", "keydown", "touchstart", "wheel", "scroll"];

/**
 * Calls onIdle after ms without activity — a kiosk or POS screen that returns to its start, a
 * visitor book that clears a half-filled form. Any of the events restarts the timer; the timer
 * and the listeners go away with the component. The latest onIdle is used, so it may close over
 * state. ms of 0 or less disables it.
 */
export function useIdleReset(ms: number, onIdle: () => void, { events = DEFAULT_EVENTS, enabled = true }: IdleResetOptions = {}): void {
  const callback = useRef(onIdle);
  callback.current = onIdle;
  // A stable key for the effect: the array identity may change every render.
  const eventsKey = events.join(" ");

  useEffect(() => {
    if (!enabled || ms <= 0) return;
    let timer: number | undefined;
    const arm = () => {
      window.clearTimeout(timer);
      timer = window.setTimeout(() => callback.current(), ms);
    };
    const names = eventsKey.split(" ").filter(Boolean);
    for (const name of names) window.addEventListener(name, arm, { passive: true });
    arm();
    return () => {
      window.clearTimeout(timer);
      for (const name of names) window.removeEventListener(name, arm);
    };
  }, [ms, enabled, eventsKey]);
}
