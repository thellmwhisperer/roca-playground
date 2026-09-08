# roca-playground

Optional human answering for [La Roca](https://github.com/thellmwhisperer/la-roca).
The executable owns `playground`, `explore`, model selection, local agent CLI
transports, Ollama, SQL repair and result interpretation. Core `exec` and
`query` remain zero-inference reads.

```sh
roca plugin install thellmwhisperer/roca-playground
roca playground "what decisions were made about the format"
roca playground --full "what decisions were made about the format"
roca explore --deep "format"
```

The existing `[models]` configuration and `roca model`, `roca models`, and
`roca login` commands continue through this plugin. Core doctor probes providers
only while the executable is installed. The plugin is an executable-only
schema-1 package under La Roca's `docs/plugins.md` contract.

Build and run the moved unit and acceptance tests with Go 1.25.5 or newer:

```sh
make check
make package
roca plugin install ./dist/package
```

`make` fetches the matching core source into `.worktrees/core`. For paired
changes, use `make check CORE_DIR=<core-checkout>`. The module keeps La Roca's
internal namespace so it can reuse the same read-only SQLite engine and wire
formats without copying them. It is distributed as an executable, not a Go
library. Provider tests use synthetic homes and local fake model servers.

Core delegation uses `--transport` before the verb: stdout remains the live
command output, while stderr carries one JSON envelope with `stderr`, a row-free
`query`, and `cleaned_sql`. Core owns durable auditing; the plugin does not append
a second call record. Direct plugin commands keep their ordinary streams.
