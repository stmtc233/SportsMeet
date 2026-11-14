import { StNotice, StSection, StSelect, StSwitchItem } from "../../design";
import { useOptimisticMeetSettings } from "../meets/data";
import { minuteOptions } from "../settings/minutes";

/** The minimum rest choices; 0 checks overlaps only. */
const rests = [0, 5, 10, 15, 20, 30, 45, 60];
/** The check-in lead choices; 0 prints no check-in time. */
const leads = [0, 10, 15, 20, 30, 45, 60, 90, 120, 180, 240];

/** Where the 检录单 and the 成绩记录表 start a new page. */
const sheetBreaks = [{ value: "none", label: "不分页" }, { value: "round", label: "每个轮次一页" }, { value: "heat", label: "每组一页" }];

/** The meet's own arrange and print settings; each takes effect at once. */
export function MeetSettingsSection({ meetId }: { meetId: string }) {
  const { data, error, save } = useOptimisticMeetSettings(meetId);
  if (error) return <StNotice tone="danger">{error}</StNotice>;
  if (!data) return null;
  return (
    <>
      <StSection title="编排" padded={false}>
        <StSwitchItem title="同单位分散" subtitle="自动编排时同单位尽量不同组" checked={data.separateUnits} onChange={(separateUnits) => save({ separateUnits })} />
        <StSwitchItem title="均衡人数" subtitle="自动编排的默认选项" checked={data.balance} onChange={(balance) => save({ balance })} />
        <div className="setup-fields">
          <StSelect label="最短休息时间" value={String(data.minRestMinutes)} onChange={(v) => save({ minRestMinutes: Number(v) })}
            options={minuteOptions(rests, data.minRestMinutes, "只查重叠")} />
          <StSelect label="检录提前时间" value={String(data.checkInLeadMinutes)} onChange={(v) => save({ checkInLeadMinutes: Number(v) })}
            options={minuteOptions(leads, data.checkInLeadMinutes, "不显示")} />
        </div>
      </StSection>
      <StSection title="打印" padded={false}>
        <StSwitchItem title="代表队名单" subtitle="印在秩序册的项目之前" checked={data.printRoster} onChange={(printRoster) => save({ printRoster })} />
        <StSwitchItem title="竞赛日程表" subtitle="印在秩序册的项目之前" checked={data.printSchedule} onChange={(printSchedule) => save({ printSchedule })} />
        <StSwitchItem title="团体总分表" subtitle="印在成绩册的项目之前" checked={data.printTeamScores} onChange={(printTeamScores) => save({ printTeamScores })} />
        <StSwitchItem title="成绩时间" subtitle="成绩册每组后印最后修改时间" checked={data.printResultsTime} onChange={(printResultsTime) => save({ printResultsTime })} />
        <StSwitchItem title="每个项目另起一页" checked={data.printNewPage} onChange={(printNewPage) => save({ printNewPage })} />
        <div className="setup-fields">
          <StSelect label="检录单分页" value={data.printCheckInBreak} onChange={(printCheckInBreak) => save({ printCheckInBreak })} options={sheetBreaks} />
          <StSelect label="成绩记录表分页" value={data.printRecordBreak} onChange={(printRecordBreak) => save({ printRecordBreak })} options={sheetBreaks} />
        </div>
      </StSection>
    </>
  );
}
