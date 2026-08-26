# The canonical store is a git working tree

`~/.slopfred/` is itself a git repository with a user-configured remote, and
slopfred drives git (pull/push) under its own `sync` command. We chose this over
treating the store as plain files synced into a separate user-managed repo
because it makes the store's history, conflict handling, and remote transport
fall out of git directly, with no bespoke sync protocol. Activations are
device-local and deliberately excluded from what is versioned/synced, since scope
placement is inherently per-machine. Trade-off: slopfred inherits git's conflict
model, and users needing a non-git transport are unsupported in v1.
