# slopfred is a single Go binary

slopfred ships as a self-contained Go binary. Chosen over a Node/TypeScript
implementation (which would match the ecosystem most target tools ship in)
because slopfred's core job is filesystem projection and wrapping git, both of
which Go does with zero runtime dependencies and easy cross-platform static
binaries. Trade-off: contributors from the JS-heavy agent-tooling ecosystem face
a language switch, and we forgo reusing any Node-based Agent Skills libraries.
