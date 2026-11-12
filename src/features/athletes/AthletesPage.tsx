import { useMemo, useState } from "react";
import { IconAdd, IconImport, StButton, StEmpty, StPage, StSearchField, StSelect, StTable, StTag, StToolbar, iconStroke } from "../../design";
import type { Athlete, AthleteRow } from "../../mygo";
import { DeleteDialog } from "../meets/DeleteDialog";
import { useUnits } from "../units/UnitsPage";
import { AthleteDialog } from "./AthleteDialog";
import { AthletesController, useAthletes } from "./controller";
import { ImportDialog } from "./ImportDialog";

export function AthletesPage({ meetId }: { meetId: string }) {
  const { data: rows } = useAthletes(meetId);
  const { data: units } = useUnits(meetId);
  const [query, setQuery] = useState("");
  const [unit, setUnit] = useState("");
  const [selected, setSelected] = useState(new Set<string>());
  const [editing, setEditing] = useState<{ athlete?: Athlete } | null>(null);
  const [importing, setImporting] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const shown = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (rows ?? []).filter((r) => (unit === "" || r.athlete.unitId === unit) && (q === "" || [r.athlete.name, r.athlete.bib, r.athlete.studentNumber, r.athlete.className].some((v) => v.toLowerCase().includes(q))));
  }, [rows, query, unit]);
  const picked = shown.filter((r) => selected.has(r.athlete.id));

  return (
    <StPage flush
      toolbar={
        <StToolbar title="运动员" subtitle={rows ? `${rows.length} 人` : ""}>
          <StButton variant="danger" disabled={picked.length === 0} onClick={() => setDeleting(true)} reserveWidthFor={["删除 999 人"]}>
            {picked.length > 0 ? `删除 ${picked.length} 人` : "删除"}
          </StButton>
          <StButton icon={<IconImport size={16} strokeWidth={iconStroke} />} onClick={() => setImporting(true)}>批量导入</StButton>
          <StButton variant="primary" icon={<IconAdd size={16} strokeWidth={iconStroke} />} onClick={() => setEditing({})}>添加运动员</StButton>
        </StToolbar>
      }>
      <div className="filters">
        <StSearchField value={query} onChange={setQuery} placeholder="搜索姓名、号码、学号" />
        <StSelect value={unit} onChange={setUnit} placeholder="全部单位" options={[{ value: "", label: "全部单位" }, ...(units ?? []).map((u) => ({ value: u.id, label: u.name }))]} />
      </div>
      <div className="table-card">
        <StTable<AthleteRow> fill rows={shown} rowKey={(r) => r.athlete.id} selected={selected} onSelect={setSelected} onRowClick={(r) => setEditing({ athlete: r.athlete })}
          empty={<StEmpty>{rows?.length === 0 ? "还没有运动员" : "没有符合条件的运动员"}</StEmpty>}
          columns={[
            { key: "bib", title: "号码", width: "88px", render: (r) => r.athlete.bib },
            { key: "name", title: "姓名", width: "minmax(120px, 1.2fr)", render: (r) => <span className="cell-strong">{r.athlete.name}</span> },
            { key: "gender", title: "性别", width: "64px", render: (r) => r.athlete.gender },
            { key: "unit", title: "参赛单位", width: "minmax(120px, 1.5fr)", render: (r) => r.unitName },
            { key: "class", title: "班级", width: "minmax(96px, 1fr)", render: (r) => [r.athlete.grade, r.athlete.className].filter(Boolean).join(" ") },
            { key: "number", title: "学号", width: "minmax(96px, 1fr)", render: (r) => r.athlete.studentNumber },
            { key: "entries", title: "报名", width: "88px", align: "end", render: (r) => (r.entries > 0 ? <StTag tone={r.placed ? "info" : "neutral"}>{r.entries} 项</StTag> : "") },
          ]} />
      </div>
      <AthleteDialog meetId={meetId} open={editing !== null} athlete={editing?.athlete} units={units ?? []} onClose={() => setEditing(null)} />
      <ImportDialog meetId={meetId} open={importing} onClose={() => setImporting(false)} />
      <DeleteDialog open={deleting} title={`删除 ${picked.length} 名运动员？`} onClose={() => setDeleting(false)}
        onDelete={async () => {
          const ok = await AthletesController.remove(meetId, picked.map((r) => r.athlete.id));
          if (ok) setSelected(new Set());
          return ok;
        }}>
        他们的报名会一并删除，无法撤销。
      </DeleteDialog>
    </StPage>
  );
}
