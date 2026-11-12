import { act } from "../../lib/action";
import { Marks, Results, type EventKind, type ResultInput } from "../../mygo";

export const ResultsController = {
  save: (heatId: string, inputs: ResultInput[]) => act(() => Results.save(heatId, inputs), "已保存"),
  /** What was typed, as marks are shown; undefined when it is not a mark. */
  normalize: (kind: EventKind, input: string) => act(() => Marks.normalize(kind, input)),
};
