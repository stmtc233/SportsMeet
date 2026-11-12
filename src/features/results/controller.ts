import { act } from "../../lib/action";
import { Marks, Results, type EventKind, type HeatStartInput, type ResultInput } from "../../mygo";

export const ResultsController = {
  save: (heatId: string, inputs: ResultInput[]) => act(() => Results.save(heatId, inputs), "已保存"),
  /** Changes or clears a heat's actual start ("" clears). */
  setStarted: (input: HeatStartInput) => act(() => Results.setStarted(input), input.startedAt ? "已保存" : "已清除"),
  /** What was typed, as marks are shown; undefined when it is not a mark. */
  normalize: (kind: EventKind, input: string) => act(() => Marks.normalize(kind, input)),
};
