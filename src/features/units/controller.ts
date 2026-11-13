import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Units, type UnitInput } from "../../mygo";

export function useUnits(meetId: string) {
  return useData(() => Units.list(meetId), [meetId], (c) => c.meetId === meetId, `units:${meetId}`);
}

export const UnitsController = {
  save: (meetId: string, id: string, input: UnitInput) => act(() => Units.save(meetId, id, input), "已保存"),
  remove: (meetId: string, id: string) => act(() => Units.delete(meetId, id), "已删除"),
};

