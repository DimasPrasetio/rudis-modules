import { useState } from "react";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { DateRangePicker } from "@/shared/components/ui/date-range-picker";
import { FormField } from "@/shared/components/ui/form-field";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SearchInput } from "@/shared/components/ui/search-input";
import { SectionCard } from "@/shared/components/ui/section-card";
import { useResource } from "@/shared/hooks/use-resource";
import { todayISO } from "@/shared/lib/format";
import { fetchReport, listWarehouses } from "@/modules/stock/services/stock.service";
import { formatDay, formatQty, formatRp } from "@/modules/stock/types/stock";
import type { ReportRow } from "@/modules/stock/types/stock";

const qtyCell = (q: string, v: string) => (
  <div className="tabular">
    <div>{formatQty(q)}</div>
    <div className="text-xs text-muted-foreground">{formatRp(v)}</div>
  </div>
);

export default function MovementReportPage() {
  const today = todayISO();
  const [range, setRange] = useState({ from: today.slice(0, 8) + "01", to: today });
  const [warehouse, setWarehouse] = useState(0);
  const [query, setQuery] = useState("");
  const warehouses = useResource((signal) => listWarehouses(signal), []);
  const report = useResource((signal) => fetchReport({ warehouse_id: warehouse, q: query, ...range }, signal), [warehouse, query, range.from, range.to]);

  const columns: DataTableColumn<ReportRow>[] = [
    {
      header: "Barang",
      link: (r) => `/stock/items/${r.item_id}?warehouse_id=${r.warehouse_id}`,
      render: (r) => (
        <div className="min-w-0">
          <div className="truncate font-medium">{r.item_name}</div>
          <div className="font-mono text-xs text-muted-foreground">
            {r.item_code} · {r.base_unit}
          </div>
        </div>
      ),
    },
    { header: "Gudang", width: 110, render: (r) => <span className="font-mono text-sm">{r.warehouse_code}</span> },
    { header: "Awal", width: 140, align: "right", render: (r) => qtyCell(r.opening_qty, r.opening_value) },
    { header: "Masuk", width: 140, align: "right", render: (r) => qtyCell(r.in_qty, r.in_value) },
    { header: "Keluar", width: 140, align: "right", render: (r) => qtyCell(r.out_qty, r.out_value) },
    { header: "Akhir", width: 140, align: "right", render: (r) => <div className="font-medium">{qtyCell(r.closing_qty, r.closing_value)}</div> },
  ];

  return (
    <Page>
      <PageHeader
        title="Laporan mutasi stok"
        description={`Stok awal, masuk, keluar, dan akhir per barang dan gudang, ${formatDay(range.from)} – ${formatDay(range.to)}. Dokumen yang dibatalkan tidak dihitung sebagai masuk atau keluar.`}
      />
      <SectionCard>
        <div className="grid gap-4 lg:grid-cols-[16rem_16rem_minmax(0,1fr)]">
          <FormField label="Gudang" name="warehouse_id">
            <NativeSelect value={String(warehouse)} onChange={(e) => setWarehouse(Number(e.target.value))}>
              <NativeSelectOption value="0">Semua gudang</NativeSelectOption>
              {(warehouses.data ?? []).map((w) => (
                <NativeSelectOption key={w.id} value={String(w.id)}>
                  {w.code} — {w.name}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </FormField>
          <FormField label="Barang" name="q">
            <SearchInput value={query} placeholder="Kode atau nama…" onSearch={setQuery} />
          </FormField>
          <DateRangePicker
            from={range.from}
            to={range.to}
            onChange={(r) => r.from && r.to && setRange({ from: r.from, to: r.to })}
            fromName="from"
            toName="to"
          />
        </div>
      </SectionCard>
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <DataTable
          columns={columns}
          rows={report.data}
          rowKey={(r) => `${r.item_id}-${r.warehouse_id}`}
          loading={report.loading}
          error={report.error}
          onRetry={report.reload}
          caption="Laporan mutasi stok"
          emptyTitle="Tidak ada mutasi"
          emptyDescription="Belum ada barang yang bergerak sampai akhir periode ini."
        />
      </section>
    </Page>
  );
}
