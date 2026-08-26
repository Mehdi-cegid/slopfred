# slopfred owns only what it placed

Activation copies skill folders into shared discovery paths (`.agents/skills/`,
`.claude/skills/`) that the user may also populate with hand-authored skills. To
avoid destroying the user's own work, slopfred records every folder it places in
the device-local activation record and will only ever overwrite or delete folders
it owns. A name collision with a folder slopfred did not place is a refuse-and-warn,
never a silent overwrite. slopfred also never modifies the user's `.gitignore`
for project-scope placements; whether those files are committed is the user's
call. Trade-off: slopfred cannot "adopt" or clean up pre-existing user skills,
and a colliding name blocks activation until the user resolves it.
