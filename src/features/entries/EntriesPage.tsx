import { useState } from "react";
import { StButton, StEmpty, StPage, StSegmented, StTable, StTag, StToolbar, type StColumn } from "../../design";
import type { EntryForm, GroupEntries, UnitEntries } from "../../mygo";
import { useNav } from "../../app/nav";
import { remember, useViewState } from "../../lib/viewState";
import { ImportDialog } from "../athletes/ImportDialog";
import { EntriesController, useEntrySummary } from "./controller";
import { EntryFormDialog } from "./EntryFormDialog";

type Mode = "unit" | "group";

// The group rows key "" (no group) as this, a table needing non-empty keys.
const noGroupKey = "-";

/** 报名: hand out the 报名登记表, take them back, and see whose are still out. */
export function EntriesPage({ meetId }: { meetId: string }) {
  const { go } = useNav();
  const { data } = useEntrySummary(meetId);
  const [mode, setMode] = useViewState<Mode>(`entries:mode:${meetId}`, "unit");
  const [selectedUnits, setSelectedUnits] = useState(new Set<string>());
  const [selectedGroups, setSelectedGroups] = useState(new Set<string>());
  const [forms, setForms] = useState<EntryForm[] | null>(null);
  const [pasting, setPasting] = useState(false);

  const units = data?.units;
  const groups = data?.groups ?? [];
  const byGroup = mode === "group" && groups.length > 0;
  const selected = byGroup ? selectedGroups : selectedUnits;
  const entered = (units ?? []).filter((u) => u.entries > 0 || u.teams > 0).length;

  const pickForms = async () => {
    const picked = await EntriesController.pickForms();
    if (picked.length > 0) setForms(picked);
  };
  const exportForms = () => {
    const ids = [...selected].map((id) => (id === noGroupKey ? "" : id));
    void EntriesController.exportForms(meetId, ids, byGroup);
  };
  // A row opens 运动员 filtered to it.
  const showAthletes = (unit: string, group: string) => {
    remember(`athletes:unit:${meetId}`, unit);
    remember(`athletes:group:${meetId}`, group);
    go({ page: "meet", meetId, tab: "athletes" });
  };

  const status = (entries: number, teams: number, over: number) => {
    if (entries === 0 && teams === 0) return <StTag>未报名</StTag>;
    if (over > 0) return <StTag tone="warning">超限 {over}</StTag>;
    return null;
  };
  const counts = <T extends { athletes: number; entries: number }>(): StColumn<T>[] => [
    { key: "athletes", title: "运动员", width: "72px", align: "end", render: (r) => r.athletes },
    { key: "entries", title: "报名项次", width: "88px", align: "end", render: (r) => r.entries },
  ];
  const unitColumns: StColumn<UnitEntries>[] = [
    { key: "name", title: "参赛单位", width: "minmax(120px, 2fr)", render: (r) => <span className="cell-strong">{r.unit.name}</span> },
    ...(groups.length > 0 ? [{ key: "groups", title: "组别", width: "minmax(80px, 1fr)", render: (r: UnitEntries) => (
      <span className="cell-tags">{groups.filter((g) => g.id && r.unit.groupIds?.includes(g.id)).map((g) => <StTag key={g.id}>{g.name}</StTag>)}</span>
    ) }] : []),
    ...counts<UnitEntries>(),
    { key: "teams", title: "队伍", width: "64px", align: "end", render: (r) => r.teams },
    { key: "status", title: "", width: "minmax(72px, 1fr)", render: (r) => status(r.entries, r.teams, r.over) },
  ];
  const groupColumns: StColumn<GroupEntries>[] = [
    { key: "name", title: "组别", width: "minmax(120px, 2fr)", render: (r) => <span className="cell-strong">{r.name}</span> },
    { key: "units", title: "参赛单位", width: "88px", align: "end", render: (r) => r.units },
    ...counts<GroupEntries>(),
    { key: "status", title: "", width: "minmax(72px, 1fr)", render: (r) => status(r.entries, 0, 0) },
  ];
  const empty = <StEmpty action={<StButton variant="primary" onClick={() => go({ page: "meet", meetId, tab: "units" })}>添加单位</StButton>}>还没有参赛单位</StEmpty>;

  return (
    <StPage flush
      toolbar={
        <StToolbar title="报名" subtitle={units?.length ? `${entered} / ${units.length} 已报名` : ""}>
          {groups.length > 0 && (
            <StSegmented size="small" aria-label="登记表" value={byGroup ? "group" : "unit"} onChange={setMode}
              options={[{ value: "unit", label: "按单位" }, { value: "group", label: "按组别" }]} />
          )}
          <StButton disabled={!units?.length} onClick={() => setPasting(true)}>粘贴表格</StButton>
          <StButton disabled={!units?.length} reserveWidthFor={["导出登记表", "导出 99 份"]} onClick={exportForms}>
            {selected.size > 0 ? `导出 ${selected.size} 份` : "导出登记表"}
          </StButton>
          <StButton variant="primary" disabled={!units?.length} onClick={pickForms}>导入登记表</StButton>
        </StToolbar>
      }>
      <div className="table-card table-card--top view-enter" key={byGroup ? "group" : "unit"}>
        {byGroup ? (
          <StTable<GroupEntries> fill rows={units?.length ? groups : []} loading={!data} scrollKey={`entries:groups:${meetId}`}
            rowKey={(g) => g.id || noGroupKey} rowLabel={(g) => g.name} selected={selectedGroups} onSelect={setSelectedGroups}
            onRowClick={(g) => showAthletes("", g.id)} empty={empty} columns={groupColumns} />
        ) : (
          <StTable<UnitEntries> fill rows={units ?? []} loading={!data} scrollKey={`entries:units:${meetId}`}
            rowKey={(r) => r.unit.id} rowLabel={(r) => r.unit.name} selected={selectedUnits} onSelect={setSelectedUnits}
            onRowClick={(r) => showAthletes(r.unit.id, "all")} empty={empty} columns={unitColumns} />
        )}
      </div>
      <EntryFormDialog meetId={meetId} forms={forms} onPick={pickForms} onClose={() => setForms(null)} />
      <ImportDialog meetId={meetId} open={pasting} onClose={() => setPasting(false)} />
    </StPage>
  );
}
