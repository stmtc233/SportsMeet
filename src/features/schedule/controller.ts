import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Arrange, type HeatEdit } from "../../mygo";

/** The meet's timetable; any change in the meet, or the minimum rest, moves it. */
export const useTimetable = (meetId: string) =>
  useData(() => Arrange.meetSchedule(meetId), [meetId], (c) => c.meetId === meetId || c.kind === "settings");

export const ScheduleController = {
  editHeat: (input: HeatEdit) => act(() => Arrange.editHeat(input), "已保存"),
};
