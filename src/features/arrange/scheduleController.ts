import { act } from "../../lib/action";
import { Arrange, type ScheduleRequest } from "../../mygo";

export const ScheduleController = {
  preview: (request: ScheduleRequest) => act(() => Arrange.previewSchedule(request)),
  save: (request: ScheduleRequest, hash: string) => act(() => Arrange.schedule(request, hash), "已保存"),
};
