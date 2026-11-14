import { act } from "../../lib/action";
import { useData } from "../../lib/useData";
import { TeamScoring, type TeamScoreFilter, type TeamScoringInput, type TeamScoringRules } from "../../mygo";

/** One board per group (ungrouped events last), each ranked on its own. */
export function useTeamBoards(filter: TeamScoreFilter) {
  const key = JSON.stringify(filter);
  return useData(() => TeamScoring.boards(filter), [key], (c) => c.meetId === filter.meetId, `teamBoards:${key}`);
}

/** The meet's rules with every target they can weigh, for 赛事设置. */
export function useTeamDetail(meetId: string) {
  return useData(() => TeamScoring.get({ meetId }), [meetId], (c) => c.meetId === meetId, `teamDetail:${meetId}`);
}

/** The rules as typed: numbers stay text until Go parses them. */
export function toInput(rules: TeamScoringRules, enabled = rules.enabled): TeamScoringInput {
  return {
    enabled,
    places: rules.places.join("、"),
    tiePolicy: rules.tiePolicy,
    relayMultiplier: String(rules.relayMultiplier),
    overrides: rules.overrides.map((o) => ({ eventId: o.eventId ?? "", combinedId: o.combinedId ?? "", multiplier: String(o.multiplier) })),
  };
}

export const TeamScoringController = {
  save: (meetId: string, input: TeamScoringInput, done = "已保存") => act(() => TeamScoring.save(meetId, input), done),
};
