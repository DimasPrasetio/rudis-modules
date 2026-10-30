import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { FormField } from "@/shared/components/ui/form-field";
import { MoneyInput } from "@/shared/components/ui/money-input";
import { focusFirstError } from "@/shared/components/ui/error-summary";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { addItemUnit } from "@/modules/items/services/items.service";
import { formatFactor } from "@/modules/items/types/item";
import type { Item, ItemUnit, Unit } from "@/modules/items/types/item";

interface ItemUnitDialogProps {
  open: boolean;
  onClose: () => void;
  item: Item;
  /** The item's alternative units, to leave the active ones out of the choice. */
  current: ItemUnit[];
  units: Unit[];
  onSaved: () => void;
}

/** Adds an alternative unit to an item: 1 DUS = 12 PCS. The factor cannot be changed later. */
export function ItemUnitDialog({ open, onClose, item, current, units, onSaved }: ItemUnitDialogProps) {
  const [unitId, setUnitId] = useState<number | null>(null);
  const [factor, setFactor] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setUnitId(null);
    setFactor(null);
    setErrors({});
    setFormError(null);
  }, [open]);

  const taken = new Set(current.filter((u) => u.is_active).map((u) => u.unit_id));
  const options = units.filter((u) => u.is_active && u.id !== item.base_unit_id && !taken.has(u.id));
  const chosen = units.find((u) => u.id === unitId);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const local: Record<string, string> = {};
    if (unitId == null) local.unit_id = "Pilih satuan";
    if (factor == null) local.factor = "Isi berapa satuan dasar dalam satu satuan ini";
    if (Object.keys(local).length) {
      setErrors(local);
      focusFirstError(local);
      return;
    }
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      await addItemUnit(item.id, unitId!, factor!);
      notify.success("Satuan ditambahkan");
      onSaved();
      onClose();
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

  return (
    <Modal
      open={open}
      onClose={() => !saving && onClose()}
      title="Tambah satuan alternatif"
      description={`Satuan lain untuk ${item.name}, dengan faktor ke satuan dasar ${item.base_unit_code}. Faktor tidak bisa diubah setelah disimpan.`}
      actions={
        <>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Batal
          </Button>
          <Button type="submit" form="item-unit-form" loading={saving} disabled={options.length === 0}>
            Tambah satuan
          </Button>
        </>
      }
    >
      <form id="item-unit-form" onSubmit={submit} noValidate className="grid gap-4">
        {formError && <Notice tone="danger" title="Gagal">{formError}</Notice>}
        {options.length === 0 && (
          <Notice tone="info" title="Tidak ada satuan lain yang bisa dipilih">
            Tambahkan satuan baru dari tombol Satuan di daftar barang.
          </Notice>
        )}
        <FormField label="Satuan" name="unit_id" required error={errors.unit_id}>
          <NativeSelect value={unitId == null ? "" : String(unitId)} onChange={(e) => setUnitId(e.target.value ? Number(e.target.value) : null)}>
            <NativeSelectOption value="">Pilih satuan</NativeSelectOption>
            {options.map((u) => (
              <NativeSelectOption key={u.id} value={String(u.id)}>
                {u.code} — {u.name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </FormField>
        <FormField
          label={`Isi dalam ${item.base_unit_code}`}
          name="factor"
          required
          error={errors.factor}
          hint={
            factor
              ? `1 ${chosen?.code ?? "satuan ini"} = ${formatFactor(factor)} ${item.base_unit_code}`
              : `Berapa ${item.base_unit_code} dalam satu satuan ini, mis. 12 (1 lusin = 12 pcs) atau 0,5.`
          }
        >
          <MoneyInput value={factor} onChange={setFactor} fractionDigits={10} maxIntegerDigits={9} prefix={null} />
        </FormField>
      </form>
    </Modal>
  );
}
