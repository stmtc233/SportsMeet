import { useState } from "react";
import { IconImport, StButton, StListItem, StNavGroup, StNotice, StTag, iconStroke } from "../../../design";
import { recall, remember, useViewState } from "../../../lib/viewState";
import { PrintStylesController, defaultStyleId, savedOf, usePrintStyles } from "./controller";
import { selectedStyleKey, styleJumpKey } from "./keys";
import { StyleDetail } from "./StyleDetail";

/** 打印样式: the styles on the left, the chosen one's settings on the right; a narrow window shows one of the two. */
export function PrintStylesPanel() {
  const { data: styles, error } = usePrintStyles();
  const [picked, setPicked] = useViewState(selectedStyleKey, defaultStyleId);
  // Opened for one style: its page shows first in a narrow window.
  const [detail, setDetail] = useState(() => {
    const jump = recall<boolean>(styleJumpKey) === true;
    remember(styleJumpKey, undefined);
    return jump;
  });
  // What the last copy, import or rename said, for the style it belongs to.
  const [notes, setNotes] = useState<{ id: string; lines: string[] }>({ id: "", lines: [] });
  // A style deleted meanwhile (by AI, or in another window) reads as the default.
  const selected = styles && !styles.some((s) => s.id === picked) ? defaultStyleId : picked;
  const select = (id: string, lines: string[] = []) => {
    setPicked(id);
    setNotes({ id, lines });
    setDetail(true);
  };
  const importFile = async () => {
    const out = await PrintStylesController.import();
    const saved = savedOf(out === "canceled" ? undefined : out);
    if (saved?.style) select(saved.style.id, saved.warnings);
  };
  return (
    <div className="print-styles" data-detail={detail || undefined}>
      <div className="print-styles__list">
        <StButton icon={<IconImport size={16} strokeWidth={iconStroke} />} onClick={() => void importFile()}>导入</StButton>
        {error && <StNotice tone="danger">{error}</StNotice>}
        <nav className="print-styles__nav" aria-label="打印样式">
          <StNavGroup>
            {styles?.map((s) => (
              <StListItem key={s.id} title={s.name} subtitle={s.usedBy > 0 ? `${s.usedBy} 个赛事使用` : "未使用"} selected={s.id === selected}
                trailing={s.builtin ? <StTag>内置</StTag> : undefined} onClick={() => select(s.id)} />
            ))}
          </StNavGroup>
        </nav>
      </div>
      <div className="print-styles__detail">
        {notes.id === selected && notes.lines.map((line) => <StNotice key={line} tone="warning">{line}</StNotice>)}
        <StyleDetail id={selected} onBack={() => setDetail(false)} onSelect={select} onNotes={(lines) => setNotes({ id: selected, lines })} />
      </div>
    </div>
  );
}
