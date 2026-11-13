import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";

const Feedback = createContext<(source: string, text: string) => void>(() => {});

/** One polite announcer per page or modal; a modal's outer page is inert. */
export function StFieldFeedback({ children }: { children: ReactNode }) {
  const [text, setText] = useState("");
  const source = useRef("");
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const announce = useCallback((id: string, message: string) => {
    if (!message && source.current !== id) return;
    source.current = id;
    clearTimeout(timer.current);
    setText("");
    // Leave room for the next control's focus announcement.
    if (message) timer.current = setTimeout(() => setText(message), 500);
  }, []);
  useEffect(() => () => clearTimeout(timer.current), []);
  return <Feedback.Provider value={announce}>{children}<span className="st-sr-only" aria-live="polite" aria-atomic="true">{text}</span></Feedback.Provider>;
}

export function useFieldFeedback() {
  return useContext(Feedback);
}
