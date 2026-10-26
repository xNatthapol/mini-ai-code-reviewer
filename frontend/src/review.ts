export const MAX_LINES = 200;
export const MAX_BYTES = 12 * 1024;

export function normalizeCode(code: string): string {
  return code.replace(/\r\n/g, "\n");
}

export function inputError(code: string): string | null {
  const normalized = normalizeCode(code);
  if (!normalized.trim()) return "Paste a code snippet to review.";
  if (new TextEncoder().encode(normalized).length > MAX_BYTES)
    return "Keep your snippet within 12 KB (12,288 bytes).";
  if (normalized.split("\n").length > MAX_LINES)
    return "Keep your snippet within 200 lines.";
  return null;
}

export function affectedCode(code: string, start: number, end: number): string {
  return code
    .split("\n")
    .slice(start - 1, end)
    .join("\n");
}
