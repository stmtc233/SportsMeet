import { StSegmented, StSelect, StTextField } from "../../design";
import type { EventInput } from "../../mygo";
import { kindLabels } from "../../lib/labels";
import { useEventRules } from "./controller";

export { blankEvent } from "./form";
const int = (text: string) => Number.parseInt(text, 10) || 0;

export function EventMeasurementFields({ form, onChange, allowTeams = true }: { form: Required<EventInput>; onChange: (patch: Partial<Required<EventInput>>) => void; allowTeams?: boolean }) {
  const rules = useEventRules(form, true);
  const scalar = form.kind === "count" || form.kind === "score";
  const timed = form.kind === "track" || form.kind === "relay";
  const team = form.kind === "relay" || form.participantMode === "team";
  const changeKind = (kind: string) => onChange({ kind, startMode: kind === "track" || kind === "relay" ? "lanes" : "", distanceMode: kind === "distance" ? "jump" : "", relayLegCount: 0, participantMode: kind === "relay" ? "team" : "athlete", teamSize: 0, scoreDecimals: kind === "score" ? 2 : 0, rankDirection: kind === "count" || kind === "score" ? "desc" : "", attemptMode: kind === "count" || kind === "score" ? "single" : "", attemptCount: kind === "count" || kind === "score" ? 1 : 3 });
  return <>
    <StSelect label="类型" value={form.kind} onChange={changeKind} options={Object.entries(kindLabels).filter(([k]) => allowTeams || k !== "relay").map(([value, label]) => ({ value, label }))} />
    {allowTeams && form.kind !== "relay" && (timed || scalar) && <StSegmented label="参赛方式" value={form.participantMode || "athlete"} onChange={(participantMode) => onChange({ participantMode, teamSize: 0 })} options={[{ value: "athlete", label: "个人" }, { value: "team", label: "队伍" }]} />}
    {timed && <div className="span"><StSegmented label="起跑方式" value={form.startMode || "lanes"} onChange={(startMode) => onChange({ startMode })} options={[{ value: "lanes", label: "分道" }, { value: "group", label: "不分道" }, { value: "break", label: "分道后抢道" }]} /></div>}
    {timed && <StTextField label="道数" disabled={form.startMode === "group"} inputMode="numeric" value={String(form.laneCount || "")} onChange={(v) => onChange({ laneCount: int(v) })} />}
    {form.kind === "distance" && <StSegmented label="远度方式" value={form.distanceMode || "jump"} onChange={(distanceMode) => onChange({ distanceMode })} options={[{ value: "jump", label: "跳跃" }, { value: "throw", label: "投掷" }]} />}
    {form.kind === "score" && <StSegmented label="小数位" value={String(form.scoreDecimals)} onChange={(v) => onChange({ scoreDecimals: int(v) })} options={[0, 1, 2, 3].map((v) => ({ value: String(v), label: String(v) }))} />}
    {scalar && <StSegmented label="排名方向" value={form.rankDirection || "desc"} onChange={(rankDirection) => onChange({ rankDirection })} options={[{ value: "desc", label: "多者优先" }, { value: "asc", label: "少者优先" }]} />}
    {scalar && <StSegmented label="成绩汇总" value={form.attemptMode || "single"} onChange={(attemptMode) => onChange({ attemptMode, attemptCount: attemptMode === "single" ? 1 : 3 })} options={[{ value: "single", label: "一次" }, { value: "best", label: "取最好" }, { value: "sum", label: "求和" }]} />}
    {(form.kind === "distance" || (scalar && form.attemptMode !== "single" && form.attemptMode !== "")) && <StTextField label={`${rules?.attemptLabel || "尝试"}次数`} inputMode="numeric" value={String(form.attemptCount || "")} onChange={(v) => onChange({ attemptCount: int(v) })} />}
    {form.kind === "relay" && <StTextField label="建议棒数" placeholder="未指定" inputMode="numeric" value={String(form.relayLegCount || "")} onChange={(v) => onChange({ relayLegCount: int(v) })} />}
    {team && form.kind !== "relay" && <StTextField label="建议人数" placeholder="未指定" inputMode="numeric" value={String(form.teamSize || "")} onChange={(v) => onChange({ teamSize: int(v) })} />}
  </>;
}
