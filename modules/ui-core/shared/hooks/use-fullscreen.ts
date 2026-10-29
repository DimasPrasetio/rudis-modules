import { useCallback, useEffect, useState } from "react";

export interface Fullscreen {
  /** The document is in full screen now. */
  active: boolean;
  /** The browser offers the Fullscreen API (iOS Safari does not for a document). */
  supported: boolean;
  enter: () => Promise<void>;
  exit: () => Promise<void>;
  toggle: () => Promise<void>;
}

const supported = typeof document !== "undefined" && typeof document.documentElement.requestFullscreen === "function";

const isActive = () => typeof document !== "undefined" && document.fullscreenElement !== null;

/**
 * Puts the whole app in full screen (a kiosk, a POS, a queue display) and follows the state —
 * the user can always leave with Esc, which `active` reports. Call `enter` from a click or a key
 * (browsers refuse it otherwise). Nothing happens where it is not supported.
 */
export function useFullscreen(): Fullscreen {
  const [active, setActive] = useState(isActive);

  useEffect(() => {
    const onChange = () => setActive(isActive());
    document.addEventListener("fullscreenchange", onChange);
    return () => document.removeEventListener("fullscreenchange", onChange);
  }, []);

  const enter = useCallback(async () => {
    if (!supported || isActive()) return;
    try {
      await document.documentElement.requestFullscreen({ navigationUI: "hide" });
    } catch {
      // Refused (not from a user gesture, or an embedded frame): the screen stays as it is.
    }
  }, []);

  const exit = useCallback(async () => {
    if (!isActive()) return;
    try {
      await document.exitFullscreen();
    } catch {
      // Already left.
    }
  }, []);

  const toggle = useCallback(() => (isActive() ? exit() : enter()), [enter, exit]);

  return { active, supported, enter, exit, toggle };
}
