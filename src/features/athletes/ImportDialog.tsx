import { useEffect, useState } from "react";
import { StButton, StCheckbox, StDialog, StNotice, StRow, StSelect, StTable, StTag, StTextArea, type StOption } from "../../design";
import type { AthleteImportPreview, ImportRow, Table } from "../../mygo";
import { AthletesController } from "./controller";

const columns: StOption<string>[] = [
  { value: "", label: "不导入" },
  { value: "name", label: "姓名" },
  { value: "gender", label: "性别" },
  { value: "bib", label: "号码" },
  { value: "studentNumber", label: "学号" },
  { value: "unit", label: "参赛单位" },
  { value: "grade", label: "年级" },
  { value: "group", label: "参赛组别" },
  { value: "className", label: "班级" },
  { value: "birthDate", label: "出生日期" },
  { value: "events", label: "报名项目" },
  { value: "note", label: "备注" },
];

const actions: Record<string, { text: string; tone: "success" | "info" | "danger" }> = {
  create: { text: "新建", tone: "success" },
  update: { text: "更新", tone: "info" },
  error: { text: "有误", tone: "danger" },
};

/** Pasted or loaded table → column mapping → preview → import. */
export function ImportDialog({ meetId, open, onClose }: { meetId: string; open: boolean; onClose: () => void }) {
  const [text, setText] = useState("");
  const [table, setTable] = useState<Table | null>(null);
  const [mapping, setMapping] = useState<string[]>([]);
  const [header, setHeader] = useState(false);
  const [preview, setPreview] = useState<AthleteImportPreview | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (open) {
      setText("");
      setTable(null);
      setPreview(null);
    }
  }, [open]);

  const input = () => ({ rows: (table?.rows ?? []).slice(header ? 1 : 0), mapping });

  // The preview follows the mapping.
  useEffect(() => {
    if (!table) return;
    let live = true;
    setPreview(null);
    if (!mapping.includes("name")) return;
    void AthletesController.preview(meetId, input()).then((p) => {
      if (live && p) setPreview(p);
    });
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [table, mapping, header, meetId]);

  const parse = async (source: string) => {
    const t = await AthletesController.parse(source);
    if (!t) return;
    if (t.rows.length === 0) return;
    setTable(t);
    setMapping(t.mapping);
    setHeader(t.hasHeader);
  };
  const pick = async () => {
    const content = await AthletesController.pickTable();
    if (content) await parse(content);
  };
  const run = async () => {
    setBusy(true);
    const done = await AthletesController.import(meetId, input());
    setBusy(false);
    if (done) onClose();
  };

  const sample = (table?.rows ?? []).slice(header ? 1 : 0)[0] ?? [];
  const heads = table?.rows[0] ?? [];
  return (
    <StDialog open={open} title="导入名单" size="large" onClose={onClose}
      actions={
        table ? (
          <>
            <StButton onClick={() => setTable(null)}>上一步</StButton>
            <StButton variant="primary" loading={busy} disabled={!preview || preview.errors > 0 || preview.rows.length === 0} onClick={run}>导入</StButton>
          </>
        ) : (
          <>
            <StButton onClick={pick}>选择 CSV 文件</StButton>
            <StButton variant="primary" disabled={text.trim() === ""} onClick={() => parse(text)}>下一步</StButton>
          </>
        )
      }>
      {!table && <StTextArea label="从表格复制后粘贴到这里" value={text} onChange={setText} rows={12} placeholder={"姓名\t性别\t学号\t班级\t报名项目\n王五\t男\t1002\t初一(1)班\t100m、跳远"} />}
      {table && (
        <>
          <StRow>
            <StCheckbox checked={header} onChange={setHeader} label="第一行是表头" />
          </StRow>
          <div className="mapping">
            {mapping.map((m, i) => (
              <StSelect key={i} width="fill" label={header ? heads[i] || `第 ${i + 1} 列` : sample[i] || `第 ${i + 1} 列`} value={m} options={columns}
                onChange={(v) => setMapping((list) => list.map((old, j) => (j === i ? v : v !== "" && old === v ? "" : old)))} />
            ))}
          </div>
          {!mapping.includes("name") && <StNotice tone="warning">请指定「姓名」列</StNotice>}
          {preview && (
            <>
              <StRow>
                <StTag tone="success">新建 {preview.creates}</StTag>
                <StTag tone="info">更新 {preview.updates}</StTag>
                <StTag tone={preview.errors > 0 ? "danger" : "neutral"}>有误 {preview.errors}</StTag>
                {preview.newUnits.length > 0 && <StTag>新单位 {preview.newUnits.length}</StTag>}
                {preview.entries > 0 && <StTag>报名 {preview.entries}</StTag>}
              </StRow>
              <div className="preview-table view-enter">
                <StTable<ImportRow> dense rows={preview.rows} rowKey={(r) => String(r.line)}
                  columns={[
                    { key: "line", title: "行", width: "56px", align: "end", render: (r) => r.line },
                    { key: "action", title: "操作", width: "72px", render: (r) => <StTag tone={actions[r.action]?.tone}>{actions[r.action]?.text}</StTag> },
                    { key: "name", title: "姓名", width: "160px", render: (r) => r.name },
                    { key: "message", title: "说明", render: (r) => r.message },
                  ]} />
              </div>
            </>
          )}
        </>
      )}
    </StDialog>
  );
}
