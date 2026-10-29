# ui-feedback

Telling people what happened, in Bingkai Navy's design. Builds on `ui-core` and `ui-forms`.
Registers a `provider` (`FeedbackProvider`) that wraps the app once: an error boundary showing a
crash card (with "versi baru" detection after a deploy), the global loading bar for running
http-client requests, and the toast container.

| Piece | Use |
|---|---|
| `notify.success / info / warning / error(err)` | Toasts; `error` shows the API's message |
| `notify.undoable(message, { onUndo, onExpire, duration, label })` · `notify.dismiss(id?)` | A toast with "Urungkan" for an action that is cheap to take back (archive, remove from a list) |
| `Banner` | A message that stays, for a page or the whole app (a closed period, planned maintenance, a lost connection): tone, title, details, actions, dismiss |
| `ConfirmDialog` | Before something hard to undo; async-aware; shows the error and stays open if it fails |
| `ReasonDialog` | A confirmation that requires a reason (cancel, void) |
| `GlobalLoader` | Thin bar at the top while requests run (mounted by the provider) |
| `ErrorBoundary` · `CrashCard` | Render failures as a card with an error ID, not a blank page |

A toast reports something that happened; a `Notice` (ui-core) is something the user must read
in one place; a `Banner` is something that stays true for the whole page or app until it is
resolved or dismissed.

## Undo instead of confirm

For an action that is cheap to reverse, an undo toast beats a confirmation dialog (NN/g): the
person is not interrupted and still has eight seconds to take it back.

```ts
// 1. Optimistic: apply now, revert on undo.
setRows((rs) => rs.filter((r) => r.id !== row.id));
await api.archive(row.id);
notify.undoable("Pelanggan diarsipkan", { onUndo: () => api.unarchive(row.id).then(reload) });

// 2. Deferred: hide now, really delete only once the toast is gone without undo.
setRows((rs) => rs.filter((r) => r.id !== row.id));
notify.undoable("Pelanggan dihapus", {
  onUndo: () => setRows(previous),
  onExpire: () => api.remove(row.id),
});
```

`onExpire` runs when the toast closes by itself, by its close button, or by
`notify.dismiss(id)`, and never after undo. The toast's id is returned. Keep it for something
that is cheap to take back; a destructive action that is not (void a posted document) still
goes through `ConfirmDialog` or `ReasonDialog`.

## Banner

```tsx
<Banner tone="warning" title="Periode Oktober 2026 belum ditutup" actions={<Button size="sm" variant="outline">Tutup periode</Button>} onDismiss={() => setShown(false)}>
  Transaksi masih bisa diubah sampai periode ditutup.
</Banner>
```

Full width with a colored edge, an icon per tone, `role="status"` (danger: `role="alert"`, read
at once), and a "Tutup" button (≥ 32 px) when `onDismiss` is given — the caller decides whether a
dismissal is remembered. Place it at the top of a page's content, or in the app shell's header
slot for the whole app.

## Changes

- **0.2.0** — `notify.undoable` (sonner `action`, `onExpire` for a deferred destructive call) and
  `notify.dismiss`; `Banner` (info / warning / danger / success, actions, dismiss) for page- and
  app-level messages (ROADMAP 0.14).
- **0.1.0** — first version.
