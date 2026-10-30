import { useState } from "react";
import { Lock, Pencil, Plus, Power } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { FormField } from "@/shared/components/ui/form-field";
import { Notice } from "@/shared/components/ui/notice";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { SectionCard } from "@/shared/components/ui/section-card";
import { StatusBadge } from "@/shared/components/ui/status-badge";
import { useCan } from "@/shared/access";
import { useResource } from "@/shared/hooks/use-resource";
import { notify } from "@/shared/lib/notify";
import { WarehouseDialog } from "@/modules/stock/components/WarehouseDialog";
import { fetchSettings, listBranches, listWarehouses, saveCostMethod, setWarehouseActive } from "@/modules/stock/services/stock.service";
import { METHOD_LABEL } from "@/modules/stock/types/stock";
import type { CostMethod, Warehouse } from "@/modules/stock/types/stock";

export default function WarehousesPage() {
  const canManage = useCan("stock.manage");
  const warehouses = useResource((signal) => listWarehouses(signal), []);
  const branches = useResource((signal) => listBranches(signal), []);
  const settings = useResource((signal) => fetchSettings(signal), []);
  const [editing, setEditing] = useState<Warehouse | null>(null);
  const [creating, setCreating] = useState(false);
  const [toggling, setToggling] = useState<Warehouse | null>(null);
  const [method, setMethod] = useState<CostMethod | null>(null);
  const [savingMethod, setSavingMethod] = useState(false);

  const branchList = branches.data ?? [];
  const branchLabel = (id: number) => (id === 0 ? "Seluruh perusahaan" : (branchList.find((b) => b.id === id)?.name ?? `Cabang #${id}`));

  const columns: DataTableColumn<Warehouse>[] = [
    { header: "Kode", width: 160, render: (w) => <span className="font-mono text-sm">{w.code}</span> },
    { header: "Nama", render: (w) => <span className="font-medium">{w.name}</span> },
    { header: "Cabang", mobileHidden: branchList.length === 0, render: (w) => <span className="text-muted-foreground">{branchLabel(w.branch_id)}</span> },
    { header: "Status", width: 120, render: (w) => <StatusBadge tone={w.is_active ? "success" : "muted"}>{w.is_active ? "Aktif" : "Nonaktif"}</StatusBadge> },
    {
      header: "Aksi",
      width: 220,
      align: "right",
      render: (w) =>
        canManage && (
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={() => setEditing(w)} aria-label={`Ubah gudang ${w.code}`}>
              <Pencil />
              Ubah
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setToggling(w)} aria-label={`${w.is_active ? "Nonaktifkan" : "Aktifkan"} gudang ${w.code}`}>
              <Power />
              {w.is_active ? "Nonaktifkan" : "Aktifkan"}
            </Button>
          </div>
        ),
    },
  ];

  const st = settings.data;
  const chosen = method ?? st?.cost_method ?? "average";

  async function saveMethod() {
    setSavingMethod(true);
    try {
      await saveCostMethod(chosen);
      notify.success("Metode biaya disimpan");
      setMethod(null);
      settings.reload();
    } catch (err) {
      notify.error(err);
    } finally {
      setSavingMethod(false);
    }
  }

  return (
    <Page>
      <PageHeader
        title="Gudang"
        description="Tempat stok disimpan, dan cara persediaan dinilai. Gudang yang sudah dipakai dinonaktifkan, tidak dihapus."
        actions={
          canManage && (
            <Button onClick={() => setCreating(true)}>
              <Plus />
              Tambah gudang
            </Button>
          )
        }
      />
      <section className="overflow-hidden rounded-xl border border-border bg-card">
        <DataTable
          columns={columns}
          rows={warehouses.data}
          rowKey={(w) => w.id}
          loading={warehouses.loading}
          error={warehouses.error}
          onRetry={warehouses.reload}
          caption="Daftar gudang"
          emptyTitle="Belum ada gudang"
          emptyDescription="Tambahkan gudang sebelum mencatat barang masuk dan keluar."
          emptyAction={
            canManage && (
              <Button size="sm" onClick={() => setCreating(true)}>
                <Plus />
                Tambah gudang pertama
              </Button>
            )
          }
        />
      </section>

      <SectionCard
        title="Metode biaya persediaan"
        description="Menentukan nilai barang yang keluar. PSAK 202 dan UU PPh Pasal 10 ayat 6 hanya mengakui rata-rata atau FIFO, dan metode dipakai secara konsisten."
      >
        {st && (
          <div className="grid max-w-xl gap-4">
            {st.locked ? (
              <Notice tone="info" title={`Metode: ${METHOD_LABEL[st.cost_method]}`}>
                <span className="inline-flex items-center gap-1.5">
                  <Lock className="size-3.5" aria-hidden />
                  Terkunci karena stok sudah bergerak.
                </span>
              </Notice>
            ) : (
              <>
                <FormField label="Metode" name="cost_method" hint="Bisa diubah sampai ada mutasi stok pertama.">
                  <NativeSelect value={chosen} disabled={!st.can_manage} onChange={(e) => setMethod(e.target.value as CostMethod)}>
                    {(Object.keys(METHOD_LABEL) as CostMethod[]).map((m) => (
                      <NativeSelectOption key={m} value={m}>
                        {METHOD_LABEL[m]}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </FormField>
                {st.can_manage && (
                  <div>
                    <Button onClick={saveMethod} loading={savingMethod} disabled={chosen === st.cost_method}>
                      Simpan metode
                    </Button>
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </SectionCard>

      <WarehouseDialog
        open={creating || editing !== null}
        warehouse={editing ?? undefined}
        branches={branchList}
        onClose={() => {
          setCreating(false);
          setEditing(null);
        }}
        onSaved={warehouses.reload}
      />
      <ConfirmDialog
        open={toggling !== null}
        onOpenChange={(o) => !o && setToggling(null)}
        title={toggling?.is_active ? `Nonaktifkan gudang ${toggling?.code}?` : `Aktifkan gudang ${toggling?.code}?`}
        description={
          toggling?.is_active ? "Gudang nonaktif tidak bisa dipilih di dokumen baru. Gudang yang masih berisi stok tidak bisa dinonaktifkan." : undefined
        }
        confirmLabel={toggling?.is_active ? "Nonaktifkan" : "Aktifkan"}
        tone={toggling?.is_active ? "danger" : "primary"}
        onConfirm={async () => {
          if (!toggling) return;
          try {
            await setWarehouseActive(toggling.id, !toggling.is_active);
            notify.success(toggling.is_active ? "Gudang dinonaktifkan" : "Gudang diaktifkan");
            warehouses.reload();
          } catch (err) {
            notify.error(err);
          } finally {
            setToggling(null);
          }
        }}
      />
    </Page>
  );
}
