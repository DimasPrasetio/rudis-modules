import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Input } from "@/shared/components/primitives/input";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { Textarea } from "@/shared/components/primitives/textarea";
import { FormSheet } from "@/shared/components/ui/form-sheet";
import { Notice } from "@/shared/components/ui/notice";
import { DigitsInput } from "@/shared/components/ui/digits-input";
import { ErrorSummary } from "@/shared/components/ui/error-summary";
import { FieldGrid } from "@/shared/components/ui/form-layout";
import { FormField } from "@/shared/components/ui/form-field";
import { UnsavedChangesDialog } from "@/shared/components/ui/unsaved-changes-guard";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { addCommonUnits, createItem, updateItem } from "@/modules/items/services/items.service";
import { KIND_LABEL } from "@/modules/items/types/item";
import type { Category, Item, ItemKind, ItemValues, Unit } from "@/modules/items/types/item";

interface ItemFormSheetProps {
  open: boolean;
  onClose: () => void;
  /** The item to change; absent to add one. */
  item?: Item;
  units: Unit[];
  categories: Category[];
  /** The units changed here (the common units were added): reload them. */
  onUnitsChanged: () => void;
  onSaved: (item: Item) => void;
}

const EMPTY: ItemValues = {
  code: "",
  name: "",
  kind: "goods",
  is_stocked: true,
  category_id: null,
  base_unit_id: null,
  barcode: "",
  tax_code: "",
  description: "",
};

function valuesOf(it: Item): ItemValues {
  return {
    code: it.code,
    name: it.name,
    kind: it.kind,
    is_stocked: it.is_stocked,
    category_id: it.category_id,
    base_unit_id: it.base_unit_id,
    barcode: it.barcode,
    tax_code: it.tax_code === "000000" ? "" : it.tax_code,
    description: it.description,
  };
}

/** Field → the label the person sees, for the error summary. */
const LABELS: Record<string, string> = {
  code: "Kode",
  name: "Nama",
  kind: "Jenis",
  is_stocked: "Lacak stok",
  base_unit_id: "Satuan dasar",
  category_id: "Kategori",
  barcode: "Barcode",
  tax_code: "Kode barang/jasa Coretax",
  description: "Keterangan",
};

const KINDS = (Object.keys(KIND_LABEL) as ItemKind[]).map((k) => ({ value: k, label: KIND_LABEL[k] }));

export function ItemFormSheet({ open, onClose, item, units, categories, onUnitsChanged, onSaved }: ItemFormSheetProps) {
  const [values, setValues] = useState<ItemValues>(EMPTY);
  const [initial, setInitial] = useState<ItemValues>(EMPTY);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [addingUnits, setAddingUnits] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);

  useEffect(() => {
    if (!open) return;
    const start = item ? valuesOf(item) : EMPTY;
    setValues(start);
    setInitial(start);
    setErrors({});
    setFormError(null);
  }, [open, item]);

  const dirty = useMemo(() => JSON.stringify(values) !== JSON.stringify(initial), [values, initial]);
  const set = <K extends keyof ItemValues>(k: K) => (v: ItemValues[K]) => setValues((s) => ({ ...s, [k]: v }));

  function close() {
    if (saving) return;
    if (dirty) setConfirmLeave(true);
    else onClose();
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setErrors({});
    setFormError(null);
    try {
      // A change sends only the fields that may change; kind, stock tracking, and base unit are fixed.
      const saved = item
        ? await updateItem(item.id, {
            code: values.code,
            name: values.name,
            category_id: values.category_id ?? 0,
            barcode: values.barcode,
            tax_code: values.tax_code,
            description: values.description,
          })
        : await createItem(values);
      notify.success(item ? "Barang disimpan" : `${KIND_LABEL[saved.kind]} ${saved.code} ditambahkan`);
      onSaved(saved);
      onClose();
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length) setErrors(fields);
      else setFormError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  async function addCommon() {
    setAddingUnits(true);
    try {
      const added = await addCommonUnits();
      notify.success(added.length ? `${added.length} satuan umum ditambahkan` : "Semua satuan umum sudah ada");
      onUnitsChanged();
    } catch (err) {
      notify.error(err);
    } finally {
      setAddingUnits(false);
    }
  }

  // An inactive unit or category stays listed only when it is the one the item already has.
  const unitOptions = units.filter((u) => u.is_active || u.id === values.base_unit_id);
  const categoryOptions = categories.filter((c) => c.is_active || c.id === values.category_id);
  const editing = !!item;

  return (
    <>
      <FormSheet
        open={open}
        onClose={close}
        size="lg"
        title={item ? `Ubah ${item.name}` : "Barang atau jasa baru"}
        description={item ? undefined : "Kode dibuat otomatis bila dikosongkan. Jenis, pelacakan stok, dan satuan dasar tidak bisa diubah setelah disimpan."}
        actions={
          <>
            <Button variant="outline" onClick={close} disabled={saving}>
              Batal
            </Button>
            <Button type="submit" form="item-form" loading={saving}>
              {item ? "Simpan" : "Tambah"}
            </Button>
          </>
        }
      >
        <form id="item-form" onSubmit={submit} noValidate className="grid gap-6">
          <ErrorSummary errors={errors} labels={LABELS} />
          {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}

          <fieldset className="grid gap-4">
            <legend className="mb-1 text-sm font-semibold text-foreground">Identitas</legend>
            <FormField label="Jenis" required name="kind" error={errors.kind} hint={editing ? "Tidak bisa diubah setelah dibuat." : undefined}>
              <NativeSelect
                value={values.kind}
                disabled={editing}
                onChange={(e) => {
                  const kind = e.target.value as ItemKind;
                  setValues((s) => ({ ...s, kind, is_stocked: kind === "goods" }));
                }}
              >
                {KINDS.map((k) => (
                  <NativeSelectOption key={k.value} value={k.value}>
                    {k.label}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </FormField>
            <FieldGrid columns={2}>
              <FormField label="Kode" name="code" required={editing} error={errors.code} hint={editing ? undefined : "Kosongkan untuk kode otomatis."}>
                <Input
                  value={values.code}
                  onChange={(e) => set("code")(e.target.value.toUpperCase())}
                  maxLength={30}
                  placeholder={editing ? undefined : "Otomatis"}
                  autoComplete="off"
                />
              </FormField>
              <FormField label="Nama" name="name" required error={errors.name}>
                <Input value={values.name} onChange={(e) => set("name")(e.target.value)} maxLength={150} placeholder="mis. Air mineral 600 ml" />
              </FormField>
            </FieldGrid>
          </fieldset>

          <fieldset className="grid gap-4">
            <legend className="mb-1 text-sm font-semibold text-foreground">Satuan dan stok</legend>
            {unitOptions.length === 0 && !editing && (
              <Notice tone="info" title="Belum ada satuan">
                <p>Barang dihitung dalam satuan dasarnya. Tambahkan satuan umum (PCS, KG, DUS, …) atau buat sendiri dari tombol Satuan.</p>
                <Button className="mt-2" size="sm" variant="outline" onClick={addCommon} loading={addingUnits}>
                  Tambahkan satuan umum
                </Button>
              </Notice>
            )}
            <FieldGrid columns={2}>
              <FormField
                label="Satuan dasar"
                name="base_unit_id"
                required
                error={errors.base_unit_id}
                hint={editing ? "Tidak bisa diubah — tambahkan satuan alternatif di halaman barang." : "Stok dan dokumen dihitung dalam satuan ini."}
              >
                <NativeSelect
                  value={values.base_unit_id == null ? "" : String(values.base_unit_id)}
                  onChange={(e) => set("base_unit_id")(e.target.value ? Number(e.target.value) : null)}
                  disabled={editing}
                >
                  <NativeSelectOption value="">Pilih satuan</NativeSelectOption>
                  {unitOptions.map((u) => (
                    <NativeSelectOption key={u.id} value={String(u.id)}>
                      {u.code} — {u.name}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </FormField>
              <FormField label="Kategori" name="category_id" error={errors.category_id} hint="Opsional.">
                <NativeSelect
                  value={values.category_id == null ? "" : String(values.category_id)}
                  onChange={(e) => set("category_id")(e.target.value ? Number(e.target.value) : null)}
                >
                  <NativeSelectOption value="">Tanpa kategori</NativeSelectOption>
                  {categoryOptions.map((c) => (
                    <NativeSelectOption key={c.id} value={String(c.id)}>
                      {c.name}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </FormField>
            </FieldGrid>
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                id="field-is_stocked"
                checked={values.is_stocked}
                disabled={editing || values.kind === "service"}
                onCheckedChange={(v) => set("is_stocked")(v === true)}
              />
              Lacak stok barang ini
            </label>
            <p className="-mt-2 text-xs text-muted-foreground">
              {values.kind === "service"
                ? "Jasa tidak punya stok."
                : editing
                  ? "Tidak bisa diubah setelah dibuat."
                  : "Matikan untuk barang yang dibeli atau dijual tanpa dihitung stoknya (mis. kantong plastik)."}
            </p>
            {errors.is_stocked && <p className="text-xs font-medium text-destructive">{errors.is_stocked}</p>}
          </fieldset>

          <fieldset className="grid gap-4">
            <legend className="mb-1 text-sm font-semibold text-foreground">Kode lain</legend>
            <FieldGrid columns={2}>
              <FormField label="Barcode" name="barcode" error={errors.barcode} hint="Opsional; 8–64 karakter, unik di perusahaan ini.">
                <Input value={values.barcode} onChange={(e) => set("barcode")(e.target.value.trim())} maxLength={64} autoComplete="off" inputMode="text" />
              </FormField>
              <FormField
                label="Kode barang/jasa Coretax"
                name="tax_code"
                error={errors.tax_code}
                hint="Opsional, 6 angka untuk faktur pajak; kosong dikirim sebagai 000000."
              >
                <DigitsInput value={values.tax_code} onChange={set("tax_code")} maxDigits={6} placeholder="000000" />
              </FormField>
            </FieldGrid>
          </fieldset>

          <FormField label="Keterangan" name="description" error={errors.description} hint="Opsional.">
            <Textarea value={values.description} onChange={(e) => set("description")(e.target.value)} rows={3} maxLength={2000} />
          </FormField>
        </form>
      </FormSheet>
      <UnsavedChangesDialog
        open={confirmLeave}
        onStay={() => setConfirmLeave(false)}
        onLeave={() => {
          setConfirmLeave(false);
          onClose();
        }}
        description="Isian barang yang belum disimpan akan hilang."
        leaveLabel="Buang perubahan"
      />
    </>
  );
}
