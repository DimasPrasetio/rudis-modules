import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { FormField } from "@/shared/components/ui/form-field";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { saveWarehouse } from "@/modules/stock/services/stock.service";
import type { Warehouse, WarehouseValues } from "@/modules/stock/types/stock";

interface WarehouseDialogProps {
  open: boolean;
  onClose: () => void;
  /** The warehouse to change; absent to add one. */
  warehouse?: Warehouse;
  /** The company's branches; empty without an organization module (the field is then hidden). */
  branches: { id: number; code: string; name: string }[];
  onSaved: () => void;
}

const EMPTY: WarehouseValues = { branch_id: 0, code: "", name: "" };

export function WarehouseDialog({ open, onClose, warehouse, branches, onSaved }: WarehouseDialogProps) {
  const [values, setValues] = useState<WarehouseValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setValues(warehouse ? { branch_id: warehouse.branch_id, code: warehouse.code, name: warehouse.name } : EMPTY);
    setErrors({});
    setFormError(null);
  }, [open, warehouse]);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      const saved = await saveWarehouse(values, warehouse?.id);
      notify.success(warehouse ? "Gudang disimpan" : `Gudang ${saved.code} ditambahkan`);
      onSaved();
      onClose();
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length) setErrors(fields);
      else setFormError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={() => !saving && onClose()}
      title={warehouse ? `Ubah gudang ${warehouse.code}` : "Gudang baru"}
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="warehouse-form" loading={saving}>
            {warehouse ? "Simpan" : "Tambah gudang"}
          </Button>
        </>
      }
    >
      <form id="warehouse-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && (
          <Notice tone="danger" title="Gagal menyimpan">
            {formError}
          </Notice>
        )}
        <FormField label="Kode" name="code" required error={errors.code} hint="Huruf besar dan angka, mis. GU-UTAMA.">
          <Input value={values.code} onChange={(e) => setValues((s) => ({ ...s, code: e.target.value.toUpperCase() }))} maxLength={20} />
        </FormField>
        <FormField label="Nama" name="name" required error={errors.name}>
          <Input value={values.name} onChange={(e) => setValues((s) => ({ ...s, name: e.target.value }))} maxLength={100} />
        </FormField>
        {branches.length > 0 && (
          <FormField label="Cabang" name="branch_id" error={errors.branch_id} hint="Izin dokumen stok gudang ini diperiksa untuk cabangnya.">
            <NativeSelect value={String(values.branch_id)} onChange={(e) => setValues((s) => ({ ...s, branch_id: Number(e.target.value) }))}>
              <NativeSelectOption value="0">Seluruh perusahaan</NativeSelectOption>
              {branches.map((b) => (
                <NativeSelectOption key={b.id} value={String(b.id)}>
                  {b.code} — {b.name}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </FormField>
        )}
      </form>
    </Modal>
  );
}
