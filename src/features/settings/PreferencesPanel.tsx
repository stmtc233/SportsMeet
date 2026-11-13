import { useEffect, useRef, useState } from "react";
import type { Preferences } from "../../mygo";
import { StNotice, StSection, StSegmented, StSelect, StSwitchItem } from "../../design";
import { SettingsController, usePreferences } from "./controller";
import { minuteOptions } from "./minutes";

/** The minimum rest choices; 0 checks overlaps only. */
const rests = [0, 5, 10, 15, 20, 30, 45, 60];
/** The check-in lead choices; 0 prints no check-in time. */
const leads = [0, 10, 15, 20, 30, 45, 60, 90, 120, 180, 240];

/** Where the 检录单 and the 成绩记录表 start a new page. */
const sheetBreaks = [{ value: "none", label: "不分页" }, { value: "round", label: "每个轮次一页" }, { value: "heat", label: "每组一页" }];

/**
 * Each control takes effect at once: it shows the new value straight away
 * and the saves go out one after another, each with every change so far.
 * A value stays shown as chosen until the reloaded settings carry it, and
 * goes back if its save fails. Controls are not disabled while saving: the
 * whole panel would blink on every click.
 */
function useOptimisticPreferences() {
  const { data, error } = usePreferences();
  const [pending, setPending] = useState<Partial<Preferences>>({});
  const latest = useRef({ data, pending });
  latest.current = { data, pending };
  const queue = useRef(Promise.resolve());
  useEffect(() => {
    if (!data) return;
    setPending((old) => {
      const left = Object.fromEntries(Object.entries(old).filter(([k, v]) => data[k as keyof Preferences] !== v));
      return Object.keys(left).length === Object.keys(old).length ? old : left;
    });
  }, [data]);
  const save = (patch: Partial<Preferences>) => {
    latest.current.pending = { ...latest.current.pending, ...patch };
    setPending((old) => ({ ...old, ...patch }));
    queue.current = queue.current.then(async () => {
      const { data: saved, pending: changes } = latest.current;
      if (!saved) return;
      const ok = await SettingsController.save({ ...saved, ...changes });
      if (!ok) setPending((old) => Object.fromEntries(Object.entries(old).filter(([k]) => !(k in patch))));
    });
  };
  return { data: data && { ...data, ...pending }, error, save };
}

export function PreferencesPanel({ section }: { section: "general" | "arrange" | "print" }) {
  const { data, error, save } = useOptimisticPreferences();
  if (error) return <StNotice tone="danger">{error}</StNotice>;
  if (!data) return null;
  if (section === "general") return (
    <StSection title="外观">
      <StSegmented value={data.theme} onChange={(theme) => save({ theme })}
        options={[{ value: "system", label: "跟随系统" }, { value: "light", label: "浅色" }, { value: "dark", label: "深色" }]} />
    </StSection>
  );
  if (section === "arrange") return (
    <>
      <StSection title="自动编排默认值" padded={false}>
        <StSwitchItem title="同单位分散" subtitle="同单位尽量不同组" checked={data.separateUnits} onChange={(separateUnits) => save({ separateUnits })} />
        <StSwitchItem title="均衡人数" checked={data.balance} onChange={(balance) => save({ balance })} />
      </StSection>
      <StSection title="时间冲突">
        <StSelect label="最短休息时间" value={String(data.minRestMinutes)} onChange={(v) => save({ minRestMinutes: Number(v) })}
          options={minuteOptions(rests, data.minRestMinutes, "只查重叠")} />
      </StSection>
      <StSection title="检录">
        <StSelect label="检录提前时间" value={String(data.checkInLeadMinutes)} onChange={(v) => save({ checkInLeadMinutes: Number(v) })}
          options={minuteOptions(leads, data.checkInLeadMinutes, "不显示")} />
      </StSection>
    </>
  );
  return (
    <>
      <StSection title="秩序册" padded={false}>
        <StSwitchItem title="代表队名单" subtitle="印在项目之前" checked={data.printRoster} onChange={(printRoster) => save({ printRoster })} />
        <StSwitchItem title="竞赛日程表" subtitle="印在项目之前" checked={data.printSchedule} onChange={(printSchedule) => save({ printSchedule })} />
      </StSection>
      <StSection title="成绩册" padded={false}>
        <StSwitchItem title="团体总分表" subtitle="印在项目之前" checked={data.printTeamScores} onChange={(printTeamScores) => save({ printTeamScores })} />
        <StSwitchItem title="成绩时间" subtitle="每组成绩后印最后修改时间" checked={data.printResultsTime} onChange={(printResultsTime) => save({ printResultsTime })} />
      </StSection>
      <StSection title="检录单">
        <StSelect label="分页" value={data.printCheckInBreak} onChange={(printCheckInBreak) => save({ printCheckInBreak })} options={sheetBreaks} />
      </StSection>
      <StSection title="成绩记录表">
        <StSelect label="分页" value={data.printRecordBreak} onChange={(printRecordBreak) => save({ printRecordBreak })} options={sheetBreaks} />
      </StSection>
      <StSection title="分页" padded={false}>
        <StSwitchItem title="每个项目另起一页" checked={data.printNewPage} onChange={(printNewPage) => save({ printNewPage })} />
      </StSection>
    </>
  );
}
