import { useState } from "react";
import { Link } from "react-router-dom";
import { PackageMinus, PackagePlus } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { Pagination } from "@/shared/components/ui/pagination";
import { SearchInput } from "@/shared/components/ui/search-input";
import { Segmented } from "@/shared/components/ui/segmented";
import { SummaryCard } from "@/shared/components/ui/summary-card";
import { Toolbar } from "@/shared/components/ui/toolbar";
import { useCan } from "@/shared/access";
import { useResource } from "@/shared/hooks/use-resource";
import { listBalances, listWarehouses } from "@/modules/stock/services/stock.service";
import { formatDay, formatQty, formatRp } from "@/modules/stock/types/stock";
import type { Balance } from "@/modules/stock/types/stock";

type Show = "with" | "all";

export default function StockPage() {
  const canDraft = useCan("stock.draft");
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [query, setQuery] = useState("");
  const [warehouse, setWarehouse] = useState(0);
  const [show, setShow] = useState<Show>("with");

  const warehouses = useResource((signal) => listWarehouses(signal), []);
  const stock = useResource(
    (signal) => listBalances({ page, limit, q: query, warehouse_id: warehouse, with_stock: show === "with" }, signal),
    [page, limit, query, warehouse, show],
  );
  const rows = stock.data?.data ?? [];
  const meta = stock.data?.meta ?? { page, limit, total: 0, total_pages: 0 };
  const whName = warehouses.data?.find((w) => w.id === warehouse)?.name;

  const columns: DataTableColumn<Balance>[] = [
    {
      header: "Barang",
      link: (b) => `/stock/items/${b.item_id}${warehouse ? `?warehouse_id=${warehouse}` : ""}`,
      render: (b) => (
        <div className="min-w-0">
          <div className="truncate font-medium">{b.item_name}</div>
          <div className="font-mono text-xs text-muted-foreground">{b.item_code}</div>
        </div>
      ),
    },
    { header: "Gudang", width: 140, render: (b) => <span className="font-mono text-sm">{b.warehouse_code}</span> },
    {
      header: "Stok",
      width: 140,
      align: "right",
      render: (b) => (
        <span className="tabular">
          {formatQty(b.qty)} <span className="text-muted-foreground">{b.base_unit}</span>
        </span>
      ),
    },
    { header: "Biaya rata-rata", width: 150, align: "right", mobileHidden: true, render: (b) => <span className="tabular">{formatRp(b.unit_cost)}</span> },
    { header: "Nilai", width: 160, align: "right", render: (b) => <span className="tabular font-medium">{formatRp(b.value)}</span> },
    { header: "Mutasi terakhir", id: "last", width: 140, defaultHidden: true, render: (b) => formatDay(b.last_date) },
  ];

  return (
    <Page>
      <PageHeader
        title="Stok"
        description="Jumlah dan nilai setiap barang per gudang, dalam satuan dasarnya. Buka barang untuk melihat kartu stoknya."
        actions={
          canDraft && (
            <>
              <Button variant="outline" asChild>
                <Link to="/stock/documents/new?kind=issue">
                  <PackageMinus />
                  Barang keluar
                </Link>
              </Button>
              <Button asChild>
                <Link to="/stock/documents/new?kind=receipt">
                  <PackagePlus />
                  Barang masuk
                </Link>
              </Button>
            </>
          )
        }
      />
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <SummaryCard label={whName ? `Nilai persediaan — ${whName}` : "Nilai persediaan"} value={stock.data ? formatRp(stock.data.total_value) : "–"} />
      </div>
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <Toolbar
          plain
          search={
            <SearchInput
              value={query}
              placeholder="Cari kode atau nama barang…"
              onSearch={(q) => {
                setPage(1);
                setQuery(q);
              }}
            />
          }
          primary={
            <>
              <NativeSelect
                aria-label="Gudang"
                value={String(warehouse)}
                onChange={(e) => {
                  setPage(1);
                  setWarehouse(Number(e.target.value));
                }}
              >
                <NativeSelectOption value="0">Semua gudang</NativeSelectOption>
                {(warehouses.data ?? []).map((w) => (
                  <NativeSelectOption key={w.id} value={String(w.id)}>
                    {w.code} — {w.name}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
              <Segmented
                aria-label="Tampilkan"
                options={[
                  { value: "with", label: "Ada stok" },
                  { value: "all", label: "Semua" },
                ]}
                value={show}
                onChange={(v) => {
                  setPage(1);
                  setShow(v);
                }}
              />
            </>
          }
          active={query ? [{ key: "q", label: `Cari: ${query}`, onRemove: () => setQuery("") }] : []}
        />
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(b) => `${b.item_id}-${b.warehouse_id}`}
          loading={stock.loading}
          error={stock.error}
          onRetry={stock.reload}
          caption="Stok per barang dan gudang"
          emptyTitle={query ? "Tidak ada yang cocok" : "Belum ada stok"}
          emptyDescription={query ? `Tidak ada barang untuk "${query}".` : "Catat barang masuk untuk mulai melihat stok di sini."}
          emptyAction={
            !query &&
            canDraft && (
              <Button size="sm" asChild>
                <Link to="/stock/documents/new?kind=receipt">
                  <PackagePlus />
                  Catat barang masuk
                </Link>
              </Button>
            )
          }
        />
        {!stock.error && rows.length > 0 && (
          <Pagination
            page={meta.page}
            limit={meta.limit}
            total={meta.total}
            noun="baris"
            onPageChange={setPage}
            onLimitChange={(n) => {
              setPage(1);
              setLimit(n);
            }}
          />
        )}
      </section>
    </Page>
  );
}
