import { useState, type ReactNode } from "react";
import { StSegmented, StSection, StSelect, StSwitchItem, StTextField, toast } from "../../../design";
import { useOptimistic } from "../../../lib/optimistic";
import type { Params, PrintParamRules } from "../../../mygo";
import { PrintStylesController, useParamRules } from "./controller";
import {
  firstProblem, fontOptions, formatNumber, linesOptions, numberError, orientationOptions, ruleOf, pageNumberOptions, paperOptions, paperPatch,
  type NumberKey,
} from "./params";

/**
 * A style's layout parameters. A choice takes effect at once and is saved
 * in turn (lib/optimistic.ts); a typed number is saved as soon as it is a
 * good value, and the field says what is wrong until then. The built-in
 * style is shown, not changed.
 */
export function StyleParamsForm({ id, params, readOnly }: { id: string; params: Params; readOnly: boolean }) {
  const { data: rules } = useParamRules();
  return rules ? <ParamsForm id={id} params={params} readOnly={readOnly} rules={rules} /> : null;
}

function ParamsForm({ id, params, readOnly, rules }: { id: string; params: Params; readOnly: boolean; rules: PrintParamRules }) {
  const { data, save } = useOptimistic(params, (next: Params) => PrintStylesController.saveParams(id, next));
  const p = data ?? params;
  // What is wrong with the parameters as they would be; "" and saved otherwise.
  const commit = (patch: Partial<Params>) => {
    const problem = firstProblem(rules, { ...p, ...patch });
    if (!problem) save(patch);
    return problem;
  };
  const choose = (patch: Partial<Params>) => {
    const problem = commit(patch);
    if (problem) toast(problem, "danger");
  };
  const num = (key: NumberKey) => <NumberField rules={rules} field={key} value={p[key]} disabled={readOnly} onCommit={(n) => commit({ [key]: n })} />;
  const modes = <T extends string>(options: { value: T; label: ReactNode }[]) => options.map((o) => ({ ...o, disabled: readOnly }));
  return (
    <>
      <StSection title="纸张" padded={false}>
        <div className="setup-fields">
          <StSelect label="纸张" value={p.paper} disabled={readOnly} onChange={(paper) => choose(paperPatch(rules, p, paper))} options={paperOptions} />
          {p.paper === "custom" && num("paperWidth")}
          {p.paper === "custom" && num("paperHeight")}
          <StSegmented label="方向" value={p.orientation} options={modes(orientationOptions)} onChange={(orientation) => choose({ orientation })} />
        </div>
        <div className="setup-fields setup-fields--compact">
          {num("marginTop")}
          {num("marginBottom")}
          {num("marginLeft")}
          {num("marginRight")}
        </div>
      </StSection>
      <StSection title="文字" padded={false}>
        <div className="setup-fields">
          <StSelect label="字体" value={p.font} disabled={readOnly} onChange={(font) => choose({ font })} options={fontOptions} />
          {num("fontSize")}
          {num("lineHeight")}
          {num("titleSize")}
          {num("eventTitleSize")}
        </div>
      </StSection>
      <StSection title="表格" padded={false}>
        <div className="setup-fields">
          <StSegmented label="表格线" value={p.tableLines} options={modes(linesOptions)} onChange={(tableLines) => choose({ tableLines })} />
          {num("lineWidth")}
        </div>
      </StSection>
      <StSection title="页面" padded={false}>
        <StSwitchItem title="页眉" subtitle="第二页起印赛事名与报表名" checked={p.header} disabled={readOnly} onChange={(header) => choose({ header })} />
        <div className="setup-fields">
          <StSelect label="页码" value={p.pageNumber} disabled={readOnly} onChange={(pageNumber) => choose({ pageNumber })} options={pageNumberOptions} />
          {num("rosterColumns")}
        </div>
      </StSection>
      <StSection title="栏目" padded={false}>
        <StSwitchItem title="单位" checked={p.showUnit} disabled={readOnly} onChange={(showUnit) => choose({ showUnit })} />
        <StSwitchItem title="报名成绩" checked={p.showSeed} disabled={readOnly} onChange={(showSeed) => choose({ showSeed })} />
        <StSwitchItem title="号码" checked={p.showBib} disabled={readOnly} onChange={(showBib) => choose({ showBib })} />
      </StSection>
    </>
  );
}

/** A number field: it shows what is typed until it is saved, and the saved value after. */
function NumberField({ rules, field, value, disabled, onCommit }: { rules: PrintParamRules; field: NumberKey; value: number; disabled: boolean; onCommit: (n: number) => string }) {
  const rule = ruleOf(rules, field);
  const [text, setText] = useState<string | null>(null);
  // What is wrong with the parameters as a whole, when the typed number is good on its own.
  const [problem, setProblem] = useState("");
  const error = text === null ? "" : numberError(rules, field, text) || problem;
  const change = (v: string) => {
    setText(v);
    setProblem(numberError(rules, field, v) === "" ? onCommit(Number(v)) : "");
  };
  return (
    <StTextField label={rule.label} suffix={rule.unit} inputMode="decimal" width="fill" disabled={disabled} align="end"
      value={text ?? formatNumber(value)} error={error} onChange={change}
      onBlur={() => { if (!error) setText(null); }} />
  );
}
