import type { AgentIcon } from "../../design";

// Local clients with HTTP transport and authentication headers; sources in docs/agents/mcp.md.
export const supportedAgents: readonly { agent: AgentIcon; name: string }[] = [
  { agent: "trae", name: "TRAE" },
  { agent: "qoder", name: "Qoder" },
  { agent: "codebuddy", name: "CodeBuddy" },
  { agent: "workbuddy", name: "WorkBuddy" },
  { agent: "qwen", name: "Qwen Code" },
  { agent: "kimi", name: "Kimi Code" },
  { agent: "claude", name: "Claude Code" },
  { agent: "codex", name: "Codex" },
  { agent: "cursor", name: "Cursor" },
  { agent: "gemini", name: "Gemini CLI" },
  { agent: "antigravity", name: "Google Antigravity" },
  { agent: "cline", name: "Cline" },
];
