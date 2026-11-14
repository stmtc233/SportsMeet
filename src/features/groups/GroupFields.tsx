import { useState } from "react";
import { StButton, StCheckbox, StMenu, StMuted, StRow, StSelect, StStack, StTextField } from "../../design";
import { useViewState } from "../../lib/viewState";
import type { CompetitionGroup } from "../../mygo";
import { GroupsController, useGroups } from "./controller";

/** Shown for "" everywhere: an event, unit or athlete without a group. */
export const ungrouped = "未分组";

/**
 * A remembered group filter: "all", "" (ungrouped) or a group id. A group
 * deleted meanwhile reads as "all".
 */
export function useGroupFilter(meetId: string, page: string): [string, (id: string) => void] {
  const { data: groups } = useGroups(meetId);
  const [group, setGroup] = useViewState(`${page}:group:${meetId}`, "all");
  const known = group === "all" || group === "" || !groups || groups.some((g) => g.id === group);
  return [known ? group : "all", setGroup];
}

/** Whether a row with groupId passes the filter. */
export function inGroup(filter: string, groupId: string | undefined) {
  return filter === "all" || (groupId ?? "") === filter;
}

/** The group filter of a list page. */
export function GroupFilter({ meetId, value, onChange }: { meetId: string; value: string; onChange: (id: string) => void }) {
  const { data: groups } = useGroups(meetId);
  return (
    <StSelect aria-label="组别" value={value} onChange={onChange}
      options={[{ value: "all", label: "全部组别" }, ...(groups ?? []).map((g) => ({ value: g.id, label: g.name })), { value: "", label: ungrouped }]} />
  );
}

// The option of GroupSelect that adds a group instead of picking one.
const newGroup = "+new";

/**
 * The one group of an event, combined event or athlete. 「新建组别…」 turns
 * it into a name field right there, so a group is made where it is needed.
 */
export function GroupSelect({ meetId, value, onChange, disabled }: { meetId: string; value: string; onChange: (id: string) => void; disabled?: boolean }) {
  const { data: groups } = useGroups(meetId);
  const [name, setName] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // The group just made shows until the list read again has it.
  const [made, setMade] = useState<CompetitionGroup | null>(null);
  const options = [...(groups ?? []), ...(made && !groups?.some((g) => g.id === made.id) ? [made] : [])];
  const known = value === "" || !groups || options.some((g) => g.id === value);

  const create = async () => {
    if (!name?.trim()) return;
    setBusy(true);
    const group = await GroupsController.create(meetId, name.trim());
    setBusy(false);
    if (!group) return;
    setMade(group);
    setName(null);
    onChange(group.id);
  };
  if (name !== null) {
    return (
      // Enter adds the group rather than submitting the dialog around it.
      <div className="group-new" onKeyDownCapture={(e) => {
        if (e.key === "Enter" && !e.nativeEvent.isComposing) {
          e.preventDefault();
          e.stopPropagation();
          void create();
        } else if (e.key === "Escape") {
          e.preventDefault();
          e.stopPropagation();
          setName(null);
        }
      }}>
        <StTextField label="新组别" placeholder="初一" autoFocus width="fill" value={name} onChange={setName} disabled={busy} />
        <StButton disabled={busy} onClick={() => setName(null)}>取消</StButton>
        <StButton variant="primary" loading={busy} disabled={!name.trim()} onClick={create}>添加</StButton>
      </div>
    );
  }
  return (
    <StSelect label="组别" width="fill" value={known ? value : ""} onChange={(v) => (v === newGroup ? setName("") : onChange(v))} disabled={disabled}
      options={[{ value: "", label: ungrouped }, ...options.map((g) => ({ value: g.id, label: g.name })), { value: newGroup, label: "新建组别…" }]} />
  );
}

/** The groups of a unit; a unit may take part in several. */
export function GroupChecks({ meetId, value, onChange }: { meetId: string; value: string[]; onChange: (ids: string[]) => void }) {
  const { data: groups } = useGroups(meetId);
  if (!groups?.length) return null;
  return (
    <StStack gap={8}>
      <StMuted>组别</StMuted>
      <StRow wrap>
        {groups.map((g) => (
          <StCheckbox key={g.id} label={g.name} checked={value.includes(g.id)}
            onChange={(checked) => onChange(checked ? [...value, g.id] : value.filter((id) => id !== g.id))} />
        ))}
      </StRow>
    </StStack>
  );
}

/** 「调整组别」 for the selected rows of a list: picking a group applies it at once. */
export function GroupMenu({ meetId, count, onPick }: { meetId: string; count: number; onPick: (groupId: string) => void }) {
  const { data: groups } = useGroups(meetId);
  return (
    <StMenu
      trigger={(p) => <StButton disabled={count === 0 || !groups?.length} {...p}>调整组别</StButton>}
      items={[
        ...(groups ?? []).map((g) => ({ label: g.name, onSelect: () => onPick(g.id) })),
        { label: ungrouped, separated: true, onSelect: () => onPick("") },
      ]} />
  );
}
