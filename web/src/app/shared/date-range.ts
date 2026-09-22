/**
 * Today as `YYYY-MM-DD` in the BROWSER's local time.
 *
 * `toISOString()` would be wrong here: it converts to UTC first, so anywhere
 * east of Greenwich the date flips a day early in the evening. Shared by
 * every report screen's date-range filter, all of which send this naive
 * local string straight to the server — see docs/extraction-plan.md §4.3
 * rule 1 for why the server-side handling of these strings matters just as
 * much as this function does.
 */
export function today(): string {
  const now = new Date();
  const month = `${now.getMonth() + 1}`.padStart(2, '0');
  const day = `${now.getDate()}`.padStart(2, '0');
  return `${now.getFullYear()}-${month}-${day}`;
}

/** Seconds as m:ss, for any report row with a duration/talk/hold field. */
export function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${s.toString().padStart(2, '0')}`;
}
