// The one place Go errors become messages (docs/agents/frontend.md).
// Validation errors arrive as finished Chinese text; codes are mapped here.
import { isCallError } from "mygo-runtime";

const codes: Record<string, string> = {
  E_STALE: "数据已在别处修改",
};

export function isStale(err: unknown): boolean {
  return isCallError(err) && err.message === "E_STALE";
}

export function errorText(err: unknown): string {
  if (isCallError(err)) return codes[err.message] ?? err.message;
  console.error(err);
  return "操作失败";
}
