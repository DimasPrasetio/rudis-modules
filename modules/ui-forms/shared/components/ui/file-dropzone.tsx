import { useId, useRef, useState } from "react";
import type { DragEvent } from "react";
import { FileUp } from "lucide-react";
import { cn } from "cn";

export interface FileDropzoneProps {
  /** Receives the chosen file; uploading is the caller's job. */
  onFile: (file: File) => void;
  accept?: string;
  disabled?: boolean;
  busy?: boolean;
  label?: string;
  busyLabel?: string;
  hint?: string;
  /** The chosen file's name, shown instead of the hint. */
  fileName?: string | null;
  className?: string;
}

/** The one file input: drag and drop, or click to choose. */
export function FileDropzone({
  onFile,
  accept,
  disabled = false,
  busy = false,
  label = "Seret berkas ke sini atau pilih",
  busyLabel = "Mengunggah…",
  hint,
  fileName,
  className,
}: FileDropzoneProps) {
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const [over, setOver] = useState(false);
  const off = disabled || busy;

  function pick(files: FileList | null) {
    const f = files?.[0];
    if (f && !off) onFile(f);
    if (input.current) input.current.value = "";
  }

  function drop(e: DragEvent<HTMLLabelElement>) {
    e.preventDefault();
    setOver(false);
    pick(e.dataTransfer.files);
  }

  const status = busy ? busyLabel : fileName ? `Dipilih: ${fileName}` : "";
  return (
    <label
      htmlFor={id}
      aria-busy={busy || undefined}
      onDragOver={(e) => {
        e.preventDefault();
        if (!off) setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={drop}
      className={cn(
        "flex cursor-pointer flex-col items-center gap-1 rounded-lg border-2 border-dashed border-input bg-card px-4 py-6 text-center transition-colors hover:bg-background",
        // The file input is visually hidden: its keyboard focus shows on the whole zone (2.4.7).
        "has-[input:focus-visible]:border-ring has-[input:focus-visible]:outline-solid has-[input:focus-visible]:outline-2 has-[input:focus-visible]:outline-offset-2 has-[input:focus-visible]:outline-ring",
        over && "border-primary bg-accent",
        off && "cursor-not-allowed opacity-60",
        className,
      )}
    >
      <FileUp size={20} aria-hidden className="text-muted-foreground" />
      <span className="text-sm font-medium text-foreground">{busy ? busyLabel : label}</span>
      {fileName ? <span className="max-w-full truncate text-xs text-foreground">{fileName}</span> : hint ? <span id={`${id}-hint`} className="text-xs text-muted-foreground">{hint}</span> : null}
      <input
        ref={input}
        id={id}
        type="file"
        className="sr-only"
        accept={accept}
        disabled={off}
        aria-describedby={hint && !fileName ? `${id}-hint` : undefined}
        onChange={(e) => pick(e.target.files)}
      />
      {/* Uploading and the chosen file are announced without moving focus (4.1.3). */}
      <span role="status" className="sr-only">
        {status}
      </span>
    </label>
  );
}
