# Activation targets standard discovery paths, not a per-tool matrix

slopfred activates by writing skill folders into vendor-neutral standard paths —
`.agents/skills/` (read by Codex, Cursor, and opencode) and `.claude/skills/`
(read by Claude Code) — rather than maintaining a hand-curated list of per-tool
destination directories. "Which tools see a skill" is a consequence of which
standard paths get written, not a configuration slopfred manages tool-by-tool.
This keeps slopfred aligned with the emerging Agent Skills ecosystem and means
new conforming tools are supported automatically. The trade-off: a tool that
uses a non-standard private path is not covered until we add it explicitly.
