import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Arrange, type HeatEdit } from "../../mygo";

/** The meet's timetable; any change in the meet (its minimum rest included) moves it. */
export const useTimetable = (meetId: string) =>
  useData(() => Arrange.meetSchedule(meetId), [meetId], (c) => c.meetId === meetId, `timetable:${meetId}`);

export const ScheduleController = {
  editHeat: (input: HeatEdit) => act(() => Arrange.editHeat(input), "已保存"),
};
