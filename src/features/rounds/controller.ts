import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { Arrange, type BatchArrangeRequest } from "../../mygo";

/** Every round of the meet with where it stands; any change in the meet moves it. */
export const useMeetRounds = (meetId: string) =>
  useData(() => Arrange.meetRounds(meetId), [meetId], (c) => c.meetId === meetId, `rounds:${meetId}`);

export const RoundsController = {
  preview: (request: BatchArrangeRequest) => act(() => Arrange.previewBatch(request)),
  arrange: (request: BatchArrangeRequest, hash: string) => act(() => Arrange.batch(request, hash), "已编排"),
};
