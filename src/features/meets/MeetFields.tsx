import { StDateField, StTextField } from "../../design";
import type { EntryLimits, MeetInput } from "../../mygo";
import { limitText, limitValue } from "./limits";

const limitFields: { key: keyof EntryLimits; label: string; unit: string }[] = [
  { key: "athleteEvents", label: "每人限报项数", unit: "项" },
  { key: "unitAthletes", label: "单位每项限报人数", unit: "人" },
  { key: "unitTeams", label: "单位每项限报队数", unit: "队" },
];

type Form = Required<MeetInput>;

/** Name, dates and place of a meet. */
export function MeetInfoFields({ form, onChange, autoFocus }: { form: Form; onChange: (patch: Partial<Form>) => void; autoFocus?: boolean }) {
  return (
    <div className="form-grid">
      <div className="span">
        <StTextField label="赛事名称" value={form.name} onChange={(name) => onChange({ name })} width="fill" autoFocus={autoFocus} />
      </div>
      <StDateField label="开始日期" value={form.startDate} onChange={(startDate) => onChange({ startDate })} />
      <StDateField label="结束日期" value={form.endDate} onChange={(endDate) => onChange({ endDate })} />
      <div className="span">
        <StTextField label="地点" value={form.location} onChange={(location) => onChange({ location })} width="fill" />
      </div>
    </div>
  );
}

/** The 报名限制: blank is no limit. */
export function MeetLimitFields({ form, onChange }: { form: Form; onChange: (patch: Partial<Form>) => void }) {
  return (
    <div className="form-grid">
      {limitFields.map(({ key, label, unit }) => (
        <StTextField key={key} label={label} inputMode="numeric" placeholder="不限" suffix={unit} value={limitText(form.entryLimits[key])}
          onChange={(text) => onChange({ entryLimits: { ...form.entryLimits, [key]: limitValue(text) } })} />
      ))}
    </div>
  );
}
