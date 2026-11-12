import { act } from "../../lib/action";
import { Exchange, Meets, type MeetInput } from "../../mygo";

export const MeetsController = {
  save: (id: string, input: MeetInput) => act(async () => (id ? (await Meets.update(id, input), id) : (await Meets.create(input))!.id), "已保存"),
  remove: (id: string) => act(() => Meets.delete(id), "已删除"),
  count: (id: string) => act(() => Meets.count(id)),
  pickFile: () => act(() => Exchange.pick()),
};
