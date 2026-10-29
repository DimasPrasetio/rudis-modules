import { useEffect, useMemo, useRef, useState } from "react";
import type { KeyboardEvent, MouseEvent, ReactNode } from "react";
import { Link } from "react-router-dom";
import { ArrowDown, ArrowUp, ArrowUpDown, Columns3 } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "@/shared/components/primitives/popover";
import { Skeleton } from "@/shared/components/primitives/skeleton";
import { EmptyState } from "@/shared/components/ui/empty-state";
import { ErrorState } from "@/shared/components/ui/error-state";
import { FOCUS_RING } from "@/shared/lib/focus";
import { onHiddenColumnsChange, readHiddenColumns, writeHiddenColumns } from "@/shared/lib/table-columns";

export type DataTableAlign = "left" | "center" | "right";
export type SortDir = "asc" | "desc";
export interface DataTableSort {
  key: string;
  dir: SortDir;
}

export interface DataTableColumn<T> {
  header: string;
  render: (row: T) => ReactNode;
  /** Stable id for the column chooser (default: header). */
  id?: string;
  width?: string | number;
  /** Numbers and money: "right". */
  align?: DataTableAlign;
  /** Server-side sort key; makes the header clickable. */
  sortKey?: string;
  /** Hidden until the user shows it from the "Kolom" menu. */
  defaultHidden?: boolean;
  /**
   * The row's identity as a real link to its page: the cell wraps `render(row)` in a react-router
   * Link, so it opens in a new tab with ⌘/Ctrl-click or the middle button. On a phone this column
   * is the card's title. At most one column per table.
   */
  link?: (row: T) => string;
  /** Left out of the phone layout (a secondary column); still in the table on wider screens. */
  mobileHidden?: boolean;
}

/** Every visible string, Indonesian by default. */
export interface DataTableLabels {
  /** "{n} dipilih" — the bulk bar's count. */
  selected: (n: number) => string;
  /** "Pilih semua {total} baris" — offered when the whole page is selected and there are more. */
  selectAllPages: (total: number) => string;
  /** "Semua {total} baris dipilih" */
  allPagesSelected: (total: number) => string;
  clearSelection: string;
  /** Read by screen readers when the selection changes: "{n} baris dipilih". */
  announceSelected: (n: number) => string;
  announceAllPages: (total: number) => string;
  announceCleared: string;
  selectAllRows: string;
  selectRow: string;
  columns: string;
  loading: string;
  /** Accessible name of the scrolling region when no caption is given. */
  region: string;
}

const LABELS: DataTableLabels = {
  selected: (n) => `${n} dipilih`,
  selectAllPages: (total) => `Pilih semua ${total} baris`,
  allPagesSelected: (total) => `Semua ${total} baris dipilih`,
  clearSelection: "Batal pilih",
  announceSelected: (n) => `${n} baris dipilih`,
  announceAllPages: (total) => `Semua ${total} baris dipilih`,
  announceCleared: "Pilihan dibatalkan",
  selectAllRows: "Pilih semua baris",
  selectRow: "Pilih baris",
  columns: "Kolom",
  loading: "Memuat data",
  region: "Tabel data",
};

export interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  /** null/undefined is empty (a Go API sends null for an empty slice). */
  rows: T[] | null | undefined;
  rowKey: (row: T) => string | number;
  /** Skeleton rows while there are no rows yet; dimmed while reloading. The header stays. */
  loading?: boolean;
  /** A failed load — the API's message, with "Coba lagi" when onRetry is given. The header stays. */
  error?: string | null;
  onRetry?: () => void;
  emptyTitle?: string;
  emptyDescription?: ReactNode;
  /** E.g. "Buat produk pertama". */
  emptyAction?: ReactNode;
  /** The whole row opens the record (clicks on links, buttons, and inputs inside are ignored). */
  onRowClick?: (row: T) => void;
  /** Accessible name of a clickable row, e.g. "Buka SO-2026-00012". */
  rowLabel?: (row: T) => string;
  sort?: DataTableSort | null;
  onSortChange?: (sort: DataTableSort) => void;
  /** A checkbox per row and a bulk-action bar. */
  selectable?: boolean;
  selectedKeys?: ReadonlySet<string | number>;
  /** Any change of the page's selection; a page that tracks allPagesSelected resets it here. */
  onSelectionChange?: (keys: Set<string | number>) => void;
  /** The bulk-action bar's content; replaces the toolbar while rows are selected. */
  bulkActions?: (selected: ReadonlySet<string | number>) => ReactNode;
  /** How many rows there are across every page; with onSelectAllPages, offers to select them all. */
  totalCount?: number;
  /** The page's own state: the user chose every row across pages (sent to its API as such). */
  allPagesSelected?: boolean;
  onSelectAllPages?: () => void;
  stickyHeader?: boolean;
  /** Height before the table scrolls inside itself. */
  maxHeight?: string;
  /** Shows the "Kolom" chooser and remembers the choice under this key. */
  columnsKey?: string;
  /** Accessible name of the table. */
  caption?: string;
  labels?: Partial<DataTableLabels>;
}

const ALIGN: Record<DataTableAlign, string> = { left: "text-left", center: "text-center", right: "text-right" };

// Includes portal content (a ⋯ menu, a dialog): React events bubble through the component tree to <tr>.
const INTERACTIVE =
  "a,button,input,select,textarea,label,[role=button],[role=checkbox],[role=menu],[role=menuitem],[role=dialog],[role=alertdialog],[role=listbox],[role=option]";

const colId = <T,>(c: DataTableColumn<T>) => c.id ?? c.header;

function initialHidden<T>(key: string | undefined, columns: DataTableColumn<T>[]): Set<string> {
  const saved = key ? readHiddenColumns(key) : null;
  return new Set(saved ?? columns.filter((c) => c.defaultHidden).map(colId));
}

// Below md the <table> is laid out as cards (every element display:block, the header visually
// hidden); the explicit roles keep it a table for screen readers, which drop the implicit ones
// when the display changes.
const CELL_MOBILE = "max-md:grid max-md:grid-cols-[7rem_minmax(0,1fr)] max-md:items-baseline max-md:gap-x-3 max-md:px-0 max-md:py-1 max-md:text-left";
const LABEL_MOBILE = "text-xs font-medium text-muted-foreground md:hidden";

/**
 * A checkbox with a hit area around it: 40px by default, at least `--control-h` (44px) under the
 * touch density. A <label> around the button: clicking the padding toggles it too.
 */
function CellCheckbox({ label, checked, onChange }: { label: string; checked: boolean | "indeterminate"; onChange: (checked: boolean) => void }) {
  return (
    <label className="-m-3 inline-flex min-h-(--control-h) min-w-(--control-h) cursor-pointer items-center justify-center rounded-md p-3 align-middle">
      <Checkbox aria-label={label} checked={checked} onCheckedChange={(v) => onChange(v === true)} />
    </label>
  );
}

/**
 * The list table: sticky header, sortable columns, density-aware rows, clickable rows with the
 * identity as a real link, selection with a bulk-action bar (also across pages), a column
 * chooser, built-in loading, empty, and error states that keep the header, and a card layout
 * on phones. Pagination and filters sit around it (Pagination, Toolbar).
 */
export function DataTable<T>({
  columns,
  rows: rowsProp,
  rowKey,
  loading = false,
  error,
  onRetry,
  emptyTitle = "Belum ada data",
  emptyDescription,
  emptyAction,
  onRowClick,
  rowLabel,
  sort,
  onSortChange,
  selectable = false,
  selectedKeys,
  onSelectionChange,
  bulkActions,
  totalCount,
  allPagesSelected = false,
  onSelectAllPages,
  stickyHeader = true,
  maxHeight = "70vh",
  columnsKey,
  caption,
  labels,
}: DataTableProps<T>) {
  const t = { ...LABELS, ...labels };
  const rows = rowsProp ?? [];
  const [hidden, setHidden] = useState<Set<string>>(() => initialHidden(columnsKey, columns));
  useEffect(() => {
    if (!columnsKey) return;
    return onHiddenColumnsChange(columnsKey, () => setHidden(initialHidden(columnsKey, columns)));
    // columns is a new array every render; its defaults are read only when the event arrives.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [columnsKey]);
  const visible = useMemo(() => columns.filter((c) => !hidden.has(colId(c))), [columns, hidden]);
  const span = visible.length + (selectable ? 1 : 0);

  const selected = selectedKeys ?? new Set<string | number>();
  const allSelected = rows.length > 0 && rows.every((r) => selected.has(rowKey(r)));
  const someSelected = !allSelected && rows.some((r) => selected.has(rowKey(r)));
  const hasSelection = selectable && (selected.size > 0 || allPagesSelected);
  const canSelectAllPages = Boolean(onSelectAllPages) && totalCount !== undefined && totalCount > rows.length && allSelected;

  // What screen readers hear when the selection changes, without the focus moving (WCAG 4.1.3).
  const [announcement, setAnnouncement] = useState("");
  const previous = useRef({ size: 0, all: false });
  useEffect(() => {
    if (!selectable) return;
    const size = selected.size;
    const was = previous.current;
    previous.current = { size, all: allPagesSelected };
    if (allPagesSelected) setAnnouncement(t.announceAllPages(totalCount ?? size));
    else if (size > 0) setAnnouncement(t.announceSelected(size));
    else if (was.size > 0 || was.all) setAnnouncement(t.announceCleared);
    // Only the counts matter; the label functions are stable defaults or the page's own.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectable, selected.size, allPagesSelected, totalCount]);

  function toggleAll(checked: boolean) {
    const next = new Set(selected);
    for (const r of rows) {
      if (checked) next.add(rowKey(r));
      else next.delete(rowKey(r));
    }
    onSelectionChange?.(next);
  }

  function toggleOne(key: string | number, checked: boolean) {
    const next = new Set(selected);
    if (checked) next.add(key);
    else next.delete(key);
    onSelectionChange?.(next);
  }

  function toggleColumn(id: string) {
    const next = new Set(hidden);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setHidden(next);
    if (columnsKey) writeHiddenColumns(columnsKey, [...next]);
  }

  function rowClick(e: MouseEvent<HTMLTableRowElement>, row: T) {
    if (!onRowClick || (e.target as HTMLElement).closest(INTERACTIVE)) return;
    onRowClick(row);
  }

  function rowKeyDown(e: KeyboardEvent<HTMLTableRowElement>, row: T) {
    if (!onRowClick || e.target !== e.currentTarget || e.key !== "Enter") return;
    e.preventDefault();
    onRowClick(row);
  }

  const showBar = Boolean(columnsKey) || hasSelection;
  const showSkeleton = !error && loading && rows.length === 0;
  const showEmpty = !error && !loading && rows.length === 0;

  return (
    <div className={cn(loading && rows.length > 0 && "opacity-60 transition-opacity")} aria-busy={loading || undefined}>
      {selectable && (
        <div aria-live="polite" aria-atomic="true" className="sr-only">
          {announcement}
        </div>
      )}
      {showBar && (
        <div className="flex min-h-12 flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-2 md:px-5">
          {hasSelection ? (
            <div className="flex flex-wrap items-center gap-3">
              <span className="tabular text-sm font-medium text-foreground">
                {allPagesSelected ? t.allPagesSelected(totalCount ?? selected.size) : t.selected(selected.size)}
              </span>
              {canSelectAllPages && !allPagesSelected && (
                <Button variant="link" size="sm" className="h-auto px-0" onClick={onSelectAllPages}>
                  {t.selectAllPages(totalCount!)}
                </Button>
              )}
              {bulkActions?.(selected)}
              <Button variant="ghost" size="sm" onClick={() => onSelectionChange?.(new Set())}>
                {t.clearSelection}
              </Button>
            </div>
          ) : (
            <span />
          )}
          {columnsKey && (
            <Popover>
              <PopoverTrigger asChild>
                <Button variant="outline" size="sm">
                  <Columns3 aria-hidden /> {t.columns}
                </Button>
              </PopoverTrigger>
              <PopoverContent align="end" className="w-56 p-2">
                <ul className="flex flex-col">
                  {columns
                    .filter((c) => c.header !== "")
                    .map((c) => (
                      <li key={colId(c)}>
                        <label className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent">
                          <Checkbox checked={!hidden.has(colId(c))} onCheckedChange={() => toggleColumn(colId(c))} />
                          {c.header}
                        </label>
                      </li>
                    ))}
                </ul>
              </PopoverContent>
            </Popover>
          )}
        </div>
      )}
      <div className="overflow-auto" style={{ maxHeight }} role="region" aria-label={caption ?? t.region} tabIndex={0}>
        <table role="table" className="w-full border-collapse max-md:block md:min-w-[640px]">
          {caption && <caption className="sr-only">{caption}</caption>}
          <thead role="rowgroup" className={cn("max-md:sr-only", stickyHeader && "md:sticky md:top-0 md:z-10")}>
            <tr role="row" className="border-b border-border bg-background">
              {selectable && (
                <th role="columnheader" scope="col" className="w-10 py-(--row-py) pl-5">
                  <CellCheckbox label={t.selectAllRows} checked={allSelected ? true : someSelected ? "indeterminate" : false} onChange={toggleAll} />
                </th>
              )}
              {visible.map((col) => {
                const dir = col.sortKey && sort?.key === col.sortKey ? sort.dir : null;
                const SortIcon = dir === "asc" ? ArrowUp : dir === "desc" ? ArrowDown : ArrowUpDown;
                return (
                  <th
                    key={colId(col)}
                    role="columnheader"
                    scope="col"
                    aria-sort={dir ? (dir === "asc" ? "ascending" : "descending") : undefined}
                    style={col.width !== undefined ? { width: col.width } : undefined}
                    className={cn(
                      "px-4 py-(--row-py) text-xs font-semibold tracking-[0.02em] whitespace-nowrap text-muted-foreground md:first:pl-5 md:last:pr-5",
                      ALIGN[col.align ?? "left"],
                    )}
                  >
                    {col.sortKey && onSortChange ? (
                      <button
                        type="button"
                        onClick={() => onSortChange({ key: col.sortKey!, dir: dir === "asc" ? "desc" : "asc" })}
                        className={cn("inline-flex items-center gap-1 rounded-sm hover:text-foreground", FOCUS_RING, col.align === "right" && "flex-row-reverse")}
                      >
                        {col.header}
                        <SortIcon size={12} aria-hidden className={dir ? "text-foreground" : "opacity-50"} />
                      </button>
                    ) : (
                      col.header
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody role="rowgroup" className="max-md:block">
            {error && (
              <tr role="row" className="max-md:block">
                <td role="cell" colSpan={span} className="max-md:block">
                  <ErrorState message={error} onRetry={onRetry} className="m-4" />
                </td>
              </tr>
            )}
            {showSkeleton && (
              <>
                <tr role="row" className="sr-only">
                  <td role="cell" colSpan={span} aria-live="polite">
                    {t.loading}
                  </td>
                </tr>
                {Array.from({ length: 5 }, (_, i) => (
                  <tr key={i} role="row" aria-hidden className="border-b border-border max-md:block max-md:px-4 max-md:py-3">
                    {selectable && (
                      <td role="cell" className="w-10 py-(--row-py) pl-5 max-md:hidden">
                        <Skeleton className="size-4" />
                      </td>
                    )}
                    {visible.map((col) => (
                      <td key={colId(col)} role="cell" className="px-4 py-(--row-py) md:first:pl-5 md:last:pr-5 max-md:block max-md:px-0 max-md:py-1">
                        <Skeleton className={cn("h-4", i % 2 ? "w-2/3" : "w-3/4", col.align === "right" && "ml-auto w-1/2")} />
                      </td>
                    ))}
                  </tr>
                ))}
              </>
            )}
            {showEmpty && (
              <tr role="row" className="max-md:block">
                <td role="cell" colSpan={span} className="max-md:block">
                  <EmptyState title={emptyTitle} description={emptyDescription} action={emptyAction} />
                </td>
              </tr>
            )}
            {!error &&
              rows.map((row) => {
                const key = rowKey(row);
                const isSelected = selected.has(key);
                return (
                  <tr
                    key={key}
                    role="row"
                    data-state={isSelected ? "selected" : undefined}
                    onClick={onRowClick ? (e) => rowClick(e, row) : undefined}
                    onKeyDown={onRowClick ? (e) => rowKeyDown(e, row) : undefined}
                    tabIndex={onRowClick ? 0 : undefined}
                    aria-label={onRowClick && rowLabel ? rowLabel(row) : undefined}
                    className={cn(
                      "border-b border-border transition-colors last:border-b-0 hover:bg-background focus-visible:bg-accent focus-visible:outline-solid focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring",
                      "max-md:relative max-md:flex max-md:flex-col max-md:px-4 max-md:py-3",
                      selectable && "max-md:pr-14",
                      isSelected && "bg-accent/60",
                      onRowClick && "cursor-pointer",
                    )}
                  >
                    {selectable && (
                      <td role="cell" className="w-10 py-(--row-py) pl-5 max-md:absolute max-md:top-3 max-md:right-4 max-md:w-auto max-md:p-0">
                        <CellCheckbox label={t.selectRow} checked={isSelected} onChange={(v) => toggleOne(key, v)} />
                      </td>
                    )}
                    {visible.map((col) => {
                      const content = col.link ? (
                        <Link
                          to={col.link(row)}
                          className={cn("rounded-sm font-medium text-foreground underline-offset-4 hover:underline focus-visible:underline", FOCUS_RING)}
                        >
                          {col.render(row)}
                        </Link>
                      ) : (
                        col.render(row)
                      );
                      return (
                        <td
                          key={colId(col)}
                          role="cell"
                          className={cn(
                            "px-4 py-(--row-py) text-sm text-foreground md:first:pl-5 md:last:pr-5",
                            ALIGN[col.align ?? "left"],
                            col.align === "right" && "tabular",
                            // One phone layout per cell: hidden, the card's title, an unlabeled
                            // (actions) cell at the right, or "label · value".
                            col.mobileHidden
                              ? "max-md:hidden"
                              : col.link
                                ? "max-md:order-first max-md:block max-md:px-0 max-md:py-1 max-md:text-base"
                                : col.header === ""
                                  ? "max-md:block max-md:px-0 max-md:py-1 max-md:text-right"
                                  : CELL_MOBILE,
                          )}
                        >
                          {!col.link && col.header !== "" && (
                            <span aria-hidden className={LABEL_MOBILE}>
                              {col.header}
                            </span>
                          )}
                          {col.link ? content : <span className={cn("min-w-0", col.align === "right" && "max-md:justify-self-end")}>{content}</span>}
                        </td>
                      );
                    })}
                  </tr>
                );
              })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
