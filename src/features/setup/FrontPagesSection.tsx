import { useState } from "react";
import { IconAdd, IconMore, StButton, StIconButton, StListItem, StMenu, StNotice, StSection, iconStroke, type StMenuItem } from "../../design";
import type { FrontPageView } from "../../mygo";
import { DeleteDialog } from "../meets/DeleteDialog";
import { FrontPagesController, useFrontPages } from "../settings/print-styles/controller";
import { FrontPageDialog } from "./FrontPageDialog";
import { moved, reportsText } from "./frontPages";

/** The most front pages a meet holds (Go's limit). */
const most = 20;

/** 前置页: Markdown pages printed in front of the reports, in this order. */
export function FrontPagesSection({ meetId }: { meetId: string }) {
  const { data: pages, error } = useFrontPages(meetId);
  const [editing, setEditing] = useState<{ page?: FrontPageView } | null>(null);
  const [deleting, setDeleting] = useState<FrontPageView | null>(null);
  const [moving, setMoving] = useState(false);
  const ids = (pages ?? []).map((p) => p.id);
  const move = async (index: number, by: -1 | 1) => {
    setMoving(true);
    await FrontPagesController.reorder(meetId, moved(ids, index, by));
    setMoving(false);
  };
  const menu = (page: FrontPageView, i: number): StMenuItem[] => [
    { label: "编辑", onSelect: () => setEditing({ page }) },
    { label: "上移", disabled: moving || i === 0, onSelect: () => void move(i, -1) },
    { label: "下移", disabled: moving || i === ids.length - 1, onSelect: () => void move(i, 1) },
    { label: "删除", danger: true, separated: true, onSelect: () => setDeleting(page) },
  ];
  return (
    <StSection title="前置页" padded={false}
      actions={<StButton icon={<IconAdd size={16} strokeWidth={iconStroke} />} disabled={!pages || pages.length >= most} onClick={() => setEditing({})}>新建</StButton>}>
      {error && <StNotice tone="danger">{error}</StNotice>}
      {pages?.map((p, i) => (
        <StListItem key={p.id} title={p.title} subtitle={reportsText(p.reports)} onClick={() => setEditing({ page: p })}
          trailing={
            <span className="front-page__more" onClick={(e) => e.stopPropagation()}>
              <StMenu items={menu(p, i)} trigger={(props) => (
                <StIconButton size="small" label={`${p.title} 更多`} {...props}>
                  <IconMore size={16} strokeWidth={iconStroke} />
                </StIconButton>
              )} />
            </span>
          } />
      ))}
      {editing && <FrontPageDialog key={editing.page?.id ?? "new"} meetId={meetId} page={editing.page} onClose={() => setEditing(null)} />}
      <DeleteDialog open={deleting !== null} title={`删除「${deleting?.title ?? ""}」？`} onClose={() => setDeleting(null)}
        onDelete={() => FrontPagesController.remove(meetId, deleting!.id)}>
        不再印在报表前。
      </DeleteDialog>
    </StSection>
  );
}
