import { useState } from "react";
import { IconBack, StButton, StIconButton, StMuted, StNotice, StRow, StSection, StTag, StTextField, iconStroke, toast } from "../../../design";
import { useOptimistic } from "../../../lib/optimistic";
import type { PrintStyleDetail } from "../../../mygo";
import { DeleteDialog } from "../../meets/DeleteDialog";
import { AssetsSection } from "./AssetsSection";
import { PrintStylesController, defaultStyleId, savedOf, usePrintStyle } from "./controller";
import { StyleParamsForm } from "./StyleParamsForm";
import { VersionsDialog } from "./VersionsDialog";

/** One style: its name, copy / export / delete, versions, parameters and images. */
export function StyleDetail({ id, onBack, onSelect, onNotes }: {
  id: string;
  onBack: () => void;
  /** Another style takes over the page (a copy, or the default after a delete). */
  onSelect: (id: string, notes?: string[]) => void;
  onNotes: (notes: string[]) => void;
}) {
  const { data: style, error } = usePrintStyle(id);
  return (
    <>
      <div className="print-styles__back"><StIconButton label="样式列表" onClick={onBack}><IconBack size={18} strokeWidth={iconStroke} /></StIconButton></div>
      {error && <StNotice tone="danger">{error}</StNotice>}
      {style?.id === id && <Loaded key={id} style={style} onSelect={onSelect} onNotes={onNotes} />}
    </>
  );
}

function Loaded({ style, onSelect, onNotes }: { style: PrintStyleDetail; onSelect: (id: string, notes?: string[]) => void; onNotes: (notes: string[]) => void }) {
  const { id, builtin } = style;
  const [typed, setTyped] = useState<string | null>(null);
  const { data: named, save: saveName } = useOptimistic({ name: style.name }, async ({ name }) => {
    const out = await PrintStylesController.rename(id, name);
    const saved = savedOf(out);
    if (saved) onNotes(saved.warnings);
    return out;
  });
  const [versions, setVersions] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [busy, setBusy] = useState(false);
  const commitName = () => {
    const name = (typed ?? "").trim();
    setTyped(null);
    if (name && name !== named?.name) saveName({ name });
  };
  const copy = async () => {
    setBusy(true);
    const saved = savedOf(await PrintStylesController.copy(id));
    setBusy(false);
    if (saved?.style) onSelect(saved.style.id, saved.warnings);
  };
  const exportFile = async () => {
    setBusy(true);
    const out = await PrintStylesController.export(id);
    setBusy(false);
    if (out === "saved") toast("已导出", "success");
  };
  const used = style.usedBy;
  return (
    <>
      <StSection title="样式" padded={false}>
        <div className="style-name">
          <StTextField label="名称" width="fill" disabled={builtin} value={typed ?? named?.name ?? style.name} onChange={setTyped}
            onBlur={commitName}
            onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); e.currentTarget.blur(); } }} />
        </div>
        <div className="style-facts">
          {builtin && <StMuted>只读，可复制后修改</StMuted>}
          {style.hasCustomCss && <StTag>含自定义 CSS</StTag>}
          {style.hasTemplates && <StTag>含自定义模板</StTag>}
          <StRow>
            <StButton disabled={busy} onClick={() => void copy()}>复制</StButton>
            <StButton disabled={busy} onClick={() => void exportFile()}>导出</StButton>
            <StButton disabled={builtin} onClick={() => setVersions(true)}>历史版本</StButton>
            <StButton variant="danger" disabled={builtin} onClick={() => setDeleting(true)}>删除</StButton>
          </StRow>
        </div>
      </StSection>
      <StyleParamsForm id={id} params={style.params} readOnly={builtin} />
      {!builtin && <AssetsSection scope="style" ownerId={id} title="图片" />}
      <VersionsDialog id={id} open={versions} onClose={() => setVersions(false)} />
      <DeleteDialog open={deleting} title={`删除「${style.name}」？`} onClose={() => setDeleting(false)}
        onDelete={async () => {
          const ok = await PrintStylesController.remove(id);
          if (ok) onSelect(defaultStyleId);
          return ok;
        }}>
        {used > 0 ? `${used} 个赛事正在使用，删除后改用「默认」。` : "没有赛事使用它。"}
      </DeleteDialog>
    </>
  );
}
