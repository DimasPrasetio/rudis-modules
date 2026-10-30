import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { ArrowLeft, Pencil, Plus } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Input } from "@/shared/components/primitives/input";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { Modal } from "@/shared/components/ui/modal";
import { Notice } from "@/shared/components/ui/notice";
import { FormField } from "@/shared/components/ui/form-field";
import { focusFirstError } from "@/shared/components/ui/error-summary";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { ActiveBadge } from "@/modules/items/components/ItemStatusBadge";
import { createCategory, updateCategory } from "@/modules/items/services/items.service";
import type { Category } from "@/modules/items/types/item";

interface CategoriesDialogProps {
  open: boolean;
  onClose: () => void;
  categories: Category[];
  loading: boolean;
  error: string | null;
  onRetry: () => void;
  canManage: boolean;
  /** A category was added or changed: reload the list. */
  onChanged: () => void;
}

/** The company's item categories: the list, adding, renaming, and deactivating one. */
export function CategoriesDialog({ open, onClose, categories, loading, error, onRetry, canManage, onChanged }: CategoriesDialogProps) {
  const [editing, setEditing] = useState<Category | "new" | null>(null);
  const [name, setName] = useState("");
  const [active, setActive] = useState(true);
  const [nameError, setNameError] = useState<string>();
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) setEditing(null);
  }, [open]);

  function edit(c: Category | "new") {
    setEditing(c);
    setName(c === "new" ? "" : c.name);
    setActive(c === "new" ? true : c.is_active);
    setNameError(undefined);
    setFormError(null);
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setNameError(undefined);
    setFormError(null);
    try {
      if (editing === "new") await createCategory(name);
      else if (editing) await updateCategory(editing.id, { name, is_active: active });
      notify.success("Kategori disimpan");
      setEditing(null);
      onChanged();
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (fields.name) {
        setNameError(fields.name);
        focusFirstError(fields);
      } else setFormError(apiErrorMessage(err));
    } finally {
      setSaving(false);
    }
  }

  const columns: DataTableColumn<Category>[] = [
    { header: "Nama", render: (c) => c.name },
    { header: "Status", width: 110, render: (c) => <ActiveBadge active={c.is_active} /> },
    ...(canManage
      ? [
          {
            header: "Aksi",
            id: "actions",
            width: 80,
            align: "right" as const,
            render: (c: Category) => (
              <Button variant="ghost" size="icon-sm" onClick={() => edit(c)} aria-label={`Ubah kategori ${c.name}`}>
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
      title={form ? (editing === "new" ? "Kategori baru" : `Ubah kategori ${editing.name}`) : "Kategori"}
      description={form ? undefined : "Pengelompokan barang untuk daftar dan laporan. Kategori nonaktif tidak bisa dipilih untuk barang baru."}
      actions={
        form ? (
          <>
            <Button variant="outline" onClick={() => setEditing(null)} disabled={saving}>
              <ArrowLeft />
              Kembali
            </Button>
            <Button type="submit" form="category-form" loading={saving}>
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
        <form id="category-form" onSubmit={submit} noValidate className="grid gap-4">
          {formError && <Notice tone="danger" title="Gagal menyimpan">{formError}</Notice>}
          <FormField label="Nama" name="name" required error={nameError}>
            <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} placeholder="mis. Minuman" />
          </FormField>
          {editing !== "new" && (
            <label className="flex items-center gap-2 text-sm">
              <Checkbox checked={active} onCheckedChange={(v) => setActive(v === true)} />
              Aktif — bisa dipilih untuk barang
            </label>
          )}
        </form>
      ) : (
        <div className="grid gap-4">
          {canManage && (
            <div>
              <Button size="sm" onClick={() => edit("new")}>
                <Plus />
                Tambah kategori
              </Button>
            </div>
          )}
          <DataTable
            columns={columns}
            rows={categories}
            rowKey={(c) => c.id}
            loading={loading}
            error={error}
            onRetry={onRetry}
            caption="Daftar kategori"
            maxHeight="50vh"
            stickyHeader
            emptyTitle="Belum ada kategori"
            emptyDescription="Kategori membantu menyaring daftar barang, mis. Minuman, Alat tulis, Jasa servis."
          />
        </div>
      )}
    </Modal>
  );
}
