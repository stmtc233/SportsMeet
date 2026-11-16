import type { StOption } from "../../../design";
import type { NumberLimit, Params, PrintParamRules } from "../../../mygo";

// The layout parameters of a print style as the form takes them. The
// limits come from Go (PrintStyles.paramRules, the ones Params.Validate
// checks); they are checked here only so a wrong value is shown at its
// field before it is sent.

export type NumberKey =
  | "paperWidth" | "paperHeight"
  | "marginTop" | "marginRight" | "marginBottom" | "marginLeft"
  | "fontSize" | "lineHeight" | "titleSize" | "eventTitleSize" | "lineWidth" | "rosterColumns";

/** The limit Go gives for a field. */
export function ruleOf(rules: PrintParamRules, key: NumberKey): NumberLimit {
  const rule = rules.numbers.find((r) => r.key === key);
  if (!rule) throw new Error(`no limit for ${key}`);
  return rule;
}

export const paperOptions: StOption<string>[] = [
  { value: "a4", label: "A4" }, { value: "a3", label: "A3" }, { value: "b5", label: "B5" },
  { value: "16k", label: "16 开" }, { value: "letter", label: "Letter" }, { value: "custom", label: "自定义" },
];

export const orientationOptions: StOption<string>[] = [{ value: "portrait", label: "纵向" }, { value: "landscape", label: "横向" }];

export const fontOptions: StOption<string>[] = [
  { value: "sans", label: "无衬线" }, { value: "hei", label: "黑体" }, { value: "song", label: "宋体" }, { value: "kai", label: "楷体" }, { value: "fang", label: "仿宋" },
];

export const linesOptions: StOption<string>[] = [{ value: "rules", label: "只有横线" }, { value: "grid", label: "全框线" }];

export const pageNumberOptions: StOption<string>[] = [
  { value: "n_of_m", label: "第 N 页 / 共 M 页" }, { value: "dash", label: "- N -" }, { value: "none", label: "不印" },
];

/** A number as typed, or undefined when the text is not one (empty, letters, NaN). */
export function parseNumber(text: string): number | undefined {
  const t = text.trim();
  if (!/^-?\d+(\.\d*)?$|^-?\.\d+$/.test(t)) return undefined;
  const n = Number(t);
  return Number.isFinite(n) ? n : undefined;
}

/** A number as the field shows it: no trailing zeros. */
export function formatNumber(n: number): string {
  return String(Math.round(n * 1000) / 1000);
}

/** What is wrong with the text typed for a field; "" when it is a good value. */
export function numberError(rules: PrintParamRules, key: NumberKey, text: string): string {
  const rule = ruleOf(rules, key);
  const n = parseNumber(text);
  if (n === undefined) return "请输入数字";
  if (rule.integer && !Number.isInteger(n)) return "请输入整数";
  if (n < rule.min || n > rule.max) return `${rule.label}需在 ${formatNumber(rule.min)}–${formatNumber(rule.max)}${/^[a-z]+$/.test(rule.unit) ? ` ${rule.unit} ` : ` ${rule.unit}`}之间`;
  return "";
}

/** A named sheet's size in mm, portrait, or the custom size. */
function sheet(rules: PrintParamRules, p: Params): [number, number] {
  const [w, h] = rules.papers[p.paper] ?? [];
  return w !== undefined && h !== undefined ? [w, h] : [p.paperWidth, p.paperHeight];
}

/** The sheet in mm with the orientation applied. */
export function paperMM(rules: PrintParamRules, p: Params): [number, number] {
  const [w, h] = sheet(rules, p);
  return p.orientation === "landscape" ? [h, w] : [w, h];
}

/** The margins leave less of the sheet than Go allows to print in. */
export function marginsTooBig(rules: PrintParamRules, p: Params): boolean {
  const [w, h] = paperMM(rules, p);
  return w - p.marginLeft - p.marginRight < rules.minContent || h - p.marginTop - p.marginBottom < rules.minContent;
}

/** The first problem of a whole set of parameters, in Chinese; "" when all is well. */
export function firstProblem(rules: PrintParamRules, p: Params): string {
  for (const { key } of rules.numbers) {
    if ((key === "paperWidth" || key === "paperHeight") && p.paper !== "custom") continue;
    const problem = numberError(rules, key as NumberKey, formatNumber(p[key as NumberKey]));
    if (problem) return problem;
  }
  return marginsTooBig(rules, p) ? rules.marginsTooBig : "";
}

/** Picking another paper: a custom sheet starts from the size of the one it replaces. */
export function paperPatch(rules: PrintParamRules, current: Params, paper: string): Partial<Params> {
  if (paper !== "custom" || current.paper === "custom") return { paper };
  const [w, h] = sheet(rules, current);
  return { paper, paperWidth: w, paperHeight: h };
}
