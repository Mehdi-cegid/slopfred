# slopfred — v1 Spec

## Problem Statement

Developers who use multiple agent tools (Claude Code, Cursor, opencode, Codex)
across multiple machines have no single place to keep their agent **skills**.
The same `SKILL.md` folder gets hand-copied into each tool's directory on each
device, drifts out of sync, and there is no clean way to group skills, turn a
group on for one project but not another, or track a skill pulled from someone
else's repo so it can be updated later. Skills live scattered across
`~/.claude/skills/`, `.agents/skills/`, and friends, with no source of truth.

## Solution

slopfred is a single Go binary that keeps one **canonical store** of skills on
each device (`~/.slopfred/`, itself a git repo with a user-set remote). Skills
are grouped into named **packs**. A user **activates** a pack into the standard
tool discovery paths at **user** or **project** scope on the current device, and
**syncs** the store between devices explicitly over git. Skills can be
user-authored or pulled from an upstream git URL and pinned, then updated on
demand. slopfred only ever touches files it placed, so it never clobbers the
user's own hand-authored skills.

## User Stories

1. As a developer, I want to initialise a slopfred store on a new machine and
   point it at my git remote, so that I have one place for all my skills.
2. As a developer, I want to add a local skill folder to my store, so that it is
   version-controlled and syncable.
3. As a developer, I want to add a skill from an upstream git URL, so that I can
   reuse skills other people published.
4. As a developer, I want an upstream skill pinned to a specific commit, so that
   it does not change under me unexpectedly.
5. As a developer, I want to add a skill from a subpath of a larger repo, so that
   I can pull one skill out of a monorepo of skills.
6. As a developer, I want to create a named pack, so that I can group related
   skills together.
7. As a developer, I want to add and remove skill references from a pack, so that
   I can curate what the pack contains over time.
8. As a developer, I want a skill to belong to multiple packs without being
   duplicated on disk, so that my store stays clean.
9. As a developer, I want to activate a pack at user scope, so that its skills are
   available to my tools everywhere on this device.
10. As a developer, I want to activate a pack at project scope, so that its skills
    are available only inside one project directory.
11. As a developer, I want activation to place skills where Claude Code, Cursor,
    opencode, and Codex already look, so that I do not configure each tool
    individually.
12. As a developer, I want activation to copy skill folders (not symlink them), so
    that placement is predictable and portable across platforms.
13. As a developer, I want slopfred to refuse rather than overwrite when a skill
    name collides with a folder it did not place, so that my own skills are never
    destroyed.
14. As a developer, I want to deactivate a pack, so that slopfred removes exactly
    the skills it placed for that activation and nothing else.
15. As a developer, I want deactivation to leave my hand-authored skills in the
    same directory untouched, so that I can safely mix slopfred and manual skills.
16. As a developer, I want my project-scope skill files left as plain files
    without slopfred editing my `.gitignore`, so that whether they are committed
    stays my decision.
17. As a developer, I want to sync my store explicitly, so that I control when
    pull/push happens and can debug it.
18. As a developer, I want my skills and pack definitions to travel between
    devices, so that my second machine has the same store.
19. As a developer, I want my activations to stay device-local and not sync, so
    that each machine decides its own scope placements.
20. As a developer, I want to update an upstream skill to move its pin, so that I
    can adopt newer versions on demand.
21. As a developer, I want update to refuse if I have locally edited an upstream
    skill, so that my edits are not clobbered.
22. As a developer, I want clean (unedited) upstream skills to update in place, so
    that the common case is frictionless.
23. As a developer, I want a status command showing what is in my store and what
    is activated where, so that I understand my current state.
24. As a developer, I want slopfred to record the origin of each skill (local or
    upstream with URL and pin), so that provenance is never lost.
25. As a developer moving to a new machine, I want to clone/sync then activate the
    packs I want, so that I reproduce my environment without re-adding skills.

## Implementation Decisions

- **Language/runtime:** single self-contained Go binary, no runtime dependencies
  (ADR-0005).
- **Skill unit:** an Agent Skills `SKILL.md` folder, stored verbatim with only
  the standard frontmatter fields, so it copies unmodified into SKILL.md-native
  tools.
- **Canonical store:** `~/.slopfred/`, which **is** a git working tree with a
  user-configured remote; slopfred drives `git pull`/`git push` under its `sync`
  command (ADR-0003). Store layout: `skills/<skill-name>/` for skill folders; a
  slopfred-owned **sidecar manifest** at the store root for pack definitions and
  per-skill origin.
- **Pack:** a named, flat, ordered list of skill references (by name) held in the
  sidecar manifest. No nesting/composition in v1. A skill is stored once and may
  be referenced by many packs (library model).
- **Origin:** each skill records `local` or `upstream{git-url, optional subpath,
  pinned commit}` in the sidecar manifest — never in `SKILL.md`, to keep the
  skill portable.
- **Activation = copy, not symlink** (ADR-0001): copy each referenced skill
  folder from the store into the tools' **standard discovery paths** — not a
  per-tool matrix (ADR-0002):
  - user scope → `~/.agents/skills/` and `~/.claude/skills/`
  - project scope → `<project>/.agents/skills/` and `<project>/.claude/skills/`
- **Scope:** exactly two — user and project — mirroring the tools' own discovery
  scopes.
- **Ownership / clobber-safety** (ADR-0004): slopfred only overwrites or deletes
  folders it recorded placing. Name collision with a folder it did not place is a
  refuse-and-warn. slopfred never edits the user's `.gitignore`.
- **Activation record:** device-local, git-ignored bookkeeping at
  `~/.slopfred/local/activations.json` recording, per activation: pack, scope,
  target path(s), and the exact skill folders written. Never synced.
- **Sync:** explicit `slopfred sync` = git pull + push of the store. Skills and
  pack manifests travel; activations do not (they live in the git-ignored local
  record).
- **Upstream add:** `add <git-url>[#subpath]` pulls one named skill folder and
  pins its commit. Bulk "import all skills from a repo" is deferred.
- **Update:** `update [<skill>]` re-pulls upstream skills and moves the pin;
  refuses if the local copy has diverged from the pinned ref; clean upstream
  skills update in place.
- **Command surface (v1):** `init`, `add`, `pack`, `activate`, `deactivate`,
  `sync`, `update`, `status`.
- **Module shape:** a single `slopfred` core package exposing the eight
  operations as an in-process API; a thin CLI layer parses arguments and calls
  it.

## Testing Decisions

- **What makes a good test:** assert external, observable behaviour — files on
  disk at the correct paths, contents of the activation record and sidecar
  manifest, git remote state, and refuse-vs-clobber outcomes. Do not assert
  internal function calls or private structure.
- **Single behavioural seam (the slopfred core API).** Drive the eight
  operations against a sandbox: a temp `SLOPFRED_HOME`, a real local **bare git
  remote**, and temp directories standing in for the tools' user/project
  discovery paths. Use the real filesystem and real git — do not mock them.
- **Key scenarios to cover:** init wires a remote; add (local and
  upstream#subpath) records correct origin and pin; pack curation adds/removes
  references without duplicating storage; activate places folders at the four
  standard paths per scope and writes the activation record; collision with a
  pre-existing foreign folder refuses; deactivate removes only recorded folders
  and leaves foreign folders intact; sync round-trips skills+packs between two
  stores sharing a remote while activations stay local; update moves a clean pin
  and refuses on divergence; status reflects store + activations.
- **CLI layer:** minimal coverage of argument wiring only; behaviour is proven at
  the core seam.
- **Prior art:** none yet (greenfield); this spec establishes the seam.

## Out of Scope

- MCP servers and the agent-plugins.org packaging format (`plugin.json`).
- Format translation for Copilot (`.github/*instructions.md`) and Cursor `.mdc`
  rules; v1 is copy-only for SKILL.md-native tools.
- Symlink-based activation / live editing.
- Automatic or background/daemon sync.
- Bulk "add every skill in a repo" import.
- Pack composition/nesting.
- Non-git sync transports.
- Adopting or cleaning up the user's pre-existing hand-authored skills.

## Further Notes

- The vendor-neutral `.agents/skills/` path is read by Codex, Cursor, and
  opencode; `.claude/skills/` covers Claude Code. Writing those two locations
  covers all four v1 target tools with no per-tool configuration, and new
  conforming tools are supported automatically.
- Because slopfred owns only what it placed, a colliding skill name blocks
  activation until the user resolves it — this is intentional and preferred over
  any silent overwrite.
- Glossary and rationale live in `CONTEXT.md` and `docs/adr/0001`–`0005`.
- **Publishing:** the project tracker is Linear (via MCP), but those tools were
  not available in this session, so this spec was written to `SPEC.md`. When
  filing in Linear, apply the `ready-for-agent` label.
