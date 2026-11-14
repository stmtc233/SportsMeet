import { useEffect, useState } from "react";
import { StButton, StDialog, StSegmented, StSwitchItem, StTextField } from "../../design";
import { useMeetSettings } from "../meets/data";
import { ArrangeController } from "./controller";
import type { EventDetail, PlannedHeat, RoundView } from "../../mygo";

/** The view-state key another page sets to a round id to open 「自动编排」 on it. */
export const autoArrangeKey = (eventId: string) => `arrange:auto:${eventId}`;

/** Options of 「自动编排」; the plan is computed by Go and appended to the draft. */
export function AutoDialog({ open, detail, round, exclude, onClose, onPlanned }: {
  open: boolean;
  detail: EventDetail;
  round: RoundView;
  exclude: string[];
  onClose: () => void;
  onPlanned: (heats: PlannedHeat[]) => void;
}) {
  const { data: preferences } = useMeetSettings(detail.event.meetId);
  const laned = detail.rules.usesLanes;
  const prev = detail.rounds.find((r) => r.ordinal === round.ordinal - 1);
  const canAdvance = Boolean(prev?.advance);
  const [perHeat, setPerHeat] = useState("8");
  const [order, setOrder] = useState("seed");
  const [lanes, setLanes] = useState("center");
  const [separate, setSeparate] = useState(true);
  const [balance, setBalance] = useState(true);
  const [source, setSource] = useState("entries");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!open) return;
    setPerHeat(String(detail.rules.defaultPerHeat));
    setLanes(laned ? "center" : "sequential");
    setSeparate(preferences?.separateUnits ?? true);
    setBalance(preferences?.balance ?? true);
    setSource(canAdvance ? "advance" : "entries");
  }, [open, laned, detail.rules.defaultPerHeat, canAdvance, preferences]);

  const run = async () => {
    setBusy(true);
    // The seed is made here and sent along, so the same draw can be repeated.
    const seed = Math.floor(Math.random() * 2 ** 31);
    const plan = await ArrangeController.plan({ eventId: detail.event.id, roundId: round.id, exclude, fromAdvance: source === "advance",
      perHeat: Number.parseInt(perHeat, 10) || 0, order, lanes, separateUnits: separate, balance, seed });
    setBusy(false);
    if (!plan) return;
    onPlanned(plan);
    onClose();
  };
  return (
    <StDialog open={open} title="自动编排" onClose={onClose} onSubmit={run}
      actions={
        <>
          <StButton onClick={onClose}>取消</StButton>
          <StButton variant="primary" type="submit" loading={busy}>生成</StButton>
        </>
      }>
      {canAdvance && (
        <StSegmented label="来源" value={source} onChange={setSource} options={[{ value: "advance", label: `${prev!.label}晋级者` }, { value: "entries", label: "全部报名" }]} />
      )}
      <StTextField label={detail.rules.team ? "每组队数" : "每组人数"} inputMode="numeric" value={perHeat} onChange={setPerHeat} width="narrow" suffix={detail.rules.team ? "队" : "人"} />
      <StSegmented label="排列顺序" value={order} onChange={setOrder} options={[{ value: "seed", label: source === "advance" ? "按上一轮成绩" : "按报名成绩" }, { value: "random", label: "随机" }]} />
      <StSegmented label="位置分配" value={lanes} onChange={setLanes}
        options={laned ? [{ value: "center", label: "好成绩排中间道" }, { value: "sequential", label: "依次" }, { value: "random", label: "随机" }] : [{ value: "sequential", label: "依次" }, { value: "random", label: "随机" }]} />
      <div>
        <StSwitchItem title="同单位分散" subtitle="同单位尽量不同组" checked={separate} onChange={setSeparate} />
        <StSwitchItem title={detail.rules.team ? "均衡队数" : "均衡人数"} checked={balance} onChange={setBalance} />
      </div>
    </StDialog>
  );
}
