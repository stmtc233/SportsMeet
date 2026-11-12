// Every write goes through act(): one place for the failure toast and the
// optional success toast. Pages never touch local data after a write; the
// dataChanged event reloads it (docs/agents/frontend.md).
import { toast } from "../design";
import { errorText } from "./errors";

export async function act<T>(fn: () => Promise<T>, done?: string): Promise<T | undefined> {
  try {
    const out = await fn();
    if (done) toast(done, "success");
    return out === undefined ? (true as T) : out;
  } catch (err) {
    toast(errorText(err), "danger");
    return undefined;
  }
}
