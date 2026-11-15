import { StPage, StToolbar } from "../../design";
import { ExchangeSection } from "../exchange/ExchangeSection";
import { TeamScoringSection } from "../team-scoring/TeamScoringSection";
import { MeetSection } from "./MeetSection";
import { MeetSettingsSection } from "./MeetSettingsSection";
import { setupScrollKey } from "./open";

/** 赛事设置: what the meet is and the rules it runs by, set before entries come in. */
export function SetupPage({ meetId }: { meetId: string }) {
  return (
    <StPage scrollKey={setupScrollKey(meetId)} toolbar={<StToolbar title="赛事设置" />}>
      <div className="narrow">
        <MeetSection meetId={meetId} />
        <TeamScoringSection meetId={meetId} />
        <MeetSettingsSection meetId={meetId} />
        <ExchangeSection meetId={meetId} />
      </div>
    </StPage>
  );
}
