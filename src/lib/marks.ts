import { Marks, type Event, type EventKind } from "../mygo";

export const normalizeMark = (kind: EventKind, text: string, event?: Event) => event ? Marks.normalizeEvent(event, text) : Marks.normalize(kind, text);
