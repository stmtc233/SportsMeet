import { useState } from "react";
import { StButton, StCheckbox, StDialog, StSearchField, StTable } from "../../design";
import type { Athlete, CombinedDetail } from "../../mygo";
import { CombinedController } from "./controller";

export function CombinedEntriesDialog({ detail, onClose }: { detail: CombinedDetail; onClose: () => void }) {
  const [selected, setSelected] = useState(new Set(detail.athleteIds));
  const [sameGroup, setSameGroup] = useState(true);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const save = async () => { setBusy(true); const ok = await CombinedController.entries(detail.combined.id, [...selected]); setBusy(false); if (ok !== undefined) onClose(); };
  return <StDialog open title="全能报名" size="large" onClose={() => !busy && onClose()} onSubmit={save}
    actions={<><StButton disabled={busy} onClick={onClose}>取消</StButton><StButton variant="primary" loading={busy} type="submit">保存报名</StButton></>}>
    {detail.combined.groupId && <StCheckbox checked={sameGroup} onChange={setSameGroup} label="只看同组别" />}
    <StSearchField value={query} onChange={setQuery} placeholder="搜索姓名或号码" />
    <div inert={busy}><StTable<Athlete> rows={detail.athletes.filter((a) => `${a.name} ${a.bib}`.includes(query) && (!sameGroup || !detail.combined.groupId || a.groupId === detail.combined.groupId || selected.has(a.id)))} rowKey={(a) => a.id} selected={selected} onSelect={setSelected} dense columns={[
      { key: "bib", title: "号码", width: "96px", render: (a) => a.bib },
      { key: "name", title: "姓名", render: (a) => a.name },
      { key: "gender", title: "性别", render: (a) => a.gender },
    ]} /></div>
  </StDialog>;
}
