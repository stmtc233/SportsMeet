import { expect, test } from "bun:test";
import { addHeat, deleteHeat, diff, draftTimes, editHeat, merge, moveHeat, place, pushHistory, redo, resetAll, undo, unlock, unplace, type ArrangeDraft, type DraftHeat } from "./draft";

function heat(id: string, slots: [number, string][], keepReason = ""): DraftHeat {
  return { key: id, id, base: `base-${id}`, label: id, name: "", scheduledAt: "", durationMinutes: null, scheduledText: "", scheduleInherited: false,
    durationText: "", keepReason, slots: slots.map(([position, competitorId]) => ({ position, competitorId })) };
}

function board(...heats: DraftHeat[]): ArrangeDraft {
  return { unlocked: [], reset: false, rounds: [{ roundId: "r", startAt: "", startText: "", heats, deleted: [] }] };
}

/** The same board with the round starting at `at`, shown as `text`; heats without their own time follow it. */
function started(d: ArrangeDraft, at: string, text: string): ArrangeDraft {
  return { ...d, rounds: d.rounds.map((r) => ({ ...r, startAt: at, startText: text,
    heats: r.heats.map((h) => (h.scheduledAt ? h : { ...h, scheduledText: text, scheduleInherited: at !== "" })) })) };
}

const ok = (e: { draft?: ArrangeDraft; error?: string }) => {
  if (!e.draft) throw new Error(e.error);
  return e.draft;
};
const slots = (d: ArrangeDraft, i: number) => d.rounds[0]!.heats[i]!.slots.map((s) => `${s.position}${s.competitorId}`).join(" ");

test("place moves, swaps and sends the occupant back to the list", () => {
  const loaded = board(heat("h1", [[1, "a"], [2, "b"]]), heat("h2", [[1, "c"]]));
  let d = ok(place(loaded, "r", "h2", 3, "a", true));
  expect(slots(d, 0)).toBe("2b");
  expect(slots(d, 1)).toBe("1c 3a");
  d = ok(place(d, "r", "h1", 2, "c", true)); // c and b swap
  expect(slots(d, 0)).toBe("2c");
  expect(slots(d, 1)).toBe("1b 3a");
  d = ok(place(d, "r", "h1", 2, "x", true)); // from the list: c goes back
  expect(slots(d, 0)).toBe("2x");
  d = ok(unplace(d, "r", "a", false)); // an order has no gaps
  expect(slots(d, 1)).toBe("1b");
  expect(loaded.rounds[0]!.heats[0]!.slots.length).toBe(2); // the loaded draft is untouched
});

test("a heat with results is locked until unlocked", () => {
  const loaded = board(heat("h1", [[1, "a"]], "已有成绩"), heat("h2", []));
  expect(place(loaded, "r", "h2", 1, "a", true).error).toContain("先解锁");
  expect(place(loaded, "r", "h1", 2, "x", true).error).toContain("先解锁");
  expect(deleteHeat(loaded, "r", "h1").error).toContain("不能删除");
  const d = ok(place(unlock(loaded, "h1"), "r", "h2", 1, "a", true));
  expect(slots(d, 1)).toBe("1a");
  const reset = resetAll(loaded);
  expect(reset.rounds[0]!.deleted.map((x) => x.id)).toEqual(["h1", "h2"]);
  expect(diff(reset, loaded, "e").save.reset).toBe(true);
});

test("diff submits only what changed", () => {
  const loaded = board(heat("h1", [[1, "a"]]), heat("h2", [[1, "b"]]), heat("h3", []));
  expect(diff(loaded, loaded, "e").count).toBe(0);
  let d = ok(place(loaded, "r", "h1", 2, "x", true));
  d = addHeat(d, "r");
  d = ok(deleteHeat(d, "r", "h3"));
  const { save, count } = diff(d, loaded, "e");
  const r = save.rounds[0]!;
  expect(count).toBe(3);
  expect(r.changed.map((h) => h.id)).toEqual(["h1"]);
  expect(r.changed[0]!.base).toBe("base-h1");
  expect(r.added.length).toBe(1);
  expect(r.deleted).toEqual([{ id: "h3", base: "base-h3" }]);
  expect(r.order).toEqual(["h1", "h2", r.added[0]!.key]);
  const moved = diff(moveHeat(loaded, "r", "h1", 1), loaded, "e");
  expect(moved.count).toBe(1);
  expect(moved.save.rounds[0]!.order).toEqual(["h2", "h1", "h3"]);
  expect(moved.save.rounds[0]!.changed).toEqual([]);
});

test("merge keeps local changes to heats untouched elsewhere", () => {
  const loaded = board(heat("h1", [[1, "a"]]), heat("h2", [[1, "b"]]));
  let d = ok(place(loaded, "r", "h1", 2, "x", true)); // local change to h1
  d = ok(place(d, "r", "h2", 2, "y", true)); // local change to h2
  d = addHeat(d, "r");
  const fresh = board(heat("h1", [[1, "a"]]), { ...heat("h2", [[1, "b"], [3, "x"]]), base: "other" }, heat("h9", [[1, "z"]]));
  const out = merge(d, loaded, fresh);
  expect(out.dropped).toBe(true); // h2 changed elsewhere: the local y is gone
  const heats = out.draft.rounds[0]!.heats;
  expect(heats.map((h) => h.key).slice(0, 3)).toEqual(["h1", "h2", "h9"]);
  expect(heats.length).toBe(4); // the new local heat stays
  expect(slots(out.draft, 1)).toBe("1b 3x");
  expect(slots(out.draft, 0)).toBe("1a"); // x is now in h2 elsewhere, so it leaves the local h1
  const same = merge(d, loaded, loaded);
  expect(same.dropped).toBe(false);
  expect(slots(same.draft, 0)).toBe("1a 2x");
});

test("merge after a save leaves nothing to submit", () => {
  const loaded = board(heat("h1", [[1, "a"]]));
  const d = ok(place(loaded, "r", "h1", 2, "x", true));
  const fresh = board({ ...heat("h1", [[1, "a"], [2, "x"]]), base: "saved" });
  const out = merge(d, loaded, fresh);
  expect(out.dropped).toBe(false);
  expect(diff(out.draft, fresh, "e").count).toBe(0);
});

test("history undoes and redoes", () => {
  const a = board(heat("h1", []));
  const b = addHeat(a, "r");
  let h = pushHistory({ past: [], present: a, future: [] }, b);
  h = undo(h);
  expect(h.present).toBe(a);
  h = redo(h);
  expect(h.present).toBe(b);
  expect(undo(undo(h)).present).toBe(a);
});

test("a heat's own time and duration are submitted; a new heat follows the round's start", () => {
  const loaded = started(board(heat("h1", [[1, "a"]]), heat("h2", [])), "2026-11-12T09:00", "09:00");
  expect(loaded.rounds[0]!.heats[0]!.scheduleInherited).toBe(true);
  const patch = { name: "", scheduledAt: "", durationMinutes: 5, scheduledText: "09:00", scheduleInherited: true, durationText: "5 分钟" };
  let d = editHeat(loaded, "r", "h1", patch);
  d = addHeat(d, "r");
  const added = d.rounds[0]!.heats[2]!;
  expect([added.scheduledText, added.scheduleInherited, added.durationMinutes]).toEqual(["09:00", true, null]);
  const { save, count } = diff(d, loaded, "e");
  expect(count).toBe(2);
  expect(save.rounds[0]!.changed).toEqual([expect.objectContaining({ id: "h1", scheduledAt: "", durationMinutes: 5 })]);
  expect(save.rounds[0]!.added[0]!.durationMinutes).toBeNull();
});

test("a round start changed elsewhere shows on local heats without their own time", () => {
  const loaded = started(board(heat("h1", [[1, "a"]]), heat("h2", [[1, "b"]])), "2026-11-12T09:00", "09:00");
  let d = ok(place(loaded, "r", "h1", 2, "x", true)); // local change, no own time
  d = editHeat(d, "r", "h2", { name: "", scheduledAt: "2026-11-12T10:00", durationMinutes: null, scheduledText: "10:00", scheduleInherited: false, durationText: "" });
  d = addHeat(d, "r");
  // The fingerprints stay: the round's start is not part of a heat.
  const fresh = started(loaded, "2026-11-12T14:00", "14:00");
  const out = merge(d, loaded, fresh);
  expect(out.dropped).toBe(false);
  const heats = out.draft.rounds[0]!.heats;
  expect(out.draft.rounds[0]!.startText).toBe("14:00");
  expect(heats.map((h) => [h.scheduledText, h.scheduleInherited])).toEqual([["14:00", true], ["10:00", false], ["14:00", true]]);
  expect(slots(out.draft, 0)).toBe("1a 2x");
  expect(diff(out.draft, fresh, "e").count).toBe(3);
});

test("draftTimes sends every draft heat with its key, time and competitors, not deleted ones", () => {
  const d = ok(deleteHeat(addHeat(board(heat("h1", [[2, "a"], [1, "b"]]), heat("h2", [])), "r"), "r", "h2"));
  const withTime = ok({ draft: editHeat(d, "r", "h1", { name: "甲", scheduledAt: "2026-11-12T09:00", durationMinutes: 5, scheduledText: "09:00", scheduleInherited: false, durationText: "5 分钟" }) });
  const req = draftTimes("e", withTime.rounds);
  expect(req.eventId).toBe("e");
  expect(req.heats.map((h) => h.key)).toEqual(["h1", withTime.rounds[0]!.heats[1]!.key]);
  expect(req.heats[0]).toEqual({ key: "h1", roundId: "r", name: "甲", scheduledAt: "2026-11-12T09:00", durationMinutes: 5, competitors: ["a", "b"] });
});
