// Form input for dates and scheduled times (docs/agents/domain.md「时间」):
// a scheduled time is stored as YYYY-MM-DDTHH:mm and edited as a date field
// plus a time field. This file only splits and joins those fields; Go
// validates them and formats every displayed time.

export interface WallTimeFields {
  date: string;
  time: string;
}

/** Splits a stored scheduled time into its date and time fields; "" gives empty fields. */
export function splitScheduled(at: string): WallTimeFields {
  const [date = "", time = ""] = at.split("T");
  return { date, time };
}

/** Joins the fields back; a missing date or time means unscheduled (""). */
export function joinScheduled(date: string, time: string): string {
  return date && time ? `${date}T${time}` : "";
}

/** Today's date on this computer, YYYY-MM-DD, as a form default. */
export function today(now = new Date()): string {
  const p = (n: number) => String(n).padStart(2, "0");
  return `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}`;
}
