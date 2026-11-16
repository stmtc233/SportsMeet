import { StPage, StToolbar } from "../../design";
import { ExchangeSection } from "../exchange/ExchangeSection";
import { TeamScoringSection } from "../team-scoring/TeamScoringSection";
import { AssetsSection } from "../settings/print-styles/AssetsSection";
import { MeetSection } from "./MeetSection";
import { FrontPagesSection } from "./FrontPagesSection";
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
        <FrontPagesSection meetId={meetId} />
        <AssetsSection scope="meet" ownerId={meetId} title="赛事图片" />
        <ExchangeSection meetId={meetId} />
      </div>
    </StPage>
  );
}
