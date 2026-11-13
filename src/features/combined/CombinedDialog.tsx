import { useState } from "react";
import { StButton, StDialog, StNotice, StSection, StSegmented, StSelect, StTextField } from "../../design";
import type { CombinedComponentInput, CombinedDetail, EventInput } from "../../mygo";
import { GroupSelect } from "../groups/GroupFields";
import { useEventPresets } from "../events/controller";
import { blankEvent, EventMeasurementFields } from "../events/EventMeasurementFields";
import { CombinedController, emptyRule, inputFromEvent } from "./controller";
import { PointRuleEditor } from "./PointRuleEditor";

const emptyPart = (): CombinedComponentInput => ({ event: { ...blankEvent }, rule: emptyRule() });
export function CombinedDialog({ meetId, detail, initialGroupId = "", onClose }: { meetId: string; detail?: CombinedDetail; initialGroupId?: string; onClose: () => void }) {
  const { data: presets } = useEventPresets();
  const [name, setName] = useState(detail?.combined.name ?? "");
  const [gender, setGender] = useState<string>(detail?.combined.gender ?? "男");
  const [groupId, setGroupId] = useState(detail?.combined.groupId ?? initialGroupId);
  const [parts, setParts] = useState<CombinedComponentInput[]>(() => detail ? detail.components.map((c) => ({ eventId: c.event.id, event: inputFromEvent(c.event), rule: c.input })) : [emptyPart(), emptyPart()]);
  const [busy, setBusy] = useState(false);
  const locked = detail?.hasPlacements ?? false;
  const set = (index: number, patch: Partial<CombinedComponentInput>) => setParts((old) => old.map((p, i) => i === index ? { ...p, ...patch } : p));
  const save = async () => {
    setBusy(true);
    const result = await CombinedController.save(meetId, detail?.combined.id ?? "", { name, gender, groupId, groupLabel: "", note: detail?.combined.note ?? "", components: parts });
    setBusy(false); if (result) onClose();
  };
  return <StDialog open title={detail ? "编辑全能" : "添加全能"} size="large" onClose={() => !busy && onClose()} onSubmit={save}
    actions={<><StButton disabled={busy} onClick={onClose}>取消</StButton><StButton variant="primary" loading={busy} type="submit">保存</StButton></>}>
    <div className="form-grid" inert={busy}>
      <StTextField label="全能名称" value={name} onChange={setName} autoFocus />
      <GroupSelect meetId={meetId} value={groupId} onChange={setGroupId} />
      <StSegmented label="性别" value={gender} onChange={setGender} options={[{ value: "男", label: "男", disabled: locked }, { value: "女", label: "女", disabled: locked }, { value: "混合", label: "混合", disabled: locked }]} />
    </div>
    {detail && <StNotice>计分表请在「计分规则」中修改</StNotice>}
    {parts.map((part, i) => <StSection key={i} title={`分项 ${i + 1}`} actions={<StButton size="small" disabled={busy || locked || parts.length <= 2} onClick={() => setParts((old) => old.filter((_, n) => n !== i))}>移除</StButton>}>
      <div className="form-grid" inert={busy || locked}>
        {(!detail || !part.eventId) && <div className="span"><StSelect label="常用项目" width="fill" value="" placeholder={part.event.discipline || "请选择"} options={(presets ?? []).filter((p) => p.input.kind !== "relay" && p.input.participantMode !== "team").map((p) => ({ value: p.id, label: `${p.category} · ${p.label}` }))}
          onChange={(id) => { const p = presets?.find((p) => p.id === id); if (p) set(i, { event: { ...blankEvent, ...p.input } }); }} /></div>}
        <StTextField label="项目名" value={part.event.discipline} onChange={(discipline) => set(i, { event: { ...part.event, discipline } })} />
        <EventMeasurementFields form={{ ...blankEvent, ...part.event }} allowTeams={false} onChange={(patch: Partial<Required<EventInput>>) => set(i, { event: { ...part.event, ...patch } })} />
        {(!detail || !part.eventId) && <PointRuleEditor event={part.event} value={part.rule} onChange={(rule) => set(i, { rule })} />}
      </div>
    </StSection>)}
    <StButton disabled={busy || locked} onClick={() => setParts((old) => [...old, emptyPart()])}>添加分项</StButton>
  </StDialog>;
}
