import { toast } from "../../design";
import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Athletes, type AthleteImport, type AthleteInput } from "../../mygo";

export function useAthletes(meetId: string) {
  return useData(() => Athletes.list(meetId), [meetId], (c) => c.meetId === meetId);
}

export const AthletesController = {
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
