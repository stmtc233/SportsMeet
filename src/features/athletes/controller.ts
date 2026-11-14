import { toast } from "../../design";
import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Athletes, Entries, type AthleteImport, type AthleteInput } from "../../mygo";

export function useAthletes(meetId: string) {
  return useData(() => Athletes.list(meetId), [meetId], (c) => c.meetId === meetId, `athletes:${meetId}`);
}

/** The individual events the athlete dialog offers for this gender and group; entries change them. */
export function useAthleteEntryOptions(meetId: string, athleteId: string, gender: string, groupId: string, open: boolean) {
  return useData(() => (open ? Entries.athleteOptions(meetId, athleteId, gender, groupId) : Promise.resolve(undefined)), [meetId, athleteId, gender, groupId, open],
    (c) => open && c.meetId === meetId && (c.kind === "entries" || c.kind === "events" || c.kind === "heats"));
}

export const AthletesController = {
  setEntries: (meetId: string, athleteId: string, eventIds: string[]) => act(() => Entries.setAthlete(meetId, athleteId, eventIds)),
  save: async (meetId: string, id: string, input: AthleteInput) => {
    const saved = await act(() => Athletes.save(meetId, id, input), "已保存");
    for (const warning of saved?.warnings ?? []) toast(warning, "warning");
    return saved;
  },
  remove: (meetId: string, ids: string[]) => act(() => Athletes.delete(meetId, ids), "已删除"),
  pickTable: () => act(() => Athletes.pickTable()),
  parse: (text: string) => act(() => Athletes.parseTable(text)),
  preview: (meetId: string, input: AthleteImport) => act(() => Athletes.previewImport(meetId, input)),
  import: (meetId: string, input: AthleteImport) => act(() => Athletes.import(meetId, input), "已导入"),
};
