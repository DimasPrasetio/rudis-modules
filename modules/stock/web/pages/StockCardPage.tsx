import { useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { DateRangePicker } from "@/shared/components/ui/date-range-picker";
import { ErrorState } from "@/shared/components/ui/error-state";
import { FormField } from "@/shared/components/ui/form-field";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SectionCard } from "@/shared/components/ui/section-card";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { useResource } from "@/shared/hooks/use-resource";
import { todayISO } from "@/shared/lib/format";
import { fetchCard, listWarehouses } from "@/modules/stock/services/stock.service";
import { KIND_SHORT, formatDay, formatQty, formatRp } from "@/modules/stock/types/stock";
import type { Move } from "@/modules/stock/types/stock";

const monthStart = (iso: string) => iso.slice(0, 8) + "01";

export default function StockCardPage() {
  const { id } = useParams();
  const itemId = Number(id);
  const [search] = useSearchParams();
  const [warehouse, setWarehouse] = useState(Number(search.get("warehouse_id") ?? 0));
  const today = todayISO();
  const [range, setRange] = useState({ from: monthStart(today), to: today });

  const warehouses = useResource((signal) => listWarehouses(signal), []);
  const card = useResource((signal) => fetchCard(itemId, { warehouse_id: warehouse, ...range }, signal), [itemId, warehouse, range.from, range.to]);
  const c = card.data;
  const title = c ? `Kartu stok — ${c.item.name}` : "Kartu stok";
  const unit = c?.item.base_unit ?? "";

  const columns: DataTableColumn<Move>[] = [
    { header: "Tanggal", width: 110, render: (m) => <span className="tabular">{formatDay(m.date)}</span> },
    {
      header: "Dokumen",
      render: (m) =>
        m.document_id ? (
          <div className="flex flex-wrap items-center gap-2">
            <Link className="font-mono text-sm text-primary underline-offset-4 hover:underline" to={`/stock/documents/${m.document_id}`}>
              {m.document_number || `#${m.document_id}`}
            </Link>
            <span className="text-xs text-muted-foreground">{m.document_kind ? KIND_SHORT[m.document_kind] : ""}</span>
            {m.is_reversal && <StatusBadge tone="muted">Pembatalan</StatusBadge>}
          </div>
        ) : (
          <span className="font-mono text-sm text-muted-foreground">{m.source}</span>
        ),
    },
    ...(warehouse ? [] : [{ header: "Gudang", width: 110, render: (m: Move) => <span className="font-mono text-sm">{m.warehouse_code}</span> }]),
    { header: "Masuk", width: 110, align: "right", render: (m) => <span className="tabular">{m.in && /[1-9]/.test(m.in) ? formatQty(m.in) : ""}</span> },
    { header: "Keluar", width: 110, align: "right", render: (m) => <span className="tabular">{m.out && /[1-9]/.test(m.out) ? formatQty(m.out) : ""}</span> },
    { header: "Saldo", width: 110, align: "right", render: (m) => <span className="tabular font-medium">{formatQty(m.qty_after)}</span> },
    { header: "Nilai mutasi", width: 150, align: "right", mobileHidden: true, render: (m) => <span className="tabular">{formatRp(m.value)}</span> },
    { header: "Nilai saldo", width: 150, align: "right", render: (m) => <span className="tabular">{formatRp(m.value_after)}</span> },
  ];

  return (
    <Page>
      <PageHeader
        title={title}
        subtitle={c ? `${c.item.code} · satuan dasar ${unit}` : undefined}
        crumbs={[{ label: "Stok", to: "/stock" }, { label: c?.item.code ?? "Kartu stok" }]}
        description="Setiap mutasi barang ini menurut tanggal. Pembatalan tampil sebagai baris pembalik; saldo dihitung per gudang."
      />
      <SectionCard>
        <div className="grid gap-4 md:grid-cols-[16rem_minmax(0,1fr)]">
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
          <DateRangePicker
            from={range.from}
            to={range.to}
            onChange={(r) => r.from && r.to && setRange({ from: r.from, to: r.to })}
            fromName="from"
            toName="to"
          />
        </div>
      </SectionCard>
      {card.error ? (
        <ErrorState message={card.error} onRetry={card.reload} />
      ) : (
        <SectionCard
          title="Mutasi"
          flush
          description={
            c && (
              <span>
                Saldo awal {formatDay(c.from)}:{" "}
                {c.openings.length === 0
                  ? `0 ${unit}`
                  : c.openings
                      .map(
                        (o) =>
                          `${formatQty(o.qty)} ${unit} (${formatRp(o.value)})${warehouse ? "" : ` di ${warehouses.data?.find((w) => w.id === o.warehouse_id)?.code ?? "#" + o.warehouse_id}`}`,
                      )
                      .join(", ")}
              </span>
            )
          }
        >
          <DataTable
            columns={columns}
            rows={c?.moves}
            rowKey={(m) => m.id}
            loading={card.loading}
            caption={`Mutasi ${c?.item.name ?? ""}`}
            emptyTitle="Tidak ada mutasi"
            emptyDescription="Tidak ada barang masuk atau keluar pada periode ini."
          />
        </SectionCard>
      )}
    </Page>
  );
}
