# Activation copies skill folders, does not symlink

Activating a pack copies each referenced skill folder from the canonical store
(`~/.slopfred/skills/`) into the target tools' discovery paths. We chose copy
over symlink because copies are portable across platforms (symlinks are
unreliable on Windows and some tools do not follow them) and predictable (what
is on disk is exactly what the tool reads). The cost is that edits to a skill
must be re-projected by re-running activation/sync; symlink-based live editing
may be added later as an opt-in.
