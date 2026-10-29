# ui-forms

Data entry, in Bingkai Navy's design. Builds on `ui-core`. Every control uses the same field look
and works inside `Modal` and `FormSheet`.

| Component | Use |
|---|---|
| `FormField` | Label (required "*"), control, hint or error — `id`/`aria-*` wired; `name` gives the control the stable id `field-<name>` (what `ErrorSummary` links to) |
| `ErrorSummary` | The list of a form's errors above it (GOV.UK pattern): takes focus when errors arrive, each line links to its field; `focusFirstError(errors)` for a form without a summary |
| `UnsavedChangesGuard` · `useUnsavedChanges` · `UnsavedChangesDialog` | Asks before leaving a dirty form — in-app navigation (react-router `useBlocker`) and the browser's reload prompt |
| `DatePicker` · `Calendar` | A date typed dd/mm/yyyy or picked from a month grid in Indonesian; value `"YYYY-MM-DD"` |
| `DateRangePicker` | From/to as two `DatePicker`s with quick ranges (Hari ini … Tahun ini) |
| `PeriodPicker` · `MonthGrid` | A month of a year ("Oktober 2026") for reports and period closing; value `"YYYY-MM"` |
| `PasswordInput` | Password with show/hide |
| `DigitsInput` | A number typed as text (IDs, accounts): digits only, `maxDigits`, groups shown while not editing |
| `AddressFields` | A postal address as fields — street, city, province, postal code, country (when `countries` is given); labels are props |
| `SearchSelect` | Select with search over known options |
| `AsyncSearchSelect` | Server-side search: debounced, stale requests aborted, `minChars`, `paged` infinite list, label kept for edit forms |
| `HierarchicalSelect` | Tree options with guide lines and paths; group nodes not selectable |
| `Segmented` · `MultiToggle` | One of a few (pill switch) · several as chips |
| `DateRangeFields` | From/to as two plain `<input type="date">` that limit each other — the browser's own picker, for a toolbar that wants no popover |
| `KeyValueEditor` | Free name/value pairs with duplicate and empty-name checks |
| `FieldHint` | "?" popover beside a label |
| `FormSection` · `FieldGrid` · `StickyFooter` · `ActionRow` | Form layout: titled sections, field grid, pinned action bar, closing buttons |
| `LineItemsEditor` | A document's editable lines; Enter adds a line; cards when narrow |
| `FileDropzone` | Drag and drop or choose a file |
| `Stepper` | Steps of a flow or a lifecycle; `stopped` for a halted flow |

Selects share one keyboard model: ↓/↑ between search and options, Enter in the search picks the
first option.

## Form errors

A form validated on submit (never while typing) shows the same errors twice, as GOV.UK and NN/g
ask: the summary above the form, and the message beside each field.

```tsx
const [errors, setErrors] = useState<Record<string, string>>({});
// on a 422: setErrors(apiFieldErrors(err));
<ErrorSummary errors={errors} labels={{ email: "Email", customer_id: "Pelanggan" }} />
<FormField label="Email" name="email" error={errors.email}>
  <Input type="email" … />
</FormField>
```

`errors` is what `apiFieldErrors` returns (field → message) or the API's own list
(`{ field, message }[]`). The summary renders nothing while empty, takes focus whenever the list
changes (`role="alert"`, `tabIndex=-1`), and each line is a link to `#field-<name>` that focuses
the control without a hash jump (scrolling respects `prefers-reduced-motion`). `name` on a
FormField is what makes the link land — give it to every field the API can reject. For a short
dialog without room for a summary, `focusFirstError(errors)` after the 422.

## Unsaved changes

```tsx
const dirty = form.isDirty;
<UnsavedChangesGuard when={dirty} />
```

While `when` is true, a navigation inside the app is held and the AlertDialog asks "Perubahan
belum disimpan" — Tinggalkan goes on, Tetap di halaman (focused, so Enter is safe) stays — and
reloading or closing the tab shows the browser's own prompt (`beforeunload`; browsers no longer
show custom text). In-app blocking uses react-router's `useBlocker`, which the kernel's data
router (`createBrowserRouter`) provides; the guard must render inside the router, as every page
does. A navigation that only changes the search params (filters, tabs) is not blocked. One guard
per form: react-router allows one blocker at a time. Closing a `FormSheet`/`Modal` is not a
route change — keep the question for it with `UnsavedChangesDialog` from the sheet's `onClose`,
or `useUnsavedChanges(when)` when you draw your own.

## Dates

Every date leaves a component as `"YYYY-MM-DD"` (the API's form) and is shown as people write it,
dd/mm/yyyy with Indonesian month and weekday names (`Intl`, "id-ID", no date library). The math
is in `shared/lib/dates.ts` (`parseDateID`, `formatDateID`, `addDays`, `addMonths`, `monthGrid`,
…), pure, with no time zone: a calendar date never shifts.

- `DatePicker` — as `FormField`'s single child. Typing gets its slashes placed ("5/1/2026" →
  "05/01/2026" on blur); a real date in `min`/`max` is handed back at once; anything else stays
  visible and `aria-invalid` with a hidden explanation, never wiped. The calendar button opens
  `Calendar`: ←/→ a day, ↑/↓ a week, PageUp/PageDown a month (Shift: a year), Home/End the week's
  edges, Enter picks, Escape returns to the field. Days are 32 px, 44 px under
  `<html data-density="touch">`. `todayLabel`/`clearLabel` (null hides them).
- `DateRangePicker` — a report's or list's range: two `DatePicker`s and presets computed from
  `todayISO()` (ui-core; the app's display time zone). A reversed range is told under the second
  field, not silently swapped. `presets` takes your own (`{ key, label, range(today) }`) or `[]`.
- `PeriodPicker` — a month: value `"YYYY-MM"`, bounds `min`/`max` in the same form; the grid's
  keys are ←/→ a month, ↑/↓ a row, PageUp/PageDown a year, Home/End January/December.
- `DateRangeFields` stays for a toolbar's "more filters" slot that wants two native inputs.

## Changes

- **0.4.0** — Forms that fail the way the design system asks (ROADMAP F23): `FormField` `name`
  → id `field-<name>`; `ErrorSummary` (GOV.UK error summary: focused on arrival, links to the
  fields) with `focusFirstError`; `UnsavedChangesGuard` / `useUnsavedChanges` /
  `UnsavedChangesDialog` (react-router `useBlocker` + `beforeunload`). Indonesian dates without
  a library (F24): `DatePicker` (dd/mm/yyyy typed with masking, a hand-built accessible month
  grid — `role="grid"`, roving tab stop, Indonesian names, min/max, today, touch density),
  `DateRangePicker` with presets, `PeriodPicker` for months, and `shared/lib/dates.ts`.
  `DateRangeFields` stays as the plain-inputs variant.
- **0.3.0** — `MoneyInput`: amounts typed and pasted the Indonesian way ("1.250.000",
  "1.250.000,50", "Rp 1.250.000"), grouped while typing, handed back as the money kit's plain
  string ("1250000.5") with no floating point — "1.500" is one thousand five hundred, never 1.5;
  `fractionDigits`, `allowNegative`, `min`/`max` (aria-invalid outside them); text input with an
  inputmode, not type="number" (GOV.UK Design System, Text input › Numbers). Fixes from the
  accessibility audit: select options show keyboard focus (a `--ring` outline on `--accent`, was
  1.04:1); the clear button is a real button beside the select trigger, no longer nested inside
  it; `FileDropzone` shows focus on the whole zone and announces uploading and the chosen file;
  `labelAside` sits beside the label, not inside it, so a field's name is its label alone;
  `DigitsInput` lets callers set `autoComplete`/`inputMode` (an OTP or a phone);
  `AddressFields` `autofill="self" | "billing" | "shipping"` gives the fields their autocomplete
  tokens (WCAG 1.3.5) and the postal code a numeric keypad.
- **0.2.0** — `DigitsInput` (digits only, grouped while not editing) and `AddressFields`
  (street, city, province, postal code, country).
- **0.1.0** — first version.
