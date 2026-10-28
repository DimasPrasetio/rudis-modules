/**
 * The one keyboard-focus look of every control the kits draw (WCAG 2.2 · 2.4.7 Focus Visible,
 * 1.4.11 Non-text Contrast): a solid 2px outline in --ring, offset from the control. An outline,
 * not a box-shadow ring: it never clips, needs no color mixing (a half-transparent ring falls
 * under 3:1), and is the same thing the preset draws for controls outside the kits.
 * Bingkai Navy: --ring is 4.86:1 on card and 4.65:1 on background.
 */
export const FOCUS_RING =
  "focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring";

/** The same outline drawn inside the element, for rows and options in a scrolling list. */
export const FOCUS_RING_INSET =
  "focus-visible:outline-solid focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring";
