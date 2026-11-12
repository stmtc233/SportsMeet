import { expect, test } from "bun:test";
import { addHeat, deleteHeat, diff, merge, moveHeat, place, pushHistory, redo, resetAll, undo, unlock, unplace, type ArrangeDraft, type DraftHeat } from "./draft";

function heat(id: string, slots: [number, string][], keepReason = ""): DraftHeat {
  return { key: id, id, base: `base-${id}`, label: id, name: "", scheduledAt: "", keepReason, slots: slots.map(([position, athleteId]) => ({ position, athleteId })) };
}

function board(...heats: DraftHeat[]): ArrangeDraft {
  return { unlocked: [], reset: false, rounds: [{ roundId: "r", heats, deleted: [] }] };
}

const ok = (e: { draft?: ArrangeDraft; error?: string }) => {
  if (!e.draft) throw new Error(e.error);
  return e.draft;
};
const slots = (d: ArrangeDraft, i: number) => d.rounds[0]!.heats[i]!.slots.map((s) => `${s.position}${s.athleteId}`).join(" ");

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
