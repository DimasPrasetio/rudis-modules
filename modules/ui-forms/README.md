# ui-forms

Data entry, in Bingkai Navy's design. Builds on `ui-core`. Every control uses the same field look
and works inside `Modal` and `FormSheet`.

| Component | Use |
|---|---|
| `FormField` | Label (required "*"), control, hint or error — `id`/`aria-*` wired |
| `PasswordInput` | Password with show/hide |
| `DigitsInput` | A number typed as text (IDs, accounts): digits only, `maxDigits`, groups shown while not editing |
| `AddressFields` | A postal address as fields — street, city, province, postal code, country (when `countries` is given); labels are props |
| `SearchSelect` | Select with search over known options |
| `AsyncSearchSelect` | Server-side search: debounced, stale requests aborted, `minChars`, `paged` infinite list, label kept for edit forms |
| `HierarchicalSelect` | Tree options with guide lines and paths; group nodes not selectable |
| `Segmented` · `MultiToggle` | One of a few (pill switch) · several as chips |
| `DateRangeFields` | From/to dates that limit each other |
| `KeyValueEditor` | Free name/value pairs with duplicate and empty-name checks |
| `FieldHint` | "?" popover beside a label |
| `FormSection` · `FieldGrid` · `StickyFooter` · `ActionRow` | Form layout: titled sections, field grid, pinned action bar, closing buttons |
| `LineItemsEditor` | A document's editable lines; Enter adds a line; cards when narrow |
| `FileDropzone` | Drag and drop or choose a file |
| `Stepper` | Steps of a flow or a lifecycle; `stopped` for a halted flow |

Selects share one keyboard model: ↓/↑ between search and options, Enter in the search picks the
first option.

## Changes

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
