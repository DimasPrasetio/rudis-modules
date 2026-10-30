import { useState } from "react";
import { useParams } from "react-router-dom";
import { Pencil, Plus, Power, Ruler } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { ActionMenu } from "@/shared/components/ui/action-menu";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { DescriptionList } from "@/shared/components/ui/description-list";
import { ErrorState } from "@/shared/components/ui/error-state";
import { Notice } from "@/shared/components/ui/notice";
import { Page, PageColumns } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SectionCard } from "@/shared/components/ui/section-card";
import { DetailSkeleton } from "@/shared/components/ui/skeletons";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { useCan } from "@/shared/access";
import { useResource } from "@/shared/hooks/use-resource";
import { formatDateTime } from "@/shared/lib/format";
import { notify } from "@/shared/lib/notify";
import { ItemFormSheet } from "@/modules/items/components/ItemFormSheet";
import { ActiveBadge } from "@/modules/items/components/ItemStatusBadge";
import { ItemUnitDialog } from "@/modules/items/components/ItemUnitDialog";
import { deactivateItemUnit, fetchCategories, fetchItem, fetchUnits, setItemActive } from "@/modules/items/services/items.service";
import { KIND_LABEL, formatFactor } from "@/modules/items/types/item";
import type { ItemUnit } from "@/modules/items/types/item";

export default function ItemPage() {
  const id = Number(useParams().id);
  const canManage = useCan("items.manage");
  const detail = useResource((signal) => fetchItem(id, signal), [id]);
  const units = useResource((signal) => fetchUnits(signal), []);
  const categories = useResource((signal) => fetchCategories(signal), []);
  const [editing, setEditing] = useState(false);
  const [toggling, setToggling] = useState(false);
  const [addingUnit, setAddingUnit] = useState(false);
  const [removing, setRemoving] = useState<ItemUnit | null>(null);

  if (detail.error) {
    return (
      <Page>
        <PageHeader title="Barang" crumbs={[{ label: "Barang", to: "/items" }, { label: "Tidak ditemukan" }]} />
        <ErrorState message={detail.error} onRetry={detail.reload} />
      </Page>
    );
  }
  if (!detail.data) {
    return (
      <Page>
        <DetailSkeleton />
      </Page>
    );
  }

  const { item: it, units: alternatives } = detail.data;
  const base = it.base_unit_code;

  const unitColumns: DataTableColumn<ItemUnit>[] = [
    {
      header: "Satuan",
      render: (u) => (
        <span>
          <span className="font-mono text-sm">{u.code}</span> <span className="text-muted-foreground">{u.name}</span>
        </span>
      ),
    },
    {
      header: "Konversi",
      render: (u) => (
        <span className="tabular">
          1 {u.code} = {formatFactor(u.factor)} {base}
        </span>
      ),
    },
    { header: "Status", width: 110, render: (u) => <ActiveBadge active={u.is_active} /> },
    ...(canManage
      ? [
          {
            header: "Aksi",
            id: "actions",
            width: 140,
            align: "right" as const,
            render: (u: ItemUnit) =>
              u.is_active ? (
                <Button variant="ghost" size="sm" onClick={() => setRemoving(u)} aria-label={`Nonaktifkan satuan ${u.code}`}>
                  Nonaktifkan
                </Button>
              ) : null,
          },
        ]
      : []),
  ];

  return (
    <Page>
      <PageHeader
        title={it.name}
        crumbs={[{ label: "Barang", to: "/items" }, { label: it.name }]}
        meta={
          <span className="inline-flex flex-wrap items-center gap-2">
            <span className="font-mono">{it.code}</span>
            <StatusBadge tone={it.kind === "goods" ? "info" : "neutral"}>{KIND_LABEL[it.kind]}</StatusBadge>
            <ActiveBadge active={it.is_active} />
          </span>
        }
        actions={
          canManage && (
            <>
              <Button variant="outline" onClick={() => setEditing(true)}>
                <Pencil />
                Ubah
              </Button>
              <ActionMenu
                label={`Aksi lain untuk ${it.name}`}
                items={[
                  { key: "toggle", label: it.is_active ? "Nonaktifkan" : "Aktifkan", icon: <Power />, destructive: it.is_active, onSelect: () => setToggling(true) },
                ]}
              />
            </>
          )
        }
      />
      {!it.is_active && (
        <Notice tone="info" title={`${KIND_LABEL[it.kind]} nonaktif`}>
          Dokumen lama tetap menampilkannya; tidak bisa dipakai di dokumen baru sampai diaktifkan lagi.
        </Notice>
      )}

      <PageColumns
        main={
          <>
            <SectionCard title="Data barang">
              <DescriptionList
                columns={2}
                items={[
                  { label: "Jenis", value: KIND_LABEL[it.kind] },
                  { label: "Stok", value: it.is_stocked ? "Dilacak" : it.kind === "service" ? "Jasa — tanpa stok" : "Tidak dilacak" },
                  {
                    label: "Satuan dasar",
                    value: (
                      <span>
                        <span className="font-mono">{base}</span> {it.base_unit_name}
                      </span>
                    ),
                  },
                  { label: "Kategori", value: it.category_name || "–" },
                  { label: "Barcode", value: it.barcode ? <span className="font-mono">{it.barcode}</span> : "–" },
                  {
                    label: "Kode barang/jasa Coretax",
                    value: it.tax_code === "000000" ? <span className="text-muted-foreground">000000 (tidak diisi)</span> : <span className="font-mono">{it.tax_code}</span>,
                  },
                  { label: "Keterangan", value: it.description || "–", wide: true },
                ]}
              />
            </SectionCard>

            <SectionCard
              title="Satuan alternatif"
              description={`Satuan lain untuk mencatat barang ini; stok tetap dihitung dalam ${base}. Faktor tidak bisa diubah — nonaktifkan lalu tambah yang baru.`}
              actions={
                canManage && (
                  <Button size="sm" variant="outline" onClick={() => setAddingUnit(true)}>
                    <Plus />
                    Satuan
                  </Button>
                )
              }
            >
              <DataTable
                columns={unitColumns}
                rows={alternatives}
                rowKey={(u) => u.id}
                caption={`Satuan alternatif ${it.name}`}
                emptyTitle="Belum ada satuan alternatif"
                emptyDescription={`Mis. 1 DUS = 12 ${base}. Barang dicatat dalam ${base} saja sampai satuan lain ditambahkan.`}
                emptyAction={
                  canManage && (
                    <Button size="sm" variant="outline" onClick={() => setAddingUnit(true)}>
                      <Ruler />
                      Tambah satuan alternatif
                    </Button>
                  )
                }
              />
            </SectionCard>
          </>
        }
        aside={
          <SectionCard title="Riwayat">
            <DescriptionList
              columns={1}
              items={[
                { label: "Dibuat", value: formatDateTime(it.created_at) },
                { label: "Diperbarui", value: formatDateTime(it.updated_at) },
              ]}
            />
          </SectionCard>
        }
      />

      <ItemFormSheet
        open={editing}
        onClose={() => setEditing(false)}
        item={it}
        units={units.data ?? []}
        categories={categories.data ?? []}
        onUnitsChanged={units.reload}
        onSaved={detail.reload}
      />
      <ItemUnitDialog
        open={addingUnit}
        onClose={() => setAddingUnit(false)}
        item={it}
        current={alternatives}
        units={units.data ?? []}
        onSaved={detail.reload}
      />
      <ConfirmDialog
        open={toggling}
        onOpenChange={setToggling}
        tone={it.is_active ? "danger" : "primary"}
        title={it.is_active ? `Nonaktifkan ${it.name}?` : `Aktifkan ${it.name}?`}
        description={
          it.is_active
            ? "Tidak bisa dipakai di dokumen baru. Dokumen lama tetap menampilkannya, dan bisa diaktifkan lagi."
            : "Bisa dipakai lagi di dokumen baru."
        }
        confirmLabel={it.is_active ? "Nonaktifkan" : "Aktifkan"}
        onConfirm={async () => {
          await setItemActive(it.id, !it.is_active);
          notify.success(it.is_active ? "Barang dinonaktifkan" : "Barang diaktifkan");
          detail.reload();
        }}
      />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        tone="danger"
        title={`Nonaktifkan satuan ${removing?.code}?`}
        description={`Dokumen baru tidak bisa mencatat ${it.name} dalam ${removing?.code}. Dokumen lama tetap memakai faktornya. Satuan ini bisa ditambahkan lagi dengan faktor yang sama.`}
        confirmLabel="Nonaktifkan"
        onConfirm={async () => {
          if (!removing) return;
          await deactivateItemUnit(it.id, removing.id);
          notify.success("Satuan dinonaktifkan");
          detail.reload();
        }}
      />
    </Page>
  );
}
