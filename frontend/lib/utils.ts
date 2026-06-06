// lib/utils.ts

/**
 * Returns a human-readable Indonesian relative time string for the given ISO 8601 date.
 *
 * - < 1 minute ago: "baru saja"
 * - < 60 minutes ago: "{n} menit yang lalu"
 * - < 24 hours ago: "{n} jam yang lalu"
 * - < 30 days ago: "{n} hari yang lalu"
 * - older: formatted via `date.toLocaleDateString('id-ID')`
 */
export function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMinutes = Math.floor(diffMs / 60000);

  if (diffMinutes < 1) return 'baru saja';
  if (diffMinutes < 60) return `${diffMinutes} menit yang lalu`;
  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return `${diffHours} jam yang lalu`;
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 30) return `${diffDays} hari yang lalu`;
  return date.toLocaleDateString('id-ID');
}

/**
 * Returns a countdown string in Indonesian for the given remaining seconds.
 *
 * - If seconds <= 0: "Kadaluarsa"
 * - Otherwise: composed of non-zero day/hour/minute components, space-separated,
 *   ending with " tersisa" (e.g. "1 hari 1 jam 1 menit tersisa").
 */
export function formatRemainingTime(seconds: number): string {
  if (seconds <= 0) return 'Kadaluarsa';
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  const parts: string[] = [];
  if (days > 0) parts.push(`${days} hari`);
  if (hours > 0) parts.push(`${hours} jam`);
  if (minutes > 0) parts.push(`${minutes} menit`);
  return parts.join(' ') + ' tersisa';
}

/**
 * Returns a human-readable file size string.
 *
 * - < 1024 bytes: "{n} B"
 * - < 1 MiB: "{n.n} KB" (one decimal place)
 * - >= 1 MiB: "{n.n} MB" (one decimal place)
 */
export function formatFileSize(bytes: number): string {
  const KB = 1024;
  const MB = 1024 * 1024;

  if (bytes < KB) return `${bytes} B`;

  // KB regime. Guard against rounding the displayed value up to 1024.0, which
  // would violate the "numeric value < 1024 for B/KB" property. When the
  // one-decimal rounded value reaches 1024, promote to the next unit (MB).
  if (bytes < MB) {
    const kb = bytes / KB;
    if (parseFloat(kb.toFixed(1)) < 1024) return `${kb.toFixed(1)} KB`;
  }

  return `${(bytes / MB).toFixed(1)} MB`;
}

/**
 * Maps a language identifier to a file extension. Returns ".txt" for unknown languages.
 */
export function getFileExtension(language: string): string {
  const extensions: Record<string, string> = {
    javascript: '.js',
    typescript: '.ts',
    python: '.py',
    go: '.go',
    rust: '.rs',
    java: '.java',
    cpp: '.cpp',
    c: '.c',
    html: '.html',
    css: '.css',
    json: '.json',
    plaintext: '.txt',
  };
  return extensions[language] || '.txt';
}
