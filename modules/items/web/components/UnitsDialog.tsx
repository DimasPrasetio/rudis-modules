import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { ArrowLeft, ListPlus, Pencil, Plus } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Input } from "@/shared/components/primitives/input";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { FieldGrid } from "@/shared/components/ui/form-layout";
import { FormField } from "@/shared/components/ui/form-field";
import { focusFirstError } from "@/shared/components/ui/error-summary";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { ActiveBadge } from "@/modules/items/components/ItemStatusBadge";
import { addCommonUnits, createUnit, updateUnit } from "@/modules/items/services/items.service";
import type { Unit, UnitValues } from "@/modules/items/types/item";

interface UnitsDialogProps {
  open: boolean;
  onClose: () => void;
  units: Unit[];
  loading: boolean;
  error: string | null;
  onRetry: () => void;
  canManage: boolean;
  /** A unit was added or changed: reload the list. */
  onChanged: () => void;
}

const EMPTY: UnitValues = { code: "", name: "", tax_unit_code: "", is_active: true };

/** The company's units: the list, adding and changing one, and the common units in one click. */
export function UnitsDialog({ open, onClose, units, loading, error, onRetry, canManage, onChanged }: UnitsDialogProps) {
  const [editing, setEditing] = useState<Unit | "new" | null>(null);
  const [values, setValues] = useState<UnitValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [addingCommon, setAddingCommon] = useState(false);

  useEffect(() => {
    if (!open) setEditing(null);
  }, [open]);

  function edit(u: Unit | "new") {
    setEditing(u);
    setValues(u === "new" ? EMPTY : { code: u.code, name: u.name, tax_unit_code: u.tax_unit_code, is_active: u.is_active });
    setErrors({});
    setFormError(null);
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      if (editing === "new") {
        await createUnit(values);
        notify.success(`Satuan ${values.code} ditambahkan`);
      } else if (editing) {
        await updateUnit(editing.id, { name: values.name, tax_unit_code: values.tax_unit_code, is_active: values.is_active });
        notify.success(`Satuan ${editing.code} disimpan`);
      }
      setEditing(null);
      onChanged();
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length) {
        setErrors(fields);
        focusFirstError(fields);
      } else setFormError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  async function addCommon() {
    setAddingCommon(true);
    try {
      const added = await addCommonUnits();
      notify.success(added.length ? `Ditambahkan: ${added.join(", ")}` : "Semua satuan umum sudah ada");
      onChanged();
    } catch (err) {
      notify.error(err);
    } finally {
      setAddingCommon(false);
    }
  }

  const columns: DataTableColumn<Unit>[] = [
    { header: "Kode", width: 100, render: (u) => <span className="font-mono text-sm">{u.code}</span> },
    { header: "Nama", render: (u) => u.name },
    { header: "Kode Coretax", width: 130, mobileHidden: true, render: (u) => (u.tax_unit_code ? <span className="font-mono text-sm">{u.tax_unit_code}</span> : <span className="text-muted-foreground">–</span>) },
    { header: "Status", width: 110, render: (u) => <ActiveBadge active={u.is_active} /> },
    ...(canManage
      ? [
          {
            header: "Aksi",
            id: "actions",
            width: 80,
            align: "right" as const,
            render: (u: Unit) => (
              <Button variant="ghost" size="icon-sm" onClick={() => edit(u)} aria-label={`Ubah satuan ${u.code}`}>
                <Pencil />
              </Button>
            ),
          },
        ]
      : []),
  ];

  const form = editing !== null;

  return (
    <Modal
      open={open}
      onClose={() => !saving && onClose()}
      size="lg"
      title={form ? (editing === "new" ? "Satuan baru" : `Ubah satuan ${editing.code}`) : "Satuan"}
      description={
        form
          ? "Kode satuan tidak bisa diubah setelah disimpan: dokumen menyalinnya."
          : "Satuan yang dipakai barang perusahaan ini. Satuan nonaktif tidak bisa dipilih untuk barang baru; barang yang sudah memakainya tetap."
      }
      actions={
        form ? (
          <>
            <Button variant="outline" onClick={() => setEditing(null)} disabled={saving}>
              <ArrowLeft />
              Kembali
            </Button>
            <Button type="submit" form="unit-form" loading={saving}>
              Simpan
            </Button>
          </>
        ) : (
          <Button variant="outline" onClick={onClose}>
            Tutup
          </Button>
        )
      }
    >
      {form ? (
        <form id="unit-form" onSubmit={submit} noValidate className="grid gap-4">
          {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
          <FieldGrid columns={2}>
            <FormField label="Kode" name="code" required error={errors.code} hint={editing === "new" ? "Huruf besar dan angka, maks. 10, mis. PCS." : "Tidak bisa diubah."}>
              <Input
                value={values.code}
                onChange={(e) => setValues((s) => ({ ...s, code: e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, "") }))}
                maxLength={10}
                disabled={editing !== "new"}
                autoComplete="off"
              />
            </FormField>
            <FormField label="Nama" name="name" required error={errors.name}>
              <Input value={values.name} onChange={(e) => setValues((s) => ({ ...s, name: e.target.value }))} maxLength={60} placeholder="mis. Piece" />
            </FormField>
            <FormField label="Kode satuan Coretax" name="tax_unit_code" error={errors.tax_unit_code} hint="Opsional, untuk faktur pajak: UM. dan 4 angka, mis. UM.0021.">
              <Input
                value={values.tax_unit_code}
                onChange={(e) => setValues((s) => ({ ...s, tax_unit_code: e.target.value.toUpperCase() }))}
                maxLength={7}
                placeholder="UM.0021"
                autoComplete="off"
              />
            </FormField>
          </FieldGrid>
          {editing !== "new" && (
            <label className="flex items-center gap-2 text-sm">
              <Checkbox checked={values.is_active} onCheckedChange={(v) => setValues((s) => ({ ...s, is_active: v === true }))} />
              Aktif — bisa dipilih untuk barang dan satuan alternatif baru
            </label>
          )}
        </form>
      ) : (
        <div className="grid gap-4">
          {canManage && (
            <div className="flex flex-wrap gap-2">
              <Button size="sm" onClick={() => edit("new")}>
                <Plus />
                Tambah satuan
              </Button>
              <Button size="sm" variant="outline" onClick={addCommon} loading={addingCommon}>
                <ListPlus />
                Tambahkan satuan umum
              </Button>
            </div>
          )}
          <DataTable
            columns={columns}
            rows={units}
            rowKey={(u) => u.id}
            loading={loading}
            error={error}
            onRetry={onRetry}
            caption="Daftar satuan"
            maxHeight="50vh"
            stickyHeader
            emptyTitle="Belum ada satuan"
            emptyDescription="Tambahkan satuan umum (PCS, UNIT, KG, G, L, M, DUS, LSN, SET, LBR, ROL, KRT) dengan kode satuan Coretax-nya, atau buat sendiri."
          />
        </div>
      )}
    </Modal>
  );
}
