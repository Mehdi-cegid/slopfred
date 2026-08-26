# Issue tracker: Linear

Issues and specs for this repo live in Linear. Skills interact with Linear
through the **Linear MCP server** tools (e.g. `create_issue`, `list_issues`,
`get_issue`, `update_issue`, `create_comment`).

## Conventions

- **Create an issue**: use the MCP `create_issue` tool with a title and
  markdown description.
- **Read an issue**: use `get_issue` (and fetch its comments) by issue id or
  identifier (e.g. `ENG-123`).
- **List issues**: use `list_issues`, filtering by team, state, and labels as
  needed.
- **Comment on an issue**: use `create_comment` against the issue id.
- **Apply / remove labels**: use `update_issue` to set the issue's labels.
- **Close / change state**: use `update_issue` to move the issue to the
  appropriate workflow state (e.g. Done, Canceled).

**Default team/project:** ENG f1263c46-8640-4eae-b93d-e7fdeb940240 . Use this project by default when
creating issues unless told otherwise.

## When a skill says "publish to the issue tracker"

Create a Linear issue via the MCP `create_issue` tool.

## When a skill says "fetch the relevant ticket"

Use the MCP `get_issue` tool for the referenced identifier, including comments.

## Triage roles → Linear labels

Triage roles map to Linear labels of the same name (see
`docs/agents/triage-labels.md`). Apply them via `update_issue`.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single Linear issue; child tickets are
sub-issues linked to it via the parent relationship. Express blocking with
Linear's native issue relations ("blocked by" / "blocks"). A child is unblocked
when every blocker is in a completed/canceled state.
