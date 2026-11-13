import type { RoundInput, Stage } from "../../mygo";
import { stageLabels } from "../../lib/labels";

/** The usual formats, as the rounds held before the final. */
const usual: Stage[][] = [[], ["heat"], ["heat", "semifinal"], ["heat", "quarterfinal", "semifinal"]];

const keyOf = (stages: string[]) => stages.join("+");
const labelOf = (stages: string[]) => (stages.length ? [...stages, "final"].map((s) => stageLabels[s as Stage] ?? s).join(" + ") : "直接决赛");

/** The value of the 赛制 select for these rounds. */
export const formatKey = (rounds: RoundInput[]) => keyOf(rounds.map((r) => r.stage));

/** The choices of the 赛制 select: the usual formats, and the event's own when it is another. */
export function formatOptions(rounds: RoundInput[]) {
  const own = rounds.map((r) => r.stage);
  const all: string[][] = usual.some((u) => keyOf(u) === keyOf(own)) ? usual : [...usual, own];
  return all.map((stages) => ({ value: keyOf(stages), label: labelOf(stages) }));
}

/** The rounds of the chosen format; a round that stays keeps its numbers. */
export function withFormat(rounds: RoundInput[], key: string): RoundInput[] {
  return (key ? key.split("+") : []).map((stage) => rounds.find((r) => r.stage === stage) ?? { stage, advanceByPlace: 2, advanceByMark: 2 });
}

/** Replaces one round's numbers. */
export function withRound(rounds: RoundInput[], stage: string, patch: Partial<RoundInput>): RoundInput[] {
  return rounds.map((r) => (r.stage === stage ? { ...r, ...patch } : r));
}
