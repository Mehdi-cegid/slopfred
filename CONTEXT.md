# slopfred

A CLI that syncs agent *skills* across a user's devices and across the agent
tools they use. One canonical store of skills, grouped into named sets, activated
per-device, travelling between machines via a user-owned git repo.

## Language

**Skill**:
An Agent Skills unit: a directory named for the skill, containing a `SKILL.md`
(with `name` + `description` frontmatter) plus any sibling files (`scripts/`,
`references/`, `assets/`). Stored canonically with only the standard frontmatter
fields so it copies unmodified into SKILL.md-native tools.
_Avoid_: prompt, rule, instruction

**Pack**:
A named, flat, ordered collection that references skills by name. A pack is a
manifest, not a container: the skills it lists live once in the canonical store,
not inside the pack. Packs do not nest in v1.
_Avoid_: set, group, bundle, collection

**Canonical store**:
The single source-of-truth copy of every skill on a device, held in the slopfred
home (`~/.slopfred/skills/<skill-name>/`). Activation projects *out* of here into
tool locations; it is never a tool's own skills directory.
_Avoid_: library, repo, cache

**Target tool**:
An agent tool slopfred can activate skills into (v1: SKILL.md-native tools such
as Claude Code, Cursor, opencode, Codex).
_Avoid_: client, editor, host

**Activation**:
The act of making a pack's skills discoverable by target tools at a given scope
on a device, by copying each referenced skill folder from the canonical store
into the tools' standard discovery paths.
_Avoid_: install, enable, link

**Scope**:
Where an activation takes effect on a device: **user** (this device, everywhere)
or **project** (this device, one directory). Mirrors the tools' own user/project
discovery scopes.
_Avoid_: level, context, target

**Sync**:
Explicitly reconciling this device's canonical store with the user-owned git
remote (pull + push). Skills and pack manifests travel; activations do not.
_Avoid_: push, backup, replicate

**Origin**:
Per-skill provenance recorded in slopfred's sidecar manifest: either **local**
(user-authored) or **upstream** (pulled from a git URL + optional subpath, pinned
to a commit). Moving an upstream pin is an explicit update; update refuses if the
local copy has diverged from the pinned ref rather than clobbering user edits.
_Avoid_: source, provenance, remote

**Sidecar manifest**:
slopfred-owned metadata at the store root holding pack definitions and per-skill
origin. It is versioned and synced, but never copied during activation — only
skill folders are projected into target tools.
_Avoid_: config, index, lockfile

**Activation record**:
Device-local, git-ignored bookkeeping (`~/.slopfred/local/activations.json`) of
exactly what each activation placed: pack, scope, path, and the skill folders
written. slopfred only ever removes or overwrites folders listed here; it never
touches skills it did not place.
_Avoid_: state, manifest, log
