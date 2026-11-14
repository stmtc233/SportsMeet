import { toast } from "../../design";
import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Entries, type EntryForm } from "../../mygo";

/** Each unit's and group's athletes, entries and teams; any change in the meet reads it again. */
export function useEntrySummary(meetId: string) {
  return useData(() => Entries.summary(meetId), [meetId], (c) => c.meetId === meetId, `entries:${meetId}`);
}

export const EntriesController = {
  /** 报名登记表 into a folder the user picks: one per unit, or one per group ("" = no group); all when ids is empty. */
  exportForms: async (meetId: string, ids: string[], byGroup: boolean) => {
    const count = await act(() => Entries.exportForms(meetId, ids, byGroup));
    if (typeof count === "number" && count > 0) toast(`已导出 ${count} 份`, "success");
  },
  /** Filled forms the user picks; [] when canceled. */
  pickForms: async (): Promise<EntryForm[]> => {
    const forms = await act(() => Entries.pickForms());
    return Array.isArray(forms) ? forms : [];
  },
  previewForms: (meetId: string, forms: EntryForm[]) => act(() => Entries.previewForms(meetId, forms)),
  importForms: (meetId: string, forms: EntryForm[]) => act(() => Entries.importForms(meetId, forms), "已导入"),
};
