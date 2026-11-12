import { Arrange, type BoardSave, type PlanRequest } from "../../mygo";
import { act } from "../../lib/action";

export const ArrangeController = {
  save: (input: BoardSave) => Arrange.save(input),
  plan: (input: PlanRequest) => act(() => Arrange.plan(input)),
};
