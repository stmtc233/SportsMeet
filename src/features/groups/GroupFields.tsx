import { StButton, StCheckbox, StMenu, StMuted, StRow, StSelect, StStack } from "../../design";
import { useViewState } from "../../lib/viewState";
import { useGroups } from "./controller";

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

/** The one group of an event, combined event or athlete. */
export function GroupSelect({ meetId, value, onChange, disabled }: { meetId: string; value: string; onChange: (id: string) => void; disabled?: boolean }) {
  const { data: groups } = useGroups(meetId);
  const known = value === "" || !groups || groups.some((g) => g.id === value);
  return (
    <StSelect label="组别" width="fill" value={known ? value : ""} onChange={onChange} disabled={disabled}
      options={[{ value: "", label: ungrouped }, ...(groups ?? []).map((g) => ({ value: g.id, label: g.name }))]} />
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
