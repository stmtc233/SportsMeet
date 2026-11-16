import { act } from "../../../lib/action";
import { useData } from "../../../lib/useData";
import { FrontPages, Meets, PrintStyles, type FrontPageInput, type FrontPageSaved, type Params, type PrintStyleSaved } from "../../../mygo";

/** The built-in style's id: a meet that uses it stores "". */
export const defaultStyleId = "default";

// A style's uses are meets' settings, so a meet's change matters to the list too.
const styleChanged = (c: { kind: string }) => c.kind === "printStyles" || c.kind === "meets";

export const usePrintStyles = () => useData(() => PrintStyles.list(), [], styleChanged, "printStyles");
/** The limits of the layout parameters; they are fixed while the app runs. */
export const useParamRules = () => useData(() => PrintStyles.paramRules(), [], () => false, "printParamRules");
export const usePrintStyle = (id: string) => useData(() => PrintStyles.get(id), [id], styleChanged, `printStyle:${id}`);
/** The kept versions of a style; read only while the dialog is open. */
export const usePrintStyleVersions = (id: string, enabled: boolean) =>
  useData(() => (enabled ? PrintStyles.versions(id) : Promise.resolve([])), [id, enabled], (c) => enabled && c.kind === "printStyles");

/** The images of a style (scope "style") or a meet (scope "meet"). */
export type AssetScope = "style" | "meet";
export const useAssets = (scope: AssetScope, ownerId: string) =>
  useData(() => PrintStyles.assets(scope, ownerId), [scope, ownerId],
    (c) => (scope === "style" ? c.kind === "printStyles" : c.kind === "meets" && c.meetId === ownerId), `assets:${scope}:${ownerId}`);
/** A thumbnail; the image's hash is in the key, so only a changed image is read again. */
export const useAssetImage = (scope: AssetScope, ownerId: string, name: string, sha: string) =>
  useData(() => PrintStyles.assetImage(scope, ownerId, name), [scope, ownerId, name, sha], () => false, `assetImage:${scope}:${ownerId}:${name}:${sha}`);

export const useFrontPages = (meetId: string) =>
  useData(() => FrontPages.list(meetId), [meetId], (c) => c.kind === "meets" && c.meetId === meetId, `frontPages:${meetId}`);

/** A save's result from what act() returns (a null result reads as true). */
export function savedOf<T extends object>(out: T | boolean | null | undefined): T | undefined {
  return out && typeof out === "object" ? out : undefined;
}

const styleInput = { name: null, params: null, paramsPatch: "", css: null, slots: {}, templates: {} };

export const PrintStylesController = {
  /** Saves the parameters at once; the page shows the style that is read back. */
  saveParams: (id: string, params: Params) => act(() => PrintStyles.save(id, { ...styleInput, params })),
  rename: (id: string, name: string) => act(() => PrintStyles.rename(id, name)),
  copy: (id: string) => act(() => PrintStyles.copy(id, ""), "已复制"),
  remove: (id: string) => act(() => PrintStyles.delete(id), "已删除"),
  restore: (id: string, version: number) => act(() => PrintStyles.restore(id, version), "已恢复"),
  /** The imported style, or "canceled". */
  import: () => act(async () => (await PrintStyles.import()) ?? "canceled"),
  /** "saved" or "canceled". */
  export: (id: string) => act(async () => ((await PrintStyles.export(id)) ? "saved" : "canceled")),
  /** The added image, or "canceled". */
  upload: (scope: AssetScope, ownerId: string) => act(async () => (await PrintStyles.uploadAsset(scope, ownerId)) ?? "canceled"),
  deleteAsset: (scope: AssetScope, ownerId: string, name: string) => act(() => PrintStyles.deleteAsset(scope, ownerId, name), "已删除"),
};

export const FrontPagesController = {
  /** Adds (id "") or changes a front page; the result carries the warnings. */
  save: (meetId: string, id: string, input: FrontPageInput) => act<FrontPageSaved | null>(() => FrontPages.save(meetId, id, input)),
  remove: (meetId: string, id: string) => act(() => FrontPages.delete(meetId, id), "已删除"),
  reorder: (meetId: string, ids: string[]) => act(() => FrontPages.reorder(meetId, ids)),
};

export const MeetStyleController = {
  /** Makes the meet's reports use the style ("" = 默认). */
  set: (meetId: string, styleId: string) => act(() => Meets.setPrintStyle(meetId, styleId)),
};

export type { PrintStyleSaved };
