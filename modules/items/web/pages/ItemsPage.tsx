import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { FolderTree, PackagePlus, Plus, Ruler, Settings2 } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { FormField } from "@/shared/components/ui/form-field";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { Pagination } from "@/shared/components/ui/pagination";
import { SearchInput } from "@/shared/components/ui/search-input";
import { Segmented } from "@/shared/components/ui/segmented";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { Toolbar } from "@/shared/components/ui/toolbar";
import type { ActiveFilter } from "@/shared/components/ui/toolbar";
import { useCan } from "@/shared/access";
import { useResource } from "@/shared/hooks/use-resource";
import { CategoriesDialog } from "@/modules/items/components/CategoriesDialog";
import { CodeSettingsDialog } from "@/modules/items/components/CodeSettingsDialog";
import { ItemFormSheet } from "@/modules/items/components/ItemFormSheet";
import { ActiveBadge } from "@/modules/items/components/ItemStatusBadge";
import { UnitsDialog } from "@/modules/items/components/UnitsDialog";
import { fetchCategories, fetchUnits, listItems } from "@/modules/items/services/items.service";
import { KIND_LABEL } from "@/modules/items/types/item";
import type { Item, ItemKind, ItemStatus } from "@/modules/items/types/item";

const STATUSES: { value: ItemStatus; label: string }[] = [
  { value: "active", label: "Aktif" },
  { value: "inactive", label: "Nonaktif" },
  { value: "", label: "Semua" },
];

export default function ItemsPage() {
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<ItemStatus>("active");
  const [kind, setKind] = useState<ItemKind | "">("");
  const [categoryId, setCategoryId] = useState<number | null>(null);
  const [creating, setCreating] = useState(false);
  const [dialog, setDialog] = useState<"units" | "categories" | "settings" | null>(null);
  const canManage = useCan("items.manage");

  const items = useResource((signal) => listItems({ page, limit, q: query, status, kind, category_id: categoryId }, signal), [page, limit, query, status, kind, categoryId]);
  const units = useResource((signal) => fetchUnits(signal), []);
  const categories = useResource((signal) => fetchCategories(signal), []);
  const rows = items.data?.data ?? [];
  const meta = items.data?.meta ?? { page, limit, total: 0, total_pages: 0 };
  const categoryName = categories.data?.find((c) => c.id === categoryId)?.name;

  const filter = <T,>(set: (v: T) => void) => (v: T) => {
    setPage(1);
    set(v);
  };
  const active: ActiveFilter[] = [
    ...(query ? [{ key: "q", label: `Cari: ${query}`, onRemove: () => filter(setQuery)("") }] : []),
    ...(kind ? [{ key: "kind", label: `Jenis: ${KIND_LABEL[kind]}`, onRemove: () => filter(setKind)("") }] : []),
    ...(categoryId ? [{ key: "category", label: `Kategori: ${categoryName ?? categoryId}`, onRemove: () => filter(setCategoryId)(null) }] : []),
  ];
  const filtered = active.length > 0;

  const columns: DataTableColumn<Item>[] = [
    { header: "Kode", width: 140, link: (it) => `/items/${it.id}`, render: (it) => <span className="font-mono text-sm">{it.code}</span> },
    {
      header: "Nama",
      render: (it) => (
        <div className="min-w-0">
          <div className="truncate font-medium">{it.name}</div>
          {it.barcode && <div className="truncate font-mono text-xs text-muted-foreground">{it.barcode}</div>}
        </div>
      ),
    },
    {
      header: "Jenis",
      width: 150,
      render: (it) => (
        <span className="inline-flex flex-wrap items-center gap-1.5">
          <StatusBadge tone={it.kind === "goods" ? "info" : "neutral"}>{KIND_LABEL[it.kind]}</StatusBadge>
          {it.kind === "goods" && !it.is_stocked && <span className="text-xs text-muted-foreground">tanpa stok</span>}
        </span>
      ),
    },
    { header: "Kategori", mobileHidden: true, render: (it) => it.category_name || <span className="text-muted-foreground">–</span> },
    { header: "Satuan", width: 100, render: (it) => <span className="font-mono text-sm">{it.base_unit_code}</span> },
    { header: "Kode Coretax", id: "tax_code", defaultHidden: true, render: (it) => <span className="font-mono text-sm">{it.tax_code}</span> },
    { header: "Status", width: 110, render: (it) => <ActiveBadge active={it.is_active} /> },
  ];

  return (
    <Page>
      <PageHeader
        title="Barang dan jasa"
        description="Yang dibeli, disimpan, atau dijual perusahaan ini. Barang yang sudah dipakai di dokumen dinonaktifkan, tidak dihapus."
        actions={
          <>
            <Button variant="outline" onClick={() => setDialog("units")}>
              <Ruler />
              Satuan
            </Button>
            <Button variant="outline" onClick={() => setDialog("categories")}>
              <FolderTree />
              Kategori
            </Button>
            {canManage && (
              <>
                <Button variant="outline" onClick={() => setDialog("settings")}>
                  <Settings2 />
                  Kode otomatis
                </Button>
                <Button onClick={() => setCreating(true)}>
                  <Plus />
                  Tambah barang
                </Button>
              </>
            )}
          </>
        }
      />
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <Toolbar
          plain
          search={<SearchInput value={query} placeholder="Cari kode, nama, atau barcode…" onSearch={filter(setQuery)} />}
          primary={<Segmented aria-label="Status" options={STATUSES} value={status} onChange={filter(setStatus)} />}
          more={
            <>
              <FormField label="Jenis">
                <NativeSelect value={kind} onChange={(e) => filter(setKind)(e.target.value as ItemKind | "")}>
                  <NativeSelectOption value="">Semua jenis</NativeSelectOption>
                  <NativeSelectOption value="goods">Barang</NativeSelectOption>
                  <NativeSelectOption value="service">Jasa</NativeSelectOption>
                </NativeSelect>
              </FormField>
              <FormField label="Kategori">
                <NativeSelect value={categoryId == null ? "" : String(categoryId)} onChange={(e) => filter(setCategoryId)(e.target.value ? Number(e.target.value) : null)}>
                  <NativeSelectOption value="">Semua kategori</NativeSelectOption>
                  {(categories.data ?? []).map((c) => (
                    <NativeSelectOption key={c.id} value={String(c.id)}>
                      {c.name}
                      {c.is_active ? "" : " (nonaktif)"}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </FormField>
            </>
          }
          moreActiveCount={(kind ? 1 : 0) + (categoryId ? 1 : 0)}
          active={active}
          onClearAll={
            active.length > 1
              ? () => {
                  setPage(1);
                  setQuery("");
                  setKind("");
                  setCategoryId(null);
                }
              : undefined
          }
        />
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(it) => it.id}
          loading={items.loading}
          error={items.error}
          onRetry={items.reload}
          onRowClick={(it) => navigate(`/items/${it.id}`)}
          rowLabel={(it) => `Buka ${it.code} ${it.name}`}
          caption="Daftar barang dan jasa"
          columnsKey="items.list"
          emptyTitle={filtered ? "Tidak ada yang cocok" : "Belum ada barang"}
          emptyDescription={filtered ? "Tidak ada barang untuk pencarian dan filter ini." : "Tambahkan barang atau jasa sebelum mencatat stok atau membuat dokumen."}
          emptyAction={
            filtered ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setPage(1);
                  setQuery("");
                  setKind("");
                  setCategoryId(null);
                }}
              >
                Hapus pencarian dan filter
              </Button>
            ) : (
              canManage &&
              status === "active" && (
                <Button size="sm" onClick={() => setCreating(true)}>
                  <PackagePlus />
                  Tambah barang pertama
                </Button>
              )
            )
          }
        />
        {!items.error && rows.length > 0 && (
          <Pagination
            page={meta.page}
            limit={meta.limit}
            total={meta.total}
            noun="barang"
            onPageChange={setPage}
            onLimitChange={(n) => {
              setPage(1);
              setLimit(n);
            }}
          />
        )}
      </section>
      <ItemFormSheet
        open={creating}
        onClose={() => setCreating(false)}
        units={units.data ?? []}
        categories={categories.data ?? []}
        onUnitsChanged={units.reload}
        onSaved={(it) => navigate(`/items/${it.id}`)}
      />
      <UnitsDialog
        open={dialog === "units"}
        onClose={() => setDialog(null)}
        units={units.data ?? []}
        loading={units.loading}
        error={units.error}
        onRetry={units.reload}
        canManage={canManage}
        onChanged={units.reload}
      />
      <CategoriesDialog
        open={dialog === "categories"}
        onClose={() => setDialog(null)}
        categories={categories.data ?? []}
        loading={categories.loading}
        error={categories.error}
        onRetry={categories.reload}
        canManage={canManage}
        onChanged={() => {
          categories.reload();
          items.reload();
        }}
      />
      <CodeSettingsDialog open={dialog === "settings"} onClose={() => setDialog(null)} />
    </Page>
  );
}
