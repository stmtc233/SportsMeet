import { useState } from "react";
import { IconAdd, IconDelete, StButton, StIconButton, StListItem, StMuted, StNotice, StSection, iconStroke, toast } from "../../../design";
import type { AssetInfo } from "../../../mygo";
import { DeleteDialog } from "../../meets/DeleteDialog";
import { sizeText } from "./assets";
import { PrintStylesController, useAssetImage, useAssets, type AssetScope } from "./controller";

/**
 * The images of a style or a meet: thumbnail, name, upload and delete. The
 * name is what text refers to as asset:名称.
 */
export function AssetsSection({ scope, ownerId, title }: { scope: AssetScope; ownerId: string; title: string }) {
  const { data, error } = useAssets(scope, ownerId);
  const [busy, setBusy] = useState(false);
  const [removing, setRemoving] = useState<AssetInfo | null>(null);
  const upload = async () => {
    setBusy(true);
    const out = await PrintStylesController.upload(scope, ownerId);
    setBusy(false);
    if (out && out !== "canceled") toast("已上传", "success");
  };
  return (
    <StSection title={title} padded={false} actions={<StButton icon={<IconAdd size={16} strokeWidth={iconStroke} />} loading={busy} onClick={upload}>上传</StButton>}>
      {error && <StNotice tone="danger">{error}</StNotice>}
      {data?.map((a) => (
        <StListItem key={a.name} leading={<Thumb scope={scope} ownerId={ownerId} asset={a} />} title={a.name} subtitle={`asset:${a.name} · ${sizeText(a.size)}`}
          trailing={
            <StIconButton label={`删除图片 ${a.name}`} onClick={() => setRemoving(a)}>
              <IconDelete size={18} strokeWidth={iconStroke} />
            </StIconButton>
          } />
      ))}
      <div className="asset-hint"><StMuted>在文字里写 asset:名称 引用图片</StMuted></div>
      <DeleteDialog open={removing !== null} title={`删除图片「${removing?.name ?? ""}」？`} onClose={() => setRemoving(null)}
        onDelete={() => PrintStylesController.deleteAsset(scope, ownerId, removing!.name)}>
        引用它的内容将不再显示图片。
      </DeleteDialog>
    </StSection>
  );
}

function Thumb({ scope, ownerId, asset }: { scope: AssetScope; ownerId: string; asset: AssetInfo }) {
  const { data } = useAssetImage(scope, ownerId, asset.name, asset.sha256);
  return data ? <img className="asset-thumb" src={data} alt="" /> : <span className="asset-thumb" aria-hidden />;
}
