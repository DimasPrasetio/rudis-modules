import { useState } from "react";
import { Link } from "react-router-dom";
import { PackageMinus, PackagePlus, SlidersHorizontal } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { Pagination } from "@/shared/components/ui/pagination";
import { SearchInput } from "@/shared/components/ui/search-input";
import { Segmented } from "@/shared/components/ui/segmented";
import { Toolbar } from "@/shared/components/ui/toolbar";
import { useCan } from "@/shared/access";
import { useResource } from "@/shared/hooks/use-resource";
import { DocumentStatusBadge } from "@/modules/stock/components/DocumentStatusBadge";
import { listDocuments, listWarehouses } from "@/modules/stock/services/stock.service";
import { KIND_SHORT, STATUS_LABEL, formatDay, formatRp } from "@/modules/stock/types/stock";
import type { DocKind, DocStatus, StockDocument } from "@/modules/stock/types/stock";

const KINDS: { value: DocKind | ""; label: string }[] = [
  { value: "", label: "Semua" },
  { value: "receipt", label: "Penerimaan" },
  { value: "issue", label: "Pengeluaran" },
  { value: "adjustment", label: "Penyesuaian" },
];

export default function DocumentsPage() {
  const canDraft = useCan("stock.draft");
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState<DocKind | "">("");
  const [status, setStatus] = useState<DocStatus | "">("");
  const [warehouse, setWarehouse] = useState(0);

  const warehouses = useResource((signal) => listWarehouses(signal), []);
  const docs = useResource(
    (signal) => listDocuments({ page, limit, q: query, kind, status, warehouse_id: warehouse }, signal),
    [page, limit, query, kind, status, warehouse],
  );
  const rows = docs.data?.data ?? [];
  const meta = docs.data?.meta ?? { page, limit, total: 0, total_pages: 0 };
  const reset = () => setPage(1);

  const columns: DataTableColumn<StockDocument>[] = [
    {
      header: "Nomor",
      width: 200,
      link: (d) => `/stock/documents/${d.id}`,
      render: (d) => (d.number ? <span className="font-mono text-sm">{d.number}</span> : <span className="text-muted-foreground">Draf #{d.id}</span>),
    },
    { header: "Tanggal", width: 120, render: (d) => <span className="tabular">{formatDay(d.date)}</span> },
    { header: "Jenis", width: 130, render: (d) => KIND_SHORT[d.kind] },
    { header: "Gudang", width: 120, mobileHidden: true, render: (d) => <span className="font-mono text-sm">{d.warehouse_code}</span> },
    {
      header: "Dari / kepada",
      mobileHidden: true,
      render: (d) => (
        <div className="min-w-0">
          <div className="truncate">{d.counterparty || <span className="text-muted-foreground">–</span>}</div>
          {d.reference && <div className="truncate text-xs text-muted-foreground">Ref. {d.reference}</div>}
        </div>
      ),
    },
    { header: "Nilai", width: 150, align: "right", render: (d) => <span className="tabular">{d.status === "draft" ? "–" : formatRp(d.total_value)}</span> },
    { header: "Status", width: 120, render: (d) => <DocumentStatusBadge status={d.status} /> },
  ];

  return (
    <Page>
      <PageHeader
        title="Dokumen stok"
        description="Penerimaan, pengeluaran, dan penyesuaian. Dokumen yang sudah diposting tidak diubah — dibatalkan dengan alasan, lalu dikoreksi."
        actions={
          canDraft && (
            <>
              <Button variant="outline" asChild>
                <Link to="/stock/documents/new?kind=adjustment">
                  <SlidersHorizontal />
                  Penyesuaian
                </Link>
              </Button>
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
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <Toolbar
          plain
          search={
            <SearchInput
              value={query}
              placeholder="Cari nomor, pihak, atau referensi…"
              onSearch={(q) => {
                reset();
                setQuery(q);
              }}
            />
          }
          primary={
            <Segmented
              aria-label="Jenis dokumen"
              options={KINDS}
              value={kind}
              onChange={(k) => {
                reset();
                setKind(k);
              }}
            />
          }
          more={
            <div className="grid gap-3">
              <NativeSelect
                aria-label="Status"
                value={status}
                onChange={(e) => {
                  reset();
                  setStatus(e.target.value as DocStatus | "");
                }}
              >
                <NativeSelectOption value="">Semua status</NativeSelectOption>
                {(Object.keys(STATUS_LABEL) as DocStatus[]).map((s) => (
                  <NativeSelectOption key={s} value={s}>
                    {STATUS_LABEL[s]}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
              <NativeSelect
                aria-label="Gudang"
                value={String(warehouse)}
                onChange={(e) => {
                  reset();
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
            </div>
          }
          moreActiveCount={(status ? 1 : 0) + (warehouse ? 1 : 0)}
          active={[
            ...(query ? [{ key: "q", label: `Cari: ${query}`, onRemove: () => setQuery("") }] : []),
            ...(status ? [{ key: "status", label: `Status: ${STATUS_LABEL[status]}`, onRemove: () => setStatus("") }] : []),
            ...(warehouse
              ? [{ key: "wh", label: `Gudang: ${warehouses.data?.find((w) => w.id === warehouse)?.code ?? warehouse}`, onRemove: () => setWarehouse(0) }]
              : []),
          ]}
          onClearAll={() => {
            setQuery("");
            setStatus("");
            setWarehouse(0);
          }}
        />
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(d) => d.id}
          loading={docs.loading}
          error={docs.error}
          onRetry={docs.reload}
          caption="Daftar dokumen stok"
          emptyTitle={query || status || warehouse ? "Tidak ada yang cocok" : "Belum ada dokumen"}
          emptyDescription={query || status || warehouse ? "Ubah pencarian atau filter." : "Catat barang masuk atau keluar untuk memulai."}
        />
        {!docs.error && rows.length > 0 && (
          <Pagination
            page={meta.page}
            limit={meta.limit}
            total={meta.total}
            noun="dokumen"
            onPageChange={setPage}
            onLimitChange={(n) => {
              reset();
              setLimit(n);
            }}
          />
        )}
      </section>
    </Page>
  );
}
