import type { StOption } from "../../design";

/**
 * Choices for a setting in minutes: the given values, 0 shown as zero, and
 * the stored value appended when it is not among them so it stays visible.
 */
export function minuteOptions(values: number[], current: number, zero: string): StOption<string>[] {
  const list = values.includes(current) ? values : [...values, current];
  return list.map((m) => ({ value: String(m), label: m === 0 ? zero : `${m} 分钟` }));
}
