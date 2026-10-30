import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { Ban, CheckCircle2, FilePen, Trash2 } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { Checkbox } from "@/shared/components/primitives/checkbox";
import { Input } from "@/shared/components/primitives/input";
import { NativeSelect, NativeSelectOption } from "@/shared/components/primitives/native-select";
import { Textarea } from "@/shared/components/primitives/textarea";
import { ConfirmDialog } from "@/shared/components/ui/confirm-dialog";
import { DataTable } from "@/shared/components/ui/data-table";
import type { DataTableColumn } from "@/shared/components/ui/data-table";
import { DescriptionList } from "@/shared/components/ui/description-list";
import { ErrorState } from "@/shared/components/ui/error-state";
import { AsyncSearchSelect } from "@/shared/components/ui/async-search-select";
import { DatePicker } from "@/shared/components/ui/date-picker";
import { ErrorSummary } from "@/shared/components/ui/error-summary";
import { FieldGrid } from "@/shared/components/ui/form-layout";
import { FormField, fieldId } from "@/shared/components/ui/form-field";
import { LineItemsEditor } from "@/shared/components/ui/line-items-editor";
import { MoneyInput } from "@/shared/components/ui/money-input";
import { Notice } from "@/shared/components/ui/notice";
import { Page } from "@/shared/components/ui/page";
import { PageHeader } from "@/shared/components/ui/page-header";
import { ReasonDialog } from "@/shared/components/ui/reason-dialog";
import { SectionCard } from "@/shared/components/ui/section-card";
import { DetailSkeleton } from "@/shared/components/ui/skeletons";
import { UnsavedChangesGuard } from "@/shared/components/ui/unsaved-changes-guard";
import { useResource } from "@/shared/hooks/use-resource";
import { formatDateTime, todayISO } from "@/shared/lib/format";
import { notify } from "@/shared/lib/notify";
import { apiErrorMessage, apiFieldErrors } from "@/shared/services/api-error";
import { DocumentStatusBadge } from "@/modules/stock/components/DocumentStatusBadge";
import {
  amendDocument,
  cancelDocument,
  createDocument,
  deleteDocument,
  fetchDocument,
  fetchItemUnits,
  listWarehouses,
  lookupItems,
  postDocument,
  updateDocument,
} from "@/modules/stock/services/stock.service";
import { KIND_LABEL, formatDay, formatQty, formatRp, unsigned } from "@/modules/stock/types/stock";
import type { DocKind, DocLine, DocumentValues, LineValues, StockDocument } from "@/modules/stock/types/stock";

const COUNTERPARTY_LABEL: Record<DocKind, string> = {
  receipt: "Diterima dari",
  issue: "Dikirim kepada",
  adjustment: "Pihak terkait",
};

let lineSeq = 0;
const newLine = (): LineValues => ({ key: `n${++lineSeq}`, item: null, units: [], unit_code: "", qty: null, unit_cost: null, zero_cost: false });

function valuesOf(d: StockDocument): DocumentValues {
  return {
    kind: d.kind,
    warehouse_id: d.warehouse_id,
    date: d.date,
    counterparty: d.counterparty,
    reference: d.reference,
    notes: d.notes,
    lines: (d.lines ?? []).map((l) => ({
      key: `l${l.id}`,
      item: { id: l.item_id, code: l.item_code, name: l.item_name, base_unit: "", is_active: true },
      units: [{ code: l.unit_code, name: l.unit_code, factor: l.factor, is_base: false }],
      unit_code: l.unit_code,
      qty: l.qty,
      unit_cost: l.zero_cost ? null : l.unit_cost,
      zero_cost: l.zero_cost,
    })),
  };
}

export default function DocumentPage() {
  const { id } = useParams();
  const docId = id ? Number(id) : 0;
  const [search] = useSearchParams();
  const doc = useResource((signal) => (docId ? fetchDocument(docId, signal) : Promise.resolve(null)), [docId]);

  if (docId && doc.loading && !doc.data)
    return (
      <Page>
        <DetailSkeleton />
      </Page>
    );
  if (docId && doc.error)
    return (
      <Page>
        <ErrorState message={doc.error} onRetry={doc.reload} />
      </Page>
    );
  const d = doc.data;
  if (d && !(d.status === "draft" && d.can_edit)) return <DocumentView doc={d} onChanged={doc.reload} />;
  const kind = (d?.kind ?? search.get("kind") ?? "receipt") as DocKind;
  return <DocumentForm key={d ? `${d.id}-${d.revision}` : kind} doc={d ?? undefined} kind={KIND_LABEL[kind] ? kind : "receipt"} onSaved={doc.reload} />;
}

// --- the draft form ----------------------------------------------------------------------------

function DocumentForm({ doc, kind, onSaved }: { doc?: StockDocument; kind: DocKind; onSaved: () => void }) {
  const navigate = useNavigate();
  const warehouses = useResource((signal) => listWarehouses(signal), []);
  const initial = useMemo<DocumentValues>(
    () => (doc ? valuesOf(doc) : { kind, warehouse_id: 0, date: todayISO(), counterparty: "", reference: "", notes: "", lines: [newLine()] }),
    [doc, kind],
  );
  const [values, setValues] = useState<DocumentValues>(initial);
  // What "unchanged" means: the loaded document, plus defaults the page filled in by itself.
  const [baseline, setBaseline] = useState<DocumentValues>(initial);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState<"save" | "post" | null>(null);
  const [confirmPost, setConfirmPost] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const comparable = (v: DocumentValues) => JSON.stringify({ ...v, lines: v.lines.map(({ units: _units, ...l }) => l) });
  const dirty = comparable(values) !== comparable(baseline);

  const active = (warehouses.data ?? []).filter((w) => w.is_active || w.id === values.warehouse_id);
  useEffect(() => {
    if (!values.warehouse_id && active.length === 1) {
      setValues((v) => ({ ...v, warehouse_id: active[0].id }));
      setBaseline((v) => ({ ...v, warehouse_id: active[0].id }));
    }
  }, [active, values.warehouse_id]);

  // An existing draft's lines carry only the unit they use: load each item's units once.
  useEffect(() => {
    if (!doc) return;
    const ctl = new AbortController();
    const ids = [...new Set((doc.lines ?? []).map((l) => l.item_id))];
    Promise.all(ids.map((i) => fetchItemUnits(i, ctl.signal).then((u) => [i, u] as const)))
      .then((pairs) => {
        const byItem = new Map(pairs);
        setValues((v) => ({ ...v, lines: v.lines.map((l) => (l.item && byItem.get(l.item.id)?.length ? { ...l, units: byItem.get(l.item.id)! } : l)) }));
      })
      .catch(() => undefined);
    return () => ctl.abort();
  }, [doc]);

  const set =
    <K extends keyof DocumentValues>(k: K) =>
    (v: DocumentValues[K]) =>
      setValues((s) => ({ ...s, [k]: v }));
  const setLine = (key: string, patch: Partial<LineValues>) =>
    setValues((s) => ({ ...s, lines: s.lines.map((l) => (l.key === key ? { ...l, ...patch } : l)) }));

  async function pickItem(key: string, item: LineValues["item"]) {
    setLine(key, { item, units: [], unit_code: "" });
    if (!item) return;
    try {
      const units = await fetchItemUnits(item.id);
      setLine(key, { units, unit_code: units.find((u) => u.is_base)?.code ?? units[0]?.code ?? "" });
    } catch (err) {
      notify.error(err);
    }
  }

  async function save(post: boolean) {
    setBusy(post ? "post" : "save");
    setErrors({});
    setFormError(null);
    try {
      let saved = doc ? await updateDocument(doc.id, values) : await createDocument(values);
      if (post) {
        saved = await postDocument(saved.id);
        notify.success(`${KIND_LABEL[saved.kind]} ${saved.number} diposting`);
      } else {
        notify.success("Draf disimpan");
      }
      if (doc) onSaved();
      navigate(`/stock/documents/${saved.id}`, { replace: !!doc });
    } catch (err) {
      const fields = apiFieldErrors(err);
      if (Object.keys(fields).length) setErrors(fields);
      else setFormError(apiErrorMessage(err));
      if (doc) onSaved();
    } finally {
      setBusy(null);
    }
  }

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    void save(false);
  }

  const labels = useMemo(() => {
    const out: Record<string, string> = {
      warehouse_id: "Gudang",
      date: "Tanggal",
      counterparty: COUNTERPARTY_LABEL[kind],
      reference: "Referensi",
      notes: "Catatan",
      lines: "Barang",
    };
    values.lines.forEach((_, i) => {
      out[`lines.${i}.item_id`] = `Baris ${i + 1}: barang`;
      out[`lines.${i}.unit_code`] = `Baris ${i + 1}: satuan`;
      out[`lines.${i}.qty`] = `Baris ${i + 1}: jumlah`;
      out[`lines.${i}.unit_cost`] = `Baris ${i + 1}: biaya per satuan`;
    });
    return out;
  }, [values.lines, kind]);

  const err = (i: number, f: string) => errors[`lines.${i}.${f}`];
  const showCost = kind !== "issue";
  const title = doc ? (doc.number ? `Koreksi ${doc.number}` : `Draf ${KIND_LABEL[kind].toLowerCase()}`) : `${KIND_LABEL[kind]} baru`;

  return (
    <Page>
      <PageHeader
        title={title}
        meta={doc && <DocumentStatusBadge status={doc.status} />}
        crumbs={[{ label: "Dokumen stok", to: "/stock/documents" }, { label: title }]}
        description={
          kind === "adjustment"
            ? "Isi jumlah positif untuk barang yang ditemukan, negatif untuk yang hilang atau rusak."
            : kind === "receipt"
              ? "Barang yang masuk ke gudang, dengan biayanya."
              : "Barang yang keluar dari gudang; nilainya dihitung dari metode biaya."
        }
      />
      <UnsavedChangesGuard when={dirty && busy === null} />
      <form id="document-form" onSubmit={submit} noValidate className="grid gap-6">
        <ErrorSummary errors={errors} labels={labels} />
        {formError && (
          <Notice tone="danger" title="Tidak bisa disimpan">
            {formError}
          </Notice>
        )}
        <SectionCard title="Dokumen">
          <FieldGrid columns={3}>
            <FormField label="Gudang" name="warehouse_id" required error={errors.warehouse_id}>
              <NativeSelect value={String(values.warehouse_id)} onChange={(e) => set("warehouse_id")(Number(e.target.value))}>
                <NativeSelectOption value="0">Pilih gudang…</NativeSelectOption>
                {active.map((w) => (
                  <NativeSelectOption key={w.id} value={String(w.id)}>
                    {w.code} — {w.name}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </FormField>
            <FormField label="Tanggal" name="date" required error={errors.date}>
              <DatePicker value={values.date} onChange={set("date")} />
            </FormField>
            <FormField label="Referensi" name="reference" error={errors.reference} hint="No. surat jalan, PO, atau berita acara.">
              <Input value={values.reference} onChange={(e) => set("reference")(e.target.value)} maxLength={60} />
            </FormField>
            {kind !== "adjustment" && (
              <FormField label={COUNTERPARTY_LABEL[kind]} name="counterparty" error={errors.counterparty}>
                <Input value={values.counterparty} onChange={(e) => set("counterparty")(e.target.value)} maxLength={150} autoComplete="organization" />
              </FormField>
            )}
          </FieldGrid>
        </SectionCard>

        <SectionCard
          title="Barang"
          description={
            kind === "receipt"
              ? 'Biaya per satuan yang dipilih. Centang "Tanpa biaya" untuk bonus atau hibah.'
              : kind === "adjustment"
                ? "Barang yang ditambahkan tanpa biaya dinilai dengan biaya rata-rata saat ini."
                : undefined
          }
        >
          {errors.lines && <p className="mb-3 text-sm text-destructive">{errors.lines}</p>}
          <LineItemsEditor
            lines={values.lines}
            getKey={(l) => l.key}
            onAdd={() => set("lines")([...values.lines, newLine()])}
            onRemove={(i) => set("lines")(values.lines.filter((_, j) => j !== i))}
            addLabel="Tambah barang"
            columns={[
              {
                id: "item",
                header: "Barang",
                render: (l, i) => (
                  <AsyncSearchSelect
                    id={fieldId(`lines.${i}.item_id`)}
                    aria-label={`Barang baris ${i + 1}`}
                    aria-invalid={!!err(i, "item_id")}
                    value={l.item?.id ?? null}
                    selectedOption={l.item ? { value: l.item.id, label: `${l.item.code} — ${l.item.name}` } : null}
                    placeholder="Cari barang…"
                    loadOptions={async (q, signal) =>
                      (await lookupItems(q, signal)).map((it) => ({
                        value: it.id,
                        label: `${it.code} — ${it.name}`,
                        description: `Satuan dasar ${it.base_unit}`,
                        searchText: it.code,
                      }))
                    }
                    onChange={(v, opt) =>
                      void pickItem(
                        l.key,
                        v && opt
                          ? {
                              id: Number(v),
                              code: opt.label.split(" — ")[0],
                              name: opt.label.split(" — ").slice(1).join(" — "),
                              base_unit: "",
                              is_active: true,
                            }
                          : null,
                      )
                    }
                  />
                ),
              },
              {
                id: "unit",
                header: "Satuan",
                width: "8rem",
                render: (l, i) => (
                  <NativeSelect
                    id={fieldId(`lines.${i}.unit_code`)}
                    aria-label={`Satuan baris ${i + 1}`}
                    aria-invalid={!!err(i, "unit_code")}
                    value={l.unit_code}
                    disabled={l.units.length === 0}
                    onChange={(e) => setLine(l.key, { unit_code: e.target.value })}
                  >
                    {l.units.map((u) => (
                      <NativeSelectOption key={u.code} value={u.code}>
                        {u.code}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                ),
              },
              {
                id: "qty",
                header: kind === "adjustment" ? "Tambah (+) / kurang (−)" : "Jumlah",
                width: "10rem",
                align: "right",
                render: (l, i) => (
                  <MoneyInput
                    id={fieldId(`lines.${i}.qty`)}
                    aria-label={`Jumlah baris ${i + 1}`}
                    aria-invalid={!!err(i, "qty")}
                    prefix={null}
                    fractionDigits={4}
                    allowNegative={kind === "adjustment"}
                    value={l.qty}
                    onChange={(v) => setLine(l.key, { qty: v })}
                  />
                ),
              },
              ...(showCost
                ? [
                    {
                      id: "cost",
                      header: kind === "adjustment" ? "Biaya per satuan (opsional)" : "Biaya per satuan",
                      width: "12rem",
                      align: "right" as const,
                      render: (l: LineValues, i: number) => (
                        <MoneyInput
                          id={fieldId(`lines.${i}.unit_cost`)}
                          aria-label={`Biaya per satuan baris ${i + 1}`}
                          aria-invalid={!!err(i, "unit_cost")}
                          fractionDigits={2}
                          value={l.zero_cost ? "0" : l.unit_cost}
                          disabled={l.zero_cost || (kind === "adjustment" && !!l.qty?.startsWith("-"))}
                          onChange={(v) => setLine(l.key, { unit_cost: v })}
                        />
                      ),
                    },
                  ]
                : []),
              ...(kind === "receipt"
                ? [
                    {
                      id: "free",
                      header: "Tanpa biaya",
                      width: "7rem",
                      align: "center" as const,
                      render: (l: LineValues, i: number) => (
                        <Checkbox
                          aria-label={`Tanpa biaya baris ${i + 1}`}
                          checked={l.zero_cost}
                          onCheckedChange={(c) => setLine(l.key, { zero_cost: c === true })}
                        />
                      ),
                    },
                  ]
                : []),
            ]}
          />
          {values.lines.some((_, i) => err(i, "item_id") || err(i, "unit_code") || err(i, "qty") || err(i, "unit_cost")) && (
            <ul className="mt-3 grid gap-1 text-sm text-destructive">
              {values.lines.flatMap((_, i) =>
                ["item_id", "unit_code", "qty", "unit_cost"]
                  .filter((f) => err(i, f))
                  .map((f) => <li key={`${i}-${f}`}>{`${labels[`lines.${i}.${f}`]} — ${err(i, f)}`}</li>),
              )}
            </ul>
          )}
        </SectionCard>

        <SectionCard title="Catatan">
          <FormField label="Catatan" name="notes" error={errors.notes}>
            <Textarea value={values.notes} onChange={(e) => set("notes")(e.target.value)} rows={3} maxLength={2000} />
          </FormField>
        </SectionCard>

        <div className="flex flex-wrap justify-end gap-2">
          {doc && (
            <Button type="button" variant="ghost" onClick={() => setConfirmDelete(true)} disabled={busy !== null}>
              <Trash2 />
              Hapus draf
            </Button>
          )}
          <Button type="button" variant="outline" asChild>
            <Link to="/stock/documents">Kembali</Link>
          </Button>
          <Button type="submit" variant="outline" loading={busy === "save"} disabled={busy !== null}>
            Simpan draf
          </Button>
          {(!doc || doc.can_post) && (
            <Button type="button" onClick={() => setConfirmPost(true)} loading={busy === "post"} disabled={busy !== null}>
              <CheckCircle2 />
              Simpan &amp; posting
            </Button>
          )}
        </div>
      </form>
      <ConfirmDialog
        open={confirmPost}
        onOpenChange={setConfirmPost}
        title="Posting dokumen ini?"
        description="Stok dan nilainya berubah sekarang, dan dokumen mendapat nomor. Setelah diposting dokumen tidak bisa diubah — hanya dibatalkan dengan alasan."
        confirmLabel="Posting"
        onConfirm={async () => {
          setConfirmPost(false);
          await save(true);
        }}
      />
      {doc && (
        <ConfirmDialog
          open={confirmDelete}
          onOpenChange={setConfirmDelete}
          title="Hapus draf ini?"
          description="Draf belum mengubah stok; menghapusnya tidak bisa dibatalkan."
          confirmLabel="Hapus draf"
          tone="danger"
          onConfirm={async () => {
            try {
              await deleteDocument(doc.id);
              notify.success("Draf dihapus");
              navigate("/stock/documents", { replace: true });
            } catch (e) {
              notify.error(e);
            } finally {
              setConfirmDelete(false);
            }
          }}
        />
      )}
    </Page>
  );
}

// --- a posted or cancelled document -------------------------------------------------------------

function DocumentView({ doc, onChanged }: { doc: StockDocument; onChanged: () => void }) {
  const navigate = useNavigate();
  const [cancelling, setCancelling] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const title = doc.number || `Draf ${KIND_LABEL[doc.kind].toLowerCase()}`;
  const showCost = doc.kind !== "issue" || doc.status !== "draft";

  const columns: DataTableColumn<DocLine>[] = [
    {
      header: "Barang",
      link: (l) => `/stock/items/${l.item_id}?warehouse_id=${doc.warehouse_id}`,
      render: (l) => (
        <div className="min-w-0">
          <div className="truncate font-medium">{l.item_name}</div>
          <div className="font-mono text-xs text-muted-foreground">{l.item_code}</div>
        </div>
      ),
    },
    {
      header: "Jumlah",
      width: 150,
      align: "right",
      render: (l) => (
        <span className="tabular">
          {formatQty(l.qty)} <span className="text-muted-foreground">{l.unit_code}</span>
        </span>
      ),
    },
    {
      header: "Biaya per satuan",
      width: 160,
      align: "right",
      mobileHidden: true,
      render: (l) => <span className="tabular">{l.zero_cost ? "Tanpa biaya" : formatRp(l.unit_cost)}</span>,
    },
    ...(showCost
      ? [
          {
            header: "Nilai",
            width: 160,
            align: "right" as const,
            render: (l: DocLine) => <span className="tabular font-medium">{formatRp(doc.kind === "issue" ? unsigned(l.value) : l.value)}</span>,
          },
        ]
      : []),
  ];

  async function amend() {
    try {
      const draft = await amendDocument(doc.id);
      notify.success("Draf koreksi dibuat");
      navigate(`/stock/documents/${draft.id}`);
    } catch (e) {
      notify.error(e);
    }
  }

  return (
    <Page>
      <PageHeader
        title={title}
        meta={<DocumentStatusBadge status={doc.status} />}
        subtitle={KIND_LABEL[doc.kind]}
        crumbs={[{ label: "Dokumen stok", to: "/stock/documents" }, { label: title }]}
        actions={
          <>
            {doc.status === "posted" && doc.can_cancel && (
              <Button variant="outline" onClick={() => setCancelling(true)}>
                <Ban />
                Batalkan
              </Button>
            )}
            {doc.status === "cancelled" && doc.can_cancel && (
              <Button onClick={amend}>
                <FilePen />
                Buat koreksi
              </Button>
            )}
          </>
        }
      />
      {doc.status === "cancelled" && (
        <Notice tone="warning" title={`Dibatalkan ${formatDateTime(doc.cancelled_at)}`}>
          Alasan: {doc.cancel_reason}. Mutasinya sudah dibalik; stok kembali seperti sebelum dokumen ini.
        </Notice>
      )}
      {doc.status === "draft" && (
        <Notice tone="info" title="Draf ini belum mengubah stok">
          Anda tidak punya izin mengubah draf di gudang ini.
        </Notice>
      )}
      <SectionCard title="Dokumen">
        <DescriptionList
          columns={3}
          items={[
            { label: "Tanggal", value: formatDay(doc.date) },
            { label: "Gudang", value: `${doc.warehouse_code} — ${doc.warehouse_name}` },
            { label: COUNTERPARTY_LABEL[doc.kind], value: doc.counterparty || "–" },
            { label: "Referensi", value: doc.reference || "–" },
            { label: "Diposting", value: doc.posted_at ? formatDateTime(doc.posted_at) : "–" },
            {
              label: "Mengoreksi",
              value: doc.amended_from ? (
                <Link className="text-primary underline-offset-4 hover:underline" to={`/stock/documents/${doc.amended_from}`}>
                  Dokumen asal
                </Link>
              ) : (
                "–"
              ),
            },
            ...(doc.notes ? [{ label: "Catatan", value: doc.notes, wide: true }] : []),
          ]}
        />
      </SectionCard>
      <SectionCard
        title="Barang"
        flush
        actions={
          doc.status !== "draft" && (
            <span className="tabular text-sm font-medium">Total {formatRp(doc.kind === "issue" ? unsigned(doc.total_value) : doc.total_value)}</span>
          )
        }
      >
        <DataTable columns={columns} rows={doc.lines} rowKey={(l) => l.id} caption="Barang di dokumen ini" emptyTitle="Tidak ada barang" />
      </SectionCard>
      <ReasonDialog
        open={cancelling}
        onClose={() => setCancelling(false)}
        title={`Batalkan ${title}?`}
        description="Setiap mutasi dokumen ini dibalik persis. Hanya bisa bila belum ada mutasi lebih baru untuk barang yang sama di gudang ini."
        confirmLabel="Batalkan dokumen"
        submitting={submitting}
        onSubmit={async (reason) => {
          setSubmitting(true);
          try {
            await cancelDocument(doc.id, reason);
            notify.success("Dokumen dibatalkan");
            setCancelling(false);
            onChanged();
          } catch (e) {
            notify.error(e);
          } finally {
            setSubmitting(false);
          }
        }}
      />
    </Page>
  );
}
