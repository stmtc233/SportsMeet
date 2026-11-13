import { useState } from "react";
import { StNotice, StSection, StSegmented, StSelect, StSwitchItem } from "../../design";
import { SettingsController, usePreferences } from "./controller";
import { minuteOptions } from "./minutes";

/** The minimum rest choices; 0 checks overlaps only. */
const rests = [0, 5, 10, 15, 20, 30, 45, 60];
/** The check-in lead choices; 0 prints no check-in time. */
const leads = [0, 10, 15, 20, 30, 45, 60, 90, 120, 180, 240];

/** Where the 检录单 and the 成绩记录表 start a new page. */
const sheetBreaks = [{ value: "none", label: "不分页" }, { value: "round", label: "每个轮次一页" }, { value: "heat", label: "每组一页" }];

export function PreferencesPanel({ section }: { section: "general" | "arrange" | "print" }) {
  const { data, error } = usePreferences();
  const [busy, setBusy] = useState(false);
  if (error) return <StNotice tone="danger">{error}</StNotice>;
  if (!data) return null;
  const save = async (patch: Partial<typeof data>) => {
    if (busy) return;
    setBusy(true);
    await SettingsController.save({ ...data, ...patch });
    setBusy(false);
  };
  if (section === "general") return (
    <StSection title="外观">
      <StSegmented value={data.theme} onChange={(theme) => void save({ theme })}
        options={[{ value: "system", label: "跟随系统", disabled: busy }, { value: "light", label: "浅色", disabled: busy }, { value: "dark", label: "深色", disabled: busy }]} />
    </StSection>
  );
  if (section === "arrange") return (
    <>
      <StSection title="自动编排默认值" padded={false}>
        <StSwitchItem title="同单位分散" subtitle="同单位尽量不同组" checked={data.separateUnits} disabled={busy} onChange={(separateUnits) => void save({ separateUnits })} />
        <StSwitchItem title="均衡人数" checked={data.balance} disabled={busy} onChange={(balance) => void save({ balance })} />
      </StSection>
      <StSection title="时间冲突">
        <StSelect label="最短休息时间" value={String(data.minRestMinutes)} disabled={busy} onChange={(v) => void save({ minRestMinutes: Number(v) })}
          options={minuteOptions(rests, data.minRestMinutes, "只查重叠")} />
      </StSection>
      <StSection title="检录">
        <StSelect label="检录提前时间" value={String(data.checkInLeadMinutes)} disabled={busy} onChange={(v) => void save({ checkInLeadMinutes: Number(v) })}
          options={minuteOptions(leads, data.checkInLeadMinutes, "不显示")} />
      </StSection>
    </>
  );
  return (
    <>
      <StSection title="秩序册" padded={false}>
        <StSwitchItem title="竞赛日程表" subtitle="印在项目之前" checked={data.printSchedule} disabled={busy} onChange={(printSchedule) => void save({ printSchedule })} />
      </StSection>
      <StSection title="成绩册" padded={false}>
        <StSwitchItem title="成绩时间" subtitle="每组成绩后印最后修改时间" checked={data.printResultsTime} disabled={busy} onChange={(printResultsTime) => void save({ printResultsTime })} />
      </StSection>
      <StSection title="检录单">
        <StSelect label="分页" value={data.printCheckInBreak} disabled={busy} onChange={(printCheckInBreak) => void save({ printCheckInBreak })} options={sheetBreaks} />
      </StSection>
      <StSection title="成绩记录表">
        <StSelect label="分页" value={data.printRecordBreak} disabled={busy} onChange={(printRecordBreak) => void save({ printRecordBreak })} options={sheetBreaks} />
      </StSection>
      <StSection title="分页" padded={false}>
        <StSwitchItem title="每个项目另起一页" checked={data.printNewPage} disabled={busy} onChange={(printNewPage) => void save({ printNewPage })} />
      </StSection>
    </>
  );
}
