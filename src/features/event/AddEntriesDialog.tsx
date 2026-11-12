import { useEffect, useMemo, useState } from "react";
import { StButton, StCheckbox, StDialog, StEmpty, StRow, StSearchField, StSelect, StTable } from "../../design";
import type { AthleteRow, EventDetail } from "../../mygo";
import { useAthletes } from "../athletes/controller";
import { EventsController } from "../events/controller";
import { AddTeamsDialog } from "./AddTeamsDialog";
import { useUnits } from "../units/UnitsPage";

/** Picks athletes of the meet to enter into the event, several at once. */
type Props = { open: boolean; detail: EventDetail; meetId: string; onClose: () => void };
export function AddEntriesDialog(props: Props) {
 return props.detail.event.kind === "relay" ? <AddTeamsDialog {...props} /> : <AddAthletesDialog {...props} />;
}
function AddAthletesDialog({ open, detail, meetId, onClose }: Props) {
  const { data: athletes } = useAthletes(meetId);
  const { data: units } = useUnits(meetId);
  const [query, setQuery] = useState("");
  const [unit, setUnit] = useState("");
  const [sameGender, setSameGender] = useState(true);
  const [selected, setSelected] = useState(new Set<string>());
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (open) {
      setSelected(new Set());
      setQuery("");
    }
  }, [open]);
  const gender = detail.event.gender;
  const rows = useMemo(() => {
    const entered = new Set(detail.entries.map((e) => e.athleteId));
    const q = query.trim().toLowerCase();
    return (athletes ?? []).filter((r) => !entered.has(r.athlete.id) && (unit === "" || r.athlete.unitId === unit)
      && (!sameGender || gender === "混合" || r.athlete.gender === gender)
      && (q === "" || [r.athlete.name, r.athlete.bib, r.athlete.studentNumber].some((v) => v.toLowerCase().includes(q))));
  }, [athletes, detail.entries, query, unit, sameGender, gender]);
  const add = async () => {
    setBusy(true);
    const ok = await EventsController.addEntries(detail.event.id, [...selected].map((athleteId) => ({ athleteId, seed: "" })));
    setBusy(false);
    if (ok) onClose();
  };
  return (
    <StDialog open={open} title={`报名「${detail.event.name}」`} size="large" onClose={onClose}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          <StButton variant="primary" disabled={selected.size === 0} loading={busy} onClick={add} reserveWidthFor={["报名 999 人"]}>
            {selected.size > 0 ? `报名 ${selected.size} 人` : "报名"}
          </StButton>
        </>
      }>
      <StRow>
        <StSearchField value={query} onChange={setQuery} placeholder="搜索姓名、号码" />
        <StSelect value={unit} onChange={setUnit} placeholder="全部单位" options={[{ value: "", label: "全部单位" }, ...(units ?? []).map((u) => ({ value: u.id, label: u.name }))]} />
        {gender !== "混合" && <StCheckbox checked={sameGender} onChange={setSameGender} label={`只看${gender}生`} />}
      </StRow>
      <div className="preview-table">
        <StTable<AthleteRow> dense rows={rows} rowKey={(r) => r.athlete.id} selected={selected} onSelect={setSelected}
          onRowClick={(r) => setSelected((old) => {
            const next = new Set(old);
            if (!next.delete(r.athlete.id)) next.add(r.athlete.id);
            return next;
          })}
          empty={<StEmpty>没有可报名的运动员</StEmpty>}
          columns={[
            { key: "bib", title: "号码", width: "80px", render: (r) => r.athlete.bib },
            { key: "name", title: "姓名", render: (r) => r.athlete.name },
            { key: "gender", title: "性别", width: "64px", render: (r) => r.athlete.gender },
            { key: "unit", title: "参赛单位", render: (r) => r.unitName },
          ]} />
      </div>
    </StDialog>
  );
}
