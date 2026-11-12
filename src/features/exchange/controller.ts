import { act } from "../../lib/action";
import { Exchange } from "../../mygo";

export const ExchangeController = {
  export: (meetId: string) => act(async () => ((await Exchange.export(meetId)) ? "saved" : "canceled")),
  pick: () => act(() => Exchange.pick()),
  preview: (path: string, meetId: string) => act(() => Exchange.preview(path, meetId)),
  import: (path: string, meetId: string, hash: string) => act(() => Exchange.import(path, meetId, hash), "已导入"),
};
