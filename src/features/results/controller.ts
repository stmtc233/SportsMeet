import { act } from "../../lib/action";
import { useEffect, useState } from "react";
import { Marks, Results, type HeatStartInput, type ResultInput, type HeightPreview } from "../../mygo";

export const ResultsController = {
  save: (heatId: string, inputs: ResultInput[]) => act(() => Results.save(heatId, inputs), "已保存"),
  /** Changes or clears a heat's actual start ("" clears). */
  setStarted: (input: HeatStartInput) => act(() => Results.setStarted(input), input.startedAt ? "已保存" : "已清除"),
  height: (input: string) => act(() => Marks.normalizeHeight(input)),
};

/** Superseded previews cannot replace the view of the current draft. */
export function useHeightPreview(inputs: ResultInput[]) {
  const [state, setState] = useState<{ inputs: ResultInput[]; preview?: HeightPreview; error?: string }>();
  const [shown, setShown] = useState<HeightPreview>();
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    let current = true;
    const timer = setTimeout(() => {
      Results.previewHeight(inputs).then(
        (preview) => { if (current) { setState({ inputs, preview }); setShown(preview); } },
        () => { if (current) setState({ inputs, error: "无法校验，请重试" }); },
      );
    }, 80);
    return () => { current = false; clearTimeout(timer); };
  }, [inputs, retry]);
  // `preview` matches the current draft and gates every write; `shown` keeps
  // the last answer on screen while the next one is computed, so marks, ranks
  // and cell states do not blank out on each keystroke.
  return { ...(state?.inputs === inputs ? state : undefined), shown, retry: () => { setState(undefined); setRetry((old) => old + 1); } };
}
