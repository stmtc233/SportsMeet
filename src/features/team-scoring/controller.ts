import { TeamScoring, type TeamScoreFilter, type TeamScoringInput, type TeamScoringRequest } from "../../mygo";
import { useData } from "../../lib/useData";
import { act } from "../../lib/action";

export function useTeamScores(filter: TeamScoreFilter) {
  const key = JSON.stringify(filter);
  return useData(() => TeamScoring.get(filter), [key], (c) => c.meetId === filter.meetId, `teamScores:${key}`);
}

export const TeamScoringController = {
  preview: (meetId: string, input: TeamScoringInput) => act(() => TeamScoring.preview(meetId, input)),
  apply: (request: TeamScoringRequest, hash: string) => act(() => TeamScoring.apply(request, hash, true), "已保存"),
};
