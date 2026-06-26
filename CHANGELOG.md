# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.21.31](https://github.com/flanksource/clicky/compare/v1.21.30...v1.21.31) (2026-06-26)


### 👷 CI/CD

* **release:** rename goreleaser folder field to directory ([29bb8d6](https://github.com/flanksource/clicky/commit/29bb8d6575c8757250d4cc717425a56c4a2cc866))

## [1.21.30](https://github.com/flanksource/clicky/compare/v1.21.29...v1.21.30) (2026-06-26)


### 🐛 Bug Fixes

* **aichat:** pin clicky require to v1.21.29 so go get works ([95378c7](https://github.com/flanksource/clicky/commit/95378c720a13cf252c84a90f659b4248e51f599b))

## [1.21.29](https://github.com/flanksource/clicky/compare/v1.21.28...v1.21.29) (2026-06-26)


### ⚠ BREAKING CHANGES

* **cli:** Removed SourceMarkdown field from markdown Node and Document structures; removed PreserveSource option from markdown parser. Parser provenance fields are no longer included in clicky-json output.
* **lint:** Violation struct now includes Severity field; Result.Success now depends on HasErrors() rather than HasIssues().
* **rpc:** RPCOperation.Command is now entity.ExecutableCommand instead of *cobra.Command. Callers must wrap cobra commands with NewCobraExecutableCommand or use the new interface methods (Path, Name, RootName, Runnable, Hidden, IsBoolFlag, Execute).
* **entity:** Internal package reorganization; public API preserved via root clicky re-exports.
* **aichat:** ApprovalPolicy signature unchanged but ToolApprovalPolicy takes precedence; initGenkit now accepts optional ProviderCredential args; ThreadStore.Delete method added to interface; ClickyToolset.DefineTools now wraps DefineRegisteredTools which returns registeredTool structs.

### ♻️ Code Refactoring

* **entity,rpc,mcp,task:** extract entity model to subpackage and unify command execution ([074db76](https://github.com/flanksource/clicky/commit/074db7682fd3dadb6aabf800fc47939782691b33))
* **entity:** move entity package from root clicky to dedicated subpackage ([4a7a96b](https://github.com/flanksource/clicky/commit/4a7a96be0b1d8c54ff3dc3aaa9950ea06fd4e9d5))
* **entity:** reorganize entity-related files into dedicated package ([042e4f6](https://github.com/flanksource/clicky/commit/042e4f6f223a3466ca1da96a5f597337b258c134))
* **rpc:** extract cobra command execution to entity.ExecutableCommand adapter ([de23c99](https://github.com/flanksource/clicky/commit/de23c997673f2164b32528833f4d335e591bbe16))


### ✅ Tests

* **markdown:** add kitchen sink roundtrip and clicky JSON test ([0b9045a](https://github.com/flanksource/clicky/commit/0b9045af69ee2128c85f6482cfe8ddd3391037af))
* **prompt_test:** Fix race condition in PTY output capture by waiting for reader drain ([812891a](https://github.com/flanksource/clicky/commit/812891a9b35b0fe1897b9ed4d406adcef9f30949))


### ✨ Features

* **aichat:** Add tool grouping support for preferences UI ([da7cc53](https://github.com/flanksource/clicky/commit/da7cc5332fd39c9d19859b44df7bc511a5e96d47))
* **aichat:** Add tool preferences, runtime settings, and request-scoped provider credentials ([c800642](https://github.com/flanksource/clicky/commit/c800642ea1119420d2e295b661baf055b70a3ca2))
* **api:** add support for markdown block elements (heading, blockquote, footnotes) ([ebbb9ac](https://github.com/flanksource/clicky/commit/ebbb9ac4c139e815048ae8b3929e1a4fe5e9dc9d))
* **cli:** add schema-less input format support for markdown, json, yaml, and text ([89e6d71](https://github.com/flanksource/clicky/commit/89e6d71cb30a6723c461da5a8ea8a81fb88db997))
* **docs,entity,examples:** Add local docs server and improve entity authoring guidance ([9679a4a](https://github.com/flanksource/clicky/commit/9679a4ac0629f1e5c6dac7b4eee6e6b85fcfc44e))
* **entity:** add reusable named filters with typed and dynamic entity support ([7a830f3](https://github.com/flanksource/clicky/commit/7a830f312dae3b6178ab2b79c0e009cdc0ae4789))
* **examples:** add markdown editor page with preview API ([c8076ea](https://github.com/flanksource/clicky/commit/c8076ea4713495da4de18b6d57152f782251d83d))
* **examples/entity:** add Vite HMR dev server support for local clicky-ui development ([9912ca1](https://github.com/flanksource/clicky/commit/9912ca1658648a6bb4e0afa0995698089fcc63e7))
* **exec:** detect compiler activity during process startup ([dc6ebb8](https://github.com/flanksource/clicky/commit/dc6ebb8d3fef20fdb42632ec907d692c15c98b70))
* **lint:** add severity levels and entity registration checks ([6c5ce9b](https://github.com/flanksource/clicky/commit/6c5ce9b4839a587b49504719a37ebdaed8057c22))
* **markdown:** Add markdown parsing with structured document support ([656e8e4](https://github.com/flanksource/clicky/commit/656e8e456834e82c07fedbb933b516bd42d94fc6))


### 🐛 Bug Fixes

* **aichat,entity,markdown,rpc,cmd,lint:** address CodeRabbit findings on PR [#119](https://github.com/flanksource/clicky/issues/119) ([1742312](https://github.com/flanksource/clicky/commit/17423120ed39d6e1841797ee385e0d4cb1ca940b)), closes [#127](https://github.com/flanksource/clicky/issues/127) [#128](https://github.com/flanksource/clicky/issues/128)
* **rpc,aichat,lint,api,exec:** address PR [#119](https://github.com/flanksource/clicky/issues/119) CI failures and review comments ([e5e0aea](https://github.com/flanksource/clicky/commit/e5e0aeae5e6ce5ff5dadd4b34e0e22d210c6612b))


### 👷 CI/CD

* **release:** add sub-module release workflow ([e273b40](https://github.com/flanksource/clicky/commit/e273b404bca1ff7ca56ff0524c3751b80620ef56))


### 📚 Documentation

* **gitignore:** Remove *.md from gitignore and add documentation files ([71862e0](https://github.com/flanksource/clicky/commit/71862e0fd2ebc4b46c4ec20cbe4b9b266b9d9f1e))

## [1.21.28](https://github.com/flanksource/clicky/compare/v1.21.27...v1.21.28) (2026-06-23)


### 🐛 Bug Fixes

* **aichat:** preserve dynamic tool names across chat turns ([156744d](https://github.com/flanksource/clicky/commit/156744d7550f8d14091250e6f5cbaf887e404ea8))

## [1.21.27](https://github.com/flanksource/clicky/compare/v1.21.26...v1.21.27) (2026-06-23)


### ✨ Features

* **aichat:** add runtime model registry ([58410db](https://github.com/flanksource/clicky/commit/58410dbdaaf73a353005520d5f01fddad99d4275))

## [1.21.26](https://github.com/flanksource/clicky/compare/v1.21.25...v1.21.26) (2026-06-23)


### ✨ Features

* **formatters:** add Objects/ExecutionRoots payload fields to ClickyNode ([84f4845](https://github.com/flanksource/clicky/commit/84f4845e62c8991463aaca994d7842e6f3f907f9))

## [1.21.25](https://github.com/flanksource/clicky/compare/v1.21.24...v1.21.25) (2026-06-20)


### ✅ Tests

* **batch:** uncomment TestBatch_ErrorIdentification ([1baedde](https://github.com/flanksource/clicky/commit/1baedde5f671e88237257b99320d44691798f98e))
* **formatters:** cover map[string]any input for markdown paths ([2dd611a](https://github.com/flanksource/clicky/commit/2dd611a365d2c161038b9244829a23f7aa0ac79f)), closes [flanksource/clicky#38](https://github.com/flanksource/clicky/issues/38)


### 🐛 Bug Fixes

* **batch:** detect item timeout by elapsed time, not context state ([391e6f5](https://github.com/flanksource/clicky/commit/391e6f5958c21cbdec51f2b01b83d46bdc18d393))
* **formatters:** handle map input in ToPrettyData ([c21fd39](https://github.com/flanksource/clicky/commit/c21fd3991ad84233f1d49b42204b68de0573c615))

## [1.21.24](https://github.com/flanksource/clicky/compare/v1.21.23...v1.21.24) (2026-06-16)


### ✨ Features

* **aichat:** add newer Anthropic models ([56ed4f0](https://github.com/flanksource/clicky/commit/56ed4f064f7c2ef20eba08b21c503acf247981a4))


### 🐛 Bug Fixes

* **aichat:** set Anthropic max tokens ([d6f4892](https://github.com/flanksource/clicky/commit/d6f4892110c7eb9da0101ee5d43cb00ce08bf554))

## [1.21.23](https://github.com/flanksource/clicky/compare/v1.21.22...v1.21.23) (2026-06-15)


### ✨ Features

* **exec:** add process supervision with resource limits and port detection ([e8972b6](https://github.com/flanksource/clicky/commit/e8972b6f44b1e12b861cc8720eb2cd2f28573fc3)), closes [#123](https://github.com/flanksource/clicky/issues/123)
* **webapp:** Wire ChatWidget to operations client for tool parity ([b542518](https://github.com/flanksource/clicky/commit/b542518dd3684db6bf35f24f1c5482745398ba20))

## [1.21.22](https://github.com/flanksource/clicky/compare/v1.21.21...v1.21.22) (2026-06-15)


### 🐛 Bug Fixes

* Improve in-memory timeseries store write performance ([#110](https://github.com/flanksource/clicky/issues/110)) ([bf79215](https://github.com/flanksource/clicky/commit/bf79215890ea1059a38c6905b6f723a311bb84f9))

## [1.21.21](https://github.com/flanksource/clicky/compare/v1.21.20...v1.21.21) (2026-06-14)


### 🔧 Maintenance

* add Apache 2.0 license ([1ef624b](https://github.com/flanksource/clicky/commit/1ef624be7410db462cdb074730492d9d01825275))

## [1.21.20](https://github.com/flanksource/clicky/compare/v1.21.19...v1.21.20) (2026-06-14)


### ⚠ BREAKING CHANGES

* **api:** NewTableFrom now returns a table with headers for empty slices instead of an empty table. TryTypedValue now returns a TypedValue with a header-only table for empty TableProvider slices instead of nil.

### ✅ Tests

* **task:** add testdata YAML files for task testing ([990f51d](https://github.com/flanksource/clicky/commit/990f51d5ff2b53ff27d0b087588880dcc1da94e8))
* **task:** isolate batch tests from global task manager ([ba6a373](https://github.com/flanksource/clicky/commit/ba6a373da524b15e6dadceed895b9b2ba18474df))


### ✨ Features

* **aichat:** Add AI chat backend with tool approval and conversation persistence ([ebabb9b](https://github.com/flanksource/clicky/commit/ebabb9b08a3f044a82300a4858d7c406cd3dda1c))
* **aichat:** Add token usage tracking and cost calculation for chat turns ([3cf6bee](https://github.com/flanksource/clicky/commit/3cf6bee96b03e3191dccb5a9f73eec11d30c608c))
* **api:** emit table schema for empty TableProvider slices ([b3157fd](https://github.com/flanksource/clicky/commit/b3157fdfef6ecf388cde7704965a68f795636d00))

## [1.21.19](https://github.com/flanksource/clicky/compare/v1.21.18...v1.21.19) (2026-06-12)


### ✨ Features

* **sse:** add RunsSSEHandler for streaming run listings with deduplication ([f4bb6ca](https://github.com/flanksource/clicky/commit/f4bb6cabde8c5be14dd25041799d39e2d8ed2ab4))

## [1.21.18](https://github.com/flanksource/clicky/compare/v1.21.17...v1.21.18) (2026-06-11)


### ✨ Features

* **cache:** add generic cache-browser API with valkey impl ([#114](https://github.com/flanksource/clicky/issues/114)) ([8e20a9a](https://github.com/flanksource/clicky/commit/8e20a9add8b422d1e8147407a2f9c6acd5c5c7ef))

## [1.21.17](https://github.com/flanksource/clicky/compare/v1.21.16...v1.21.17) (2026-06-09)


### ⚠ BREAKING CHANGES

* **docs:** --output-dir now writes one markdown file per controller directly into the directory instead of scaffolding a provider-based docs site. The --provider and --base-path flags are removed. Generated files no longer include YAML frontmatter.

### ♻️ Code Refactoring

* **docs:** simplify docs generation to write flat markdown files instead of scaffolding provider-based sites ([ef73ee7](https://github.com/flanksource/clicky/commit/ef73ee773ee94df5c78afe36ea8968096f6e3b3e))


### ✨ Features

* **docs:** add docs generation command with CLI reference and UI catalog ([cabe31f](https://github.com/flanksource/clicky/commit/cabe31fe46b8189afabd2df3d4268f2abba49b52))
* **task:** pluggable LiveRenderer for custom live/final rendering ([3be5046](https://github.com/flanksource/clicky/commit/3be50462948cc281f0737a41aa214b237d0cd92d))


### 🐛 Bug Fixes

* **api:** prevent blank lines in tree labels and large description lists from causing excessive padding ([24e9666](https://github.com/flanksource/clicky/commit/24e966668f776304b2344ee45328ae3813615a99))

## [1.21.16](https://github.com/flanksource/clicky/compare/v1.21.15...v1.21.16) (2026-06-09)


### ✨ Features

* **api,rpc:** Add CollapseANSI flag and multi-operand action path handling ([633d28b](https://github.com/flanksource/clicky/commit/633d28b2ae8ed112aa27bb44d308810eb69371b9))

## [1.21.15](https://github.com/flanksource/clicky/compare/v1.21.14...v1.21.15) (2026-06-07)


### ⚠ BREAKING CHANGES

* **entity:** runEntityOp now extracts paged results via PageRows() interface method before formatting, changing how paged responses are serialized in RPC responses.

### ✨ Features

* **entity:** add pagination support for entity list operations ([6903915](https://github.com/flanksource/clicky/commit/69039150788c3891ff7e9b8a82091830fbdf7e87))
* **rpc,entity:** add context-aware filter lookup support for request-scoped state ([442a560](https://github.com/flanksource/clicky/commit/442a56059fbadd47165a1ab0b3c69535017a031d))


### 🐛 Bug Fixes

* **entity,rpc:** address PR review comments and lint e2e go.sum failures ([28bd967](https://github.com/flanksource/clicky/commit/28bd96731b1703bc922a50bc2bb91f71fc2ee547))


### 📦 Build System

* **makefile:** add go mod tidy to fmt target for all modules ([eb82367](https://github.com/flanksource/clicky/commit/eb823671fd7b6d2863db35acf7ec8e0fed3c38cb))

## [1.21.14](https://github.com/flanksource/clicky/compare/v1.21.13...v1.21.14) (2026-06-03)


### ⚠ BREAKING CHANGES

* **entity,command,rpc:** None - all changes are additive with backward compatibility maintained through fallback to non-context variants.
* **task:** RunFilter.matches() renamed to RunFilter.Matches(); runMetaFromSnapshot() renamed to RunMetaFromSnapshot()
* **api,openapi,lint:** annotateEntityOperationCommand now requires an additional optionalID boolean parameter.

### ♻️ Code Refactoring

* **timeseries:** replace oipa references with generic app prefix ([d0cb68e](https://github.com/flanksource/clicky/commit/d0cb68e7b32f9229683caa2d5ba04e24bb867e3d))


### ✨ Features

* **api,openapi,lint:** Add optional ID support for entity actions and expand formatting helpers ([706ab62](https://github.com/flanksource/clicky/commit/706ab6203ec9b003e361084457b75d814d59297f))
* **entity,command,rpc:** add context-aware data functions for request-scoped state ([394a69c](https://github.com/flanksource/clicky/commit/394a69c15f8af3a076a1ca41de8312c8e2f8925b))
* **lint:** add render builder validation and helper-backed type detection ([dd00e26](https://github.com/flanksource/clicky/commit/dd00e26f9b175d6fff211a319dde1fe8dffc9940))
* **metrics:** add timeseries metrics store with in-memory and valkey backends ([786c3a5](https://github.com/flanksource/clicky/commit/786c3a5d8756f7f60378d0cffe28168241c2c9dc))
* **task:** Add OnBeforeGC hook and RunsRaw for GC lifecycle control ([4505a9f](https://github.com/flanksource/clicky/commit/4505a9f9cf1010acbbbe721f359348a2c42829ce))
* **task:** Add task registry with run listing, filtering, and drill-down APIs ([3045cfc](https://github.com/flanksource/clicky/commit/3045cfc7ba916da38e41701b099b3e4f15c7580d))


### 🐛 Bug Fixes

* **examples:** tag standalone demos with //go:build ignore ([11bbd07](https://github.com/flanksource/clicky/commit/11bbd075df9a163cb81628032a8b1ddaf71bddd8))
* **rpc:** serve ExecutionResponse envelope for structured wire formats ([f01a56a](https://github.com/flanksource/clicky/commit/f01a56a0d67c81fb486ce4ba8b43812602df244e))
* **task:** WaitFor returns result after in-closure terminal SetStatus ([d058287](https://github.com/flanksource/clicky/commit/d058287ac87ad0d2d652fd99055130439d7b3e80))


### 👷 CI/CD

* **workflows:** consolidate CI/CD workflows into gavel and dist ([6c9d5d5](https://github.com/flanksource/clicky/commit/6c9d5d5523a6583ba9eb4522e5db5bdc594f37a3))
* **workflows:** run gavel lint as raw step to avoid --show-passed injection ([a58f5c4](https://github.com/flanksource/clicky/commit/a58f5c4f3fc923fef40000454086904cc1edbdae))
* **workflows:** split gavel into separate test and lint jobs with node setup ([ce3c9c6](https://github.com/flanksource/clicky/commit/ce3c9c654148da4a5676fdfe6b9b05f3069094fb))


### 📚 Documentation

* **cache:** clarify SQLite driver selection rationale in comments ([8744c01](https://github.com/flanksource/clicky/commit/8744c0183331725fbca6bc81023ca6cd292f732e))


### 📦 Build System

* **cache:** Replace go-sqlite3 with modernc.org/sqlite for CGO-free builds ([7d4e343](https://github.com/flanksource/clicky/commit/7d4e3431678b3a17415d131c034de13c398f1f10))
* **webapp:** Add placeholder index.html for go:embed resolution in CI ([c4eee78](https://github.com/flanksource/clicky/commit/c4eee78d97d6e4fd566eacf83682494daddce08f))


### 🔧 Maintenance

* update docs ([df28a6c](https://github.com/flanksource/clicky/commit/df28a6c406da53e500222cd89c5ff505b98f3cb5))
* **examples:** tidy go.mod after tagging demos as build-ignore ([5e91e2f](https://github.com/flanksource/clicky/commit/5e91e2fe67617348ed5b45f9ac63fa2d84ce4f2b))

## [1.21.13](https://github.com/flanksource/clicky/compare/v1.21.12...v1.21.13) (2026-05-31)


### ♻️ Code Refactoring

* **exec:** Add thread-safe concurrent access to Process state ([235aac0](https://github.com/flanksource/clicky/commit/235aac06fd8a1ac134cf347411dd6deeb33228ed))


### 🔧 Maintenance

* **embed:** commit task-ui bundle + build it in release CI ([3d3b234](https://github.com/flanksource/clicky/commit/3d3b2342dbce51f9033fe1dc3fdebe59e68136e3))

## [1.21.12](https://github.com/flanksource/clicky/compare/v1.21.11...v1.21.12) (2026-05-31)


### ✨ Features

* **flags:** support comma-delimited shorthand in flag tags ([e498bb0](https://github.com/flanksource/clicky/commit/e498bb040a47fe314d52dcb9cc9d6da74cf1930f))


### 🔧 Maintenance

* bump gomplate and is-healthy for kubernetes v0.36.1 ([ac2b3d1](https://github.com/flanksource/clicky/commit/ac2b3d17cc8077cfda1a3aeda17b36d272f9f95e))

## [1.21.11](https://github.com/flanksource/clicky/compare/v1.21.10...v1.21.11) (2026-05-31)


### ✨ Features

* **api:** add Admonition and Keyed types for structured content rendering ([7a04589](https://github.com/flanksource/clicky/commit/7a0458939175a9597a129011e8daebb988b79307))
* **api:** Add PrettyShort interface for compact cell rendering and Link JSON payload support ([7702184](https://github.com/flanksource/clicky/commit/770218495b38eafa706ad31ac68c0962db836543))
* **entity,flags,rpc:** Add filter lifting, HTTP request population, and UI parameter roles ([0023d43](https://github.com/flanksource/clicky/commit/0023d438d1e625e5f280a1860fc763828be28120))
* **formatters:** Add ClickyDocument and ClickyText for direct Textable serialization ([db93e8d](https://github.com/flanksource/clicky/commit/db93e8d327b1bd59866e3f19ec6c2831103bb87f))


### 🐛 Bug Fixes

* **task:** enforce group concurrency at dequeue time instead of task body ([006f3b4](https://github.com/flanksource/clicky/commit/006f3b40034e8238c8dbd42128b470ec6791dcca))

## [1.21.10](https://github.com/flanksource/clicky/compare/v1.21.9...v1.21.10) (2026-05-27)


### ♻️ Code Refactoring

* **api,mcp,rpc:** extract helper functions and improve code organization ([1f17265](https://github.com/flanksource/clicky/commit/1f1726550675e064b311c96d2e45ef5d175d9162))


### ✨ Features

* **api,flags:** add unified diff rendering and grouped flag help output ([afc89b1](https://github.com/flanksource/clicky/commit/afc89b1de08c179be88ef529df5ac0cbfbc9c304))
* **api:** add stack trace parsing and rendering with source resolution ([074bef2](https://github.com/flanksource/clicky/commit/074bef245e1befcb58efeb80fdd440dc541269af))
* **entity:** add hidden _id column to wrapped entities ([e0ba5da](https://github.com/flanksource/clicky/commit/e0ba5daee66acb514fcef7f6516194eb9a618e7d))
* **entity:** add WithOptionalID for actions with flag-supplied targets ([85849dc](https://github.com/flanksource/clicky/commit/85849dc85f27ea5ce1f339127b91eab67edf8550))
* **examples:** Add syntax highlighting and stack trace rendering support ([0ef7feb](https://github.com/flanksource/clicky/commit/0ef7febd3496051526ef530e3de3ac2ee0fdd845))
* **mcp:** Add fluent Builder API and discover-tools MCP tool ([2f9a4da](https://github.com/flanksource/clicky/commit/2f9a4dae40bd66e0fae9a1ada1d10598d20dc7cc))
* **mcp:** add SSE transport, tools command, and multi-client install support ([693f311](https://github.com/flanksource/clicky/commit/693f311ebcb748cc860c1e9c17176391a6e5cb0d))
* **rpc,cobra:** Support rich error rendering and configurable converter paths ([98a40f9](https://github.com/flanksource/clicky/commit/98a40f944b18a07c837c590968df208e3cc9222a))


### 🐛 Bug Fixes

* **task:** treat StatusWarning as terminal state to freeze endTime ([8634866](https://github.com/flanksource/clicky/commit/86348660f8aebf4125d8d5991551a863f32cb950))


### 📚 Documentation

* **readme:** simplify and restructure README for clarity and brevity ([3ef7aba](https://github.com/flanksource/clicky/commit/3ef7aba737d9bbf6bf478c511dc99373042bfc8f))


### 📦 Build System

* **makefile:** add git commit and build date to binary via ldflags ([d9a3536](https://github.com/flanksource/clicky/commit/d9a3536359edfdf5e3e99e65e38cc38e8975afe0))


### 🔧 Maintenance

* **deps:** clean up unused dependencies in go.sum ([93dfe02](https://github.com/flanksource/clicky/commit/93dfe023c858cd3e78b29d00f2ce781d9cdaef28))
* **deps:** update dependencies to latest versions ([63987c5](https://github.com/flanksource/clicky/commit/63987c5b8b34375d791cb6da199585c551184827))
* **deps:** update dependencies to latest versions ([4999a86](https://github.com/flanksource/clicky/commit/4999a86655f35ef73e1aa79e0a5bcecd20669e9d))
* **deps:** update Go dependencies in examples module ([73dd9f9](https://github.com/flanksource/clicky/commit/73dd9f9bfb71f860c9401f6b9c7920767d8f6b5d))

## [1.21.9](https://github.com/flanksource/clicky/compare/v1.21.8...v1.21.9) (2026-05-12)


### ♻️ Code Refactoring

* **api,mcp,rpc:** extract helper functions and improve code organization ([f85ce24](https://github.com/flanksource/clicky/commit/f85ce248eb44808aaf0590856c96c1233ff83a03))
* **api:** simplify switch statement in parseJavaFrame ([fc8746a](https://github.com/flanksource/clicky/commit/fc8746a3dc8495f07d1d6025fb30f54aec7d34a0))


### ✨ Features

* **api,flags:** add unified diff rendering and grouped flag help output ([06e98ad](https://github.com/flanksource/clicky/commit/06e98adcbdfe4c4bced613d1bc3c92cc7d15d9b9))
* **api:** add stack trace parsing and rendering with source resolution ([a76d8ec](https://github.com/flanksource/clicky/commit/a76d8ecbecef68009b4d7ee34d93da6d0d1e119f))
* **api:** Add structured HTML rendering for stack traces with syntax highlighting ([3e6e880](https://github.com/flanksource/clicky/commit/3e6e880e9c3895332fbd5c8bb0c94eb158c161d1))
* **entity:** add hidden _id column to wrapped entities ([43aca2c](https://github.com/flanksource/clicky/commit/43aca2c10e03c08bab6db98ea84362f396b6b363))
* **examples:** Add syntax highlighting and stack trace rendering support ([cd1558a](https://github.com/flanksource/clicky/commit/cd1558abad79dd467471a1ef10cb900541fabe7e))
* **formatters:** add StackTrace rendering support to HTML formatters ([6beea3b](https://github.com/flanksource/clicky/commit/6beea3b4a1e20086316b40593595203a762391b1))
* **mcp:** Add fluent Builder API and discover-tools MCP tool ([9cb5cb9](https://github.com/flanksource/clicky/commit/9cb5cb964e37147e2d22bccff68c89c1e4997ad9))
* **mcp:** add SSE transport, tools command, and multi-client install support ([9345367](https://github.com/flanksource/clicky/commit/9345367ddf29b9d0bbf1eb60da3f9add27493df3))


### 📦 Build System

* **makefile:** add git commit and build date to binary via ldflags ([04f17df](https://github.com/flanksource/clicky/commit/04f17df0c43337c9d09ea29de792ed1a682788b4))


### 🔧 Maintenance

* **deps:** clean up unused dependencies in go.sum ([6b74c36](https://github.com/flanksource/clicky/commit/6b74c36624f1c7679fb03f40d9770840c47f9856))

## [1.21.8](https://github.com/flanksource/clicky/compare/v1.21.7...v1.21.8) (2026-05-01)


### ⚠ BREAKING CHANGES

* Entity type signature changed from Entity[T, ListOpts] to Entity[T, ListOpts, R]. Action and BulkAction are now factory functions returning interface types; update action definitions to use Action(), ActionWithFlags(), BulkAction(), BulkFilterAction(), or BulkActionWithFilter().

* chore(gitignore): ignore entity webapp dist directory

Add examples/enitity/webapp/dist/ to gitignore to prevent build artifacts from being tracked in version control.

* feat(entity): Add explicit HTTP method override for entity operations

Add optional Method field to EntityOperation and ActionInfo to allow explicit HTTP method specification for generated RPC/OpenAPI routes. This enables actions to override the default inferred HTTP method (e.g., using GET for a records action instead of the inferred POST).

The Method field is propagated through the command annotation system and checked during HTTP method inference in the RPC converter, allowing fine-grained control over operation semantics.

Includes test coverage for entity actions with explicit GET method using nested entity paths.

* feat(task): Add stderr gating to prevent corruption during interactive render

Introduce IsInteractiveRenderActive() to check if the task renderer owns the TTY, and GatedStderr() writer that silently drops writes when interactive rendering is active. This allows callers that emit to stderr (e.g., loggers, debug prints) to avoid corrupting the renderer's in-place frame without requiring explicit coordination.

The gating is stateless and rechecks ownership per write, enabling writers captured before rendering starts to still gate correctly when the renderer later acquires the TTY.

* feat(api): add stack trace parsing and rendering with source resolution

Introduce comprehensive stack trace support with language-agnostic parsing, styled rendering, and extensible source resolution. Adds StackTrace, StackFrame types and ParseJavaStackTrace parser to api package, with public convenience wrappers in clicky root. Includes support for frame filtering, context lines, and max frame truncation.

Also adds MultiFilter type for comma-separated flag values with include/exclude semantics, and fixes field value assignment to support type conversion in flags package.

Updates cache debug output to use task.GatedStderr() instead of os.Stderr to prevent log lines from breaking interactive renderer frame accounting. Fixes recursive struct handling in OpenAPI schema generation by tracking visited types.

Refs: stack trace rendering, source context, Java exception parsing

* docs(api): replace example package names with generic examples

Update documentation and test examples to use generic package names (com.example.admin) instead of specific internal package names (com.adminserver). This makes the codebase more suitable for public documentation and examples without exposing internal naming conventions.

### ✨ Features

* add entity response types, task stderr gating, and stack trace support ([#101](https://github.com/flanksource/clicky/issues/101)) ([0a2d157](https://github.com/flanksource/clicky/commit/0a2d157cba2928224e80588631d404b67b4e81ab))

## [1.21.7](https://github.com/flanksource/clicky/compare/v1.21.6...v1.21.7) (2026-04-30)


### ✨ Features

* support windows ([bffc0d9](https://github.com/flanksource/clicky/commit/bffc0d9d619ab2911ebd1c386b6ea93362ef0ce6))

## [1.21.6](https://github.com/flanksource/clicky/compare/v1.21.5...v1.21.6) (2026-04-27)


### ⚠ BREAKING CHANGES

* **formatters:** FormatWithOptions now delegates to FormatWithContext internally; custom FormatManager implementations should override FormatWithContext instead.
* **api,ui,example:** stackWindowOpts now uses From and To time.Time fields instead of UpdatedSince and Window duration fields. Content-Type header for clicky-json format changed from application/clicky+json to application/json+clicky.

Refs: entity demo tests, example app routing, API response handling
* **formatters:** The Content-Type header for clicky JSON responses is now application/json+clicky instead of application/clicky+json. Clients should update their expectations, though the legacy format is still accepted in Accept headers.

### ✨ Features

* **api,rpc,entity:** Add Link types and entity operation annotations for OpenAPI ([3bf7d81](https://github.com/flanksource/clicky/commit/3bf7d81eaa910c0a9f80fda5b42da79fdb2e08b8))
* **api,ui,example:** add time range filtering and entity detail page navigation ([0b78123](https://github.com/flanksource/clicky/commit/0b7812316e2917fd60872afb763315fee1babc97))
* **api:** add LabelBadge type for two-part pill badges ([8874efb](https://github.com/flanksource/clicky/commit/8874efba4b9b327e0c89b8633211ff376ad1a530))
* **entity-example:** add link and linkcommand examples page with deep-linking support ([d6d89b6](https://github.com/flanksource/clicky/commit/d6d89b6db93ff2ccf2e523e56de9405595932be3))
* **entity:** Add filter lookup support for dynamic filter metadata and completions ([168e9b2](https://github.com/flanksource/clicky/commit/168e9b2f6b679c7d58b366d322eb86ee16713ff5))
* **formatters:** add format callbacks and link/command support ([b19923a](https://github.com/flanksource/clicky/commit/b19923a7baf1af3c375b79293a0468956463cd76))
* **formatters:** standardize clicky JSON content type to application/json+clicky ([a899d50](https://github.com/flanksource/clicky/commit/a899d50a0a8d3e2b0637ffa600edaca1e7cd23f8))
* **prompt:** Add interactive prompt APIs with select, multi-select, and text input ([ad6a55d](https://github.com/flanksource/clicky/commit/ad6a55dcd247fbd44a3759bb9f45ba0ebc0b1f72))
* **task,examples:** Add interactive prompt support and task terminal ownership management ([5218f72](https://github.com/flanksource/clicky/commit/5218f72d2bae065ea4b9d9a106f22ba1f04def54))
* **task,lint,log:** add output capture and log serialization for clean task rendering ([6dc5e67](https://github.com/flanksource/clicky/commit/6dc5e67b80a661e04c5e4584c0d888bf75d78ee2))
* **task:** improve task rendering with priority-based ordering and collapsing ([aaa5043](https://github.com/flanksource/clicky/commit/aaa5043d1e1db625199f73a1eacbadd40dd86965))


### 🐛 Bug Fixes

* address PR [#99](https://github.com/flanksource/clicky/issues/99) test failures and CodeRabbit review ([db4f748](https://github.com/flanksource/clicky/commit/db4f748f776ad092d0d2e8bc9f0e74ac7d81dc70))


### 🔧 Maintenance

* **examples:** remove legacy example files and update dependencies ([190395f](https://github.com/flanksource/clicky/commit/190395f744be83e39cb53db0970f355a3eb3abd1))

## [1.21.5](https://github.com/flanksource/clicky/compare/v1.21.4...v1.21.5) (2026-04-21)


### ✨ Features

* **exec:** add atomic process tree termination with WithProcessGroup and KillTree ([0537211](https://github.com/flanksource/clicky/commit/05372118ae07217102601836cc27025e01d0e96b))
* **formatters,entity,rpc:** add clicky-json format and typed flags support for entities ([e541744](https://github.com/flanksource/clicky/commit/e54174494b77dcab166a25a13e94313029ba195d))


### 🐛 Bug Fixes

* **cobra_command:** track args field separately from flag lookup to support dual flag+args ([bdc4049](https://github.com/flanksource/clicky/commit/bdc40499ef85111de1ab923a28335b4056b6aeea))

## [1.21.4](https://github.com/flanksource/clicky/compare/v1.21.3...v1.21.4) (2026-04-14)


### 🐛 Bug Fixes

* **api,text:** Fix language normalization and style preservation in code blocks Improve code block rendering and text styling: ([#97](https://github.com/flanksource/clicky/issues/97)) ([4204ea9](https://github.com/flanksource/clicky/commit/4204ea90458d8376ecfdfc7c787a9d0352460e52))

## [1.21.3](https://github.com/flanksource/clicky/compare/v1.21.2...v1.21.3) (2026-04-13)


### 🐛 Bug Fixes

* **ci:** use semantic-release-action so step outputs are populated ([ed488d3](https://github.com/flanksource/clicky/commit/ed488d3b3202e83fbcc9660221932c46994a0be9))

## [1.21.2](https://github.com/flanksource/clicky/compare/v1.21.1...v1.21.2) (2026-04-13)


### ♻️ Code Refactoring

* **api:** simplify html rendering with fmt.fprintf and static html provider ([f89e0ef](https://github.com/flanksource/clicky/commit/f89e0efcec2dc6a32454555aaf0e749838dac91c))


### ✨ Features

* **api:** add admin entity subcommands and entity id wrapping ([1cb238c](https://github.com/flanksource/clicky/commit/1cb238c56380804b54a33c83570795a9a8a20c16))
* **api:** add entity parent nesting and command aliases support ([664876f](https://github.com/flanksource/clicky/commit/664876f36aa12c4b04862fef8c4b94de68c53100))
* **ci:** upgrade go to 1.26 and add task-ui frontend builds ([fb0818d](https://github.com/flanksource/clicky/commit/fb0818d92a94674b14f17c535f78ed8f265b0e7f))
* **flags:** add support for column selectors in CSV and Excel files ([94692d4](https://github.com/flanksource/clicky/commit/94692d406d98cf06009bd138f546b6b652c2cef9))
* **format:** support multiple output sinks with format=file syntax ([97b69e9](https://github.com/flanksource/clicky/commit/97b69e97adb7d17d89e7b5d5b0e23ffa244994ee))
* **formatters:** add html-react formatter with embedded react component ([75373a6](https://github.com/flanksource/clicky/commit/75373a6059b4f16a5b29ff12d7a327ca9f73f65f))
* **lint:** add clickylint static analyzer for api.Text usage patterns ([1fa3ab2](https://github.com/flanksource/clicky/commit/1fa3ab23766554170acd4b7ee684f55e3a307d0b))
* **mcp:** add install command and improve stdio handling ([c39ccce](https://github.com/flanksource/clicky/commit/c39cccee7cfb4e09e1fefef5b131e0e4ddb4b4a8))
* **ui:** add task progress web ui with sse streaming and json api ([29720c1](https://github.com/flanksource/clicky/commit/29720c101adb69865809939faeea9dcd62725c18))


### 🐛 Bug Fixes

* address PR [#94](https://github.com/flanksource/clicky/issues/94) review feedback and lint failures ([3865926](https://github.com/flanksource/clicky/commit/386592613d59dc791dc5e840452918b138eef41b))
* restore lint testdata stub dropped during rebase ([644b7df](https://github.com/flanksource/clicky/commit/644b7df03f36829efda907a6331eeccbd6e5f3d0))
* **api:** prevent panic when all table columns are filtered out ([5eea6b5](https://github.com/flanksource/clicky/commit/5eea6b5151f29752f8aa351517869cb02eeff74e))
* **ci:** add gitignore negations for task-ui and golangci config ([ed1a06f](https://github.com/flanksource/clicky/commit/ed1a06f6a89e488399e9e6c89c905e1b71e21e09))
* **ci:** trigger release directly on push to main ([704a544](https://github.com/flanksource/clicky/commit/704a5442de7e147a00597564c6a9f767bc7bad1e))
* **rpc:** improve command executor robustness and path sanitization ([e9b776a](https://github.com/flanksource/clicky/commit/e9b776a2c0d6405de9a5f23ca92195d6e0bf01e4))

## [1.21.1](https://github.com/flanksource/clicky/compare/v1.21.0...v1.21.1) (2026-03-26)


### ⚠ BREAKING CHANGES

* **api:** add custom actions and bulk actions to entities

### ✨ Features

* **api:** add custom actions and bulk actions to entities ([6f06836](https://github.com/flanksource/clicky/commit/6f068367c8909dd3d9e2033238ef406cf3798cc6))
* **rpc:** add structured data support to rpc operations via direct function invocation ([109c950](https://github.com/flanksource/clicky/commit/109c9504f51365eb66a821ae1c7b67d06a82baa9))


### 🔧 Maintenance

* bump version ([879e50c](https://github.com/flanksource/clicky/commit/879e50c896c6c4d9fc9d1ce4e145f017e7c49f0e))
* **build:** update release workflow and adjust versioning strategy ([c977f7e](https://github.com/flanksource/clicky/commit/c977f7e763460fa2ab8e6060b13f35cd1b1bbb5c))
* **release:** 2.1.0 [skip ci] ([eb59755](https://github.com/flanksource/clicky/commit/eb59755d7f9bf86635750f11be2448d8b3a582a0))

## [2.1.0](https://github.com/flanksource/clicky/compare/v2.0.0...v2.1.0) (2026-03-26)


### ✨ Features

* **rpc:** add structured data support to rpc operations via direct function invocation ([109c950](https://github.com/flanksource/clicky/commit/109c9504f51365eb66a821ae1c7b67d06a82baa9))

## [2.0.0](https://github.com/flanksource/clicky/compare/v1.20.0...v2.0.0) (2026-03-26)


### ⚠ BREAKING CHANGES

* **rpc:** extract route registration into separate method

### ♻️ Code Refactoring

* **rpc:** extract route registration into separate method ([5cf76c5](https://github.com/flanksource/clicky/commit/5cf76c5d09db34c31c6f6c96715b83c436b0424e))


### ✅ Tests

* **uber_demo:** add gavel fixture for --no-progress ANSI layout tests ([0f30bef](https://github.com/flanksource/clicky/commit/0f30befd4e7beedd8ac74c076a7f3544eb12b270))


### 🐛 Bug Fixes

* **tailwind:** fix text truncation to respect max width including ellipsis ([438bb96](https://github.com/flanksource/clicky/commit/438bb9626656d413b62cb468ff320358b6ee1aac))
* **task:** defer render loop start until first task is enqueued ([54b35fc](https://github.com/flanksource/clicky/commit/54b35fce7d45423e62d999bb7c412dce7a8d16e9))
* **task:** prevent deadlock when FailedWithError result is rendered ([7ad88b8](https://github.com/flanksource/clicky/commit/7ad88b8afe97e51b02f1168eb0bbe940e32180c7))

## [1.20.0](https://github.com/flanksource/clicky/compare/v1.19.0...v1.20.0) (2026-03-26)


### ✨ Features

* collapse completed tasks when they exceed terminal height ([4f9d07a](https://github.com/flanksource/clicky/commit/4f9d07a738e8917072cf9926fa360b21b5a8a67e))
* **tailwind:** add headtail truncation mode for showing first and last n lines ([7bde52f](https://github.com/flanksource/clicky/commit/7bde52fd68e1a125edc22d6ebeeb77f91456ee2e))
* **tailwind:** support terminal-relative dimensions in arbitrary classes ([cf65008](https://github.com/flanksource/clicky/commit/cf650089c2b1624c64636bfc073bee2296e61a41))


### 🐛 Bug Fixes

* html rendering of list of tables ([d0267d6](https://github.com/flanksource/clicky/commit/d0267d6ddc4b7dc59229f86eaa852ce6563f4b13))
* **api:** resolve no-color flag from environment variables ([027bdec](https://github.com/flanksource/clicky/commit/027bdecf3cbe7ada35c86adc9b93fb412cb0fcb2))


### 🔧 Maintenance

* go mod tidy ([1025b96](https://github.com/flanksource/clicky/commit/1025b96e5dccec72379224ef69a9ac8baa3b7199))
* reduce logging verbosity ([54badfc](https://github.com/flanksource/clicky/commit/54badfc237d96ab0e19eea3e8976529da0fa8cb6))
* use flankbot for semantic release ([93acf07](https://github.com/flanksource/clicky/commit/93acf074b276cc69c7e47b72522e2b0336c9d16d))

## [1.19.0](https://github.com/flanksource/clicky/compare/v1.18.1...v1.19.0) (2026-03-06)


### ✨ Features

* add support for collapsible row detail in table ([#85](https://github.com/flanksource/clicky/issues/85)) ([4b57695](https://github.com/flanksource/clicky/commit/4b5769553343b155c9f773432cce4c398cae47ab))

## [1.18.0](https://github.com/flanksource/clicky/compare/v1.17.0...v1.18.0) (2026-02-25)


### ♻️ Code Refactoring

* **task:** split manager.go and improve terminal safety ([a5d87ef](https://github.com/flanksource/clicky/commit/a5d87efeb0744072012a9de59b2a5ef8d9829f7e))


### ✨ Features

* **formatters:** add static HTML rendering for PDF output ([030d916](https://github.com/flanksource/clicky/commit/030d91654b772f1bf607ddfa679ecfdd7e12cf06))


### 🐛 Bug Fixes

* nil handling in pretty printers ([ec8f949](https://github.com/flanksource/clicky/commit/ec8f949ba6e3ab9924f1d19b5b25ccbf4b25c08e))


### 🔧 Maintenance

* go mod tidy ([c4206b6](https://github.com/flanksource/clicky/commit/c4206b6301546858d106a8e5c6122cbf6d2044df))

## [1.17.0](https://github.com/flanksource/clicky/compare/v1.16.2...v1.17.0) (2026-02-23)


### ♻️ Code Refactoring

* **task:** split manager.go and improve terminal safety ([ae606e9](https://github.com/flanksource/clicky/commit/ae606e9e821bd39c571ecf3fc8051d9e677235ee))


### ✨ Features

* **formatters:** add static HTML rendering for PDF output ([e070766](https://github.com/flanksource/clicky/commit/e0707664c325e90d12513a45393155b9aec4fce9))

## [1.16.2](https://github.com/flanksource/clicky/compare/v1.16.1...v1.16.2) (2026-01-27)


### 🐛 Bug Fixes

* prevent terminal corruption on shutdown/panic ([9d68d9a](https://github.com/flanksource/clicky/commit/9d68d9a64a978a49e7658bb3052e02b848f552c7))

## [1.16.1](https://github.com/flanksource/clicky/compare/v1.16.0...v1.16.1) (2026-01-27)


### 🔧 Maintenance

* code review fixes ([#75](https://github.com/flanksource/clicky/issues/75)) ([71a719d](https://github.com/flanksource/clicky/commit/71a719dd01f6510493628c697c80af1146d0bc2a))

## [1.16.0](https://github.com/flanksource/clicky/compare/v1.15.0...v1.16.0) (2026-01-27)


### ♻️ Code Refactoring

* improve CSV/HTML formatter handling of empty data ([934112a](https://github.com/flanksource/clicky/commit/934112aa65293dd51fccb374734de661050ead23))
* migrate task rendering to bubbletea ([1dc0bcf](https://github.com/flanksource/clicky/commit/1dc0bcf669df18d824859d0bc83b6a1833414344))
* rename TableProvider.Rows() to Row() for clarity ([5cdd030](https://github.com/flanksource/clicky/commit/5cdd030d98a3bcecae6524d786b1f52ad15aedbf))


### ✨ Features

* export Column builder in aliases ([026e98b](https://github.com/flanksource/clicky/commit/026e98b312127a30a7735fc159b1241874dd8890))


### 🐛 Bug Fixes

* use TryTypedValue for generic interface handling in struct fields ([a32c670](https://github.com/flanksource/clicky/commit/a32c6703164bf848b1f9afa78eda18a8acbb370a))


### 🔧 Maintenance

* fix tests ([0e9262e](https://github.com/flanksource/clicky/commit/0e9262e4bc82eafac69682666bc3b85937033ba1))
* remove pdf  dependencies ([fd87d74](https://github.com/flanksource/clicky/commit/fd87d74d38c4fcd9fe73c1450ac628fd473e2b3e))
* update dependencies ([73048ac](https://github.com/flanksource/clicky/commit/73048ac135d4f9e874079764b9b7f8218dae7a12))

## [1.15.0](https://github.com/flanksource/clicky/compare/v1.14.0...v1.15.0) (2026-01-27)


### ✅ Tests

* cover slack semibold markdown ([d12a994](https://github.com/flanksource/clicky/commit/d12a9943badb9f78cb1d454b3217bdad05307300))


### ✨ Features

* add slack-specific markdown rendering ([d8e38b8](https://github.com/flanksource/clicky/commit/d8e38b8ac461fc3555619059a8847813679bdd60))


### 🐛 Bug Fixes

* drop unused slack cell helper ([d7e665f](https://github.com/flanksource/clicky/commit/d7e665f1c8bd02016aa25a7a825387ebcc0af7f1))
* render slack collapsed sections without html ([c72a117](https://github.com/flanksource/clicky/commit/c72a117ac0a1fef3ef44779c49aad1108a1e57df))

## [1.14.0](https://github.com/flanksource/clicky/compare/v1.13.0...v1.14.0) (2026-01-21)


### ✨ Features

* HTML for email and fix html tables ([#66](https://github.com/flanksource/clicky/issues/66)) ([d2f670c](https://github.com/flanksource/clicky/commit/d2f670c46c2fa0580c842a11a9543e61d43878ce))

## [1.13.0](https://github.com/flanksource/clicky/compare/v1.12.2...v1.13.0) (2026-01-20)


### ✨ Features

* slack blocks formatter ([#59](https://github.com/flanksource/clicky/issues/59)) ([b4203d3](https://github.com/flanksource/clicky/commit/b4203d3762b0527a10958478dda533b1385d29e9))

## [1.12.2](https://github.com/flanksource/clicky/compare/v1.12.1...v1.12.2) (2026-01-19)


### ♻️ Code Refactoring

* simplify JSON/YAML formatters to use FormatValue directly ([92bc053](https://github.com/flanksource/clicky/commit/92bc0530ae1ce828c1a529c83bea79b4fcadacf8))
* simplify Tree/Table options from *bool to bool ([fce309e](https://github.com/flanksource/clicky/commit/fce309ee2815911d802c066ffc5faf2962e8cc84))


### ✅ Tests

* update formatter tests for date format and options changes ([d451b2e](https://github.com/flanksource/clicky/commit/d451b2ed3f4b055ece46fea8c34e489c5b07a5f9))


### 🐛 Bug Fixes

* deduplicate styles in Text.Styles() method ([665d616](https://github.com/flanksource/clicky/commit/665d616c4fe4fc7e271f73f7bbe40b5658e73b91))
* log command errors before returning ([0c2e57d](https://github.com/flanksource/clicky/commit/0c2e57da7c0e9ae920eaf24e5b2e1661b2f13d10))


### 🔧 Maintenance

* change exec run log level from debug to trace ([1df5f89](https://github.com/flanksource/clicky/commit/1df5f890bc5c83e5ce08c24cacae775e47af3485))
* misc formatting fixes ([47bb515](https://github.com/flanksource/clicky/commit/47bb515aa09057e96568f935dda61810e3a67906))

## [1.12.1](https://github.com/flanksource/clicky/compare/v1.12.0...v1.12.1) (2026-01-16)


### 🐛 Bug Fixes

* lint ([#60](https://github.com/flanksource/clicky/issues/60)) ([66cff66](https://github.com/flanksource/clicky/commit/66cff6648cb5ae36492eb38c688c1f8086f805af))

## [1.12.0](https://github.com/flanksource/clicky/compare/v1.11.2...v1.12.0) (2025-12-09)


### ♻️ Code Refactoring

* extract inline CSS/JS to go:embed files and use Iconify chevrons ([70fcd7f](https://github.com/flanksource/clicky/commit/70fcd7f1869641144b1ecfedb6e6346623df6887))
* remove Formatted(), Plain(), Markdown(), HTML() from FieldValue ([6b5b78c](https://github.com/flanksource/clicky/commit/6b5b78ce18fb43b4bb7651cd0359a3173809f0e5))
* **html:** use Value() + type switch instead of GetValue() ([61457b2](https://github.com/flanksource/clicky/commit/61457b2c8892412cc920d34149dff0b6baf6f680))
* **table:** make lipgloss table Textable-aware ([eb55aee](https://github.com/flanksource/clicky/commit/eb55aee4aca9018299fcae8cd9f1008f95c23014))


### ✨ Features

* batch timeouts ([f853be4](https://github.com/flanksource/clicky/commit/f853be4d73eabe83b1f1c402e36fb95f57f0c338))
* default to truncate-suffix when constraints specified ([df111ba](https://github.com/flanksource/clicky/commit/df111ba7d3245c776502e1f1e6e6eb5ca5b30636))
* implement nested table rendering in HTML with compact support ([99c8918](https://github.com/flanksource/clicky/commit/99c89182910dfb77e449e1a4a1c006d39dfbed61))
* migrate terminal tables to lipgloss (hybrid approach) ([26126c5](https://github.com/flanksource/clicky/commit/26126c57e852bed5664509c3a117acc831b915bd))
* migrate tree rendering to lipgloss ([ceadc44](https://github.com/flanksource/clicky/commit/ceadc44f4ce7fc1ff5c21e5baaa6877ea01e4765))
* proper rendering of api.Text and Textable in all formatters ([5f84f59](https://github.com/flanksource/clicky/commit/5f84f592407eecedb7fe47573f7aaed9e17960c8))
* refactor formatters to use FieldValue.Text and PrettyData.Pretty() ([0f7019d](https://github.com/flanksource/clicky/commit/0f7019d83be5937d9faa4de51cc1fa49cf7cb157))
* **ai:** add LLM agent adapter with structured output support ([9968fd2](https://github.com/flanksource/clicky/commit/9968fd27b4b23ffddd4034b52dfdd6b458dc837a))
* **formatters:** make TextTable self-contained with column schema ([40f0ee0](https://github.com/flanksource/clicky/commit/40f0ee0848c6371ea02022fe26b1228280b4f89c))


### 🐛 Bug Fixes

* add missing types and methods for compilation ([b331825](https://github.com/flanksource/clicky/commit/b331825e2964c76e86b68f3e1ed2001929973485))
* align tree chevrons to top ([c484df3](https://github.com/flanksource/clicky/commit/c484df31fa839a089963187a057a2911b605674c))
* args handling in clicky.AddCommand ([557b30a](https://github.com/flanksource/clicky/commit/557b30a79e36f43f331174614cc94dc41277cbbe))
* batch race conditions ([1bc0c88](https://github.com/flanksource/clicky/commit/1bc0c883b4c20f8312b5369669acbc50a000563c))
* check PrettyRow interface before extracting struct fields ([e5dca82](https://github.com/flanksource/clicky/commit/e5dca828bcb1b90952a50a6fa4043686dd18d4f1))
* duplicate rendering of logs with --no-progress ([26623d3](https://github.com/flanksource/clicky/commit/26623d3055d099e7a4649d0247a8bcd430637a44))
* formatting improvements ([99dd47d](https://github.com/flanksource/clicky/commit/99dd47d7503e46f3b0e1f1fa7057980e06bab14c))
* formatting tests ([0e6838e](https://github.com/flanksource/clicky/commit/0e6838e94c1a97ecf3db32cf934a3931390bd7f7))
* improve HTML tree rendering with embedded assets ([f049d29](https://github.com/flanksource/clicky/commit/f049d29dc9e9dc48522014ae234aaa31b7c5d39c))
* parse max-w unit suffixes (ch, px, rem, em) ([6870944](https://github.com/flanksource/clicky/commit/6870944dbae4a20cd14394f8b830e6810953a582))
* prevent truncate class from overwriting explicit max-w ([b671c3f](https://github.com/flanksource/clicky/commit/b671c3f3c1507eca557e928ba357376f864047ad))
* reset capture output when reusing exec wrappers ([77faf25](https://github.com/flanksource/clicky/commit/77faf25d7936f9adf57b2d01980a639cda741ea4))
* restore table rendering temporarily, fix ANSI test checks ([30fd9bc](https://github.com/flanksource/clicky/commit/30fd9bce79028a811fde4040f0382f336019970c))
* update test to handle Textable interface properly ([045a4fc](https://github.com/flanksource/clicky/commit/045a4fcdbc24b1f6b38ac0debad4f816544de09e))
* use interface assertion in NewTypedValue for nested trees ([d906ab6](https://github.com/flanksource/clicky/commit/d906ab686474dc9fa531314a644f43cd2bd1726a))
* wrap string values in Text for TextList compatibility ([a857c2f](https://github.com/flanksource/clicky/commit/a857c2ff32a836d127059ea61b07ff7748debba0))
* **html:** add fallback to data.Tree for tree fields ([999e3ca](https://github.com/flanksource/clicky/commit/999e3ca9f999aca4fbb0aa72f6ca035260053177))
* **task:** prevent hanging and terminal corruption in task manager ([c1f993b](https://github.com/flanksource/clicky/commit/c1f993b9a483aac3809c7d8f55521d64d8d928c4))
* **task:** resolve data races in Task and Manager ([ce82b46](https://github.com/flanksource/clicky/commit/ce82b4695b7bc8e447a6026fc21131b8c7f54785))
* **task:** resolve race conditions in batch.go ([107c875](https://github.com/flanksource/clicky/commit/107c875b488d0d2e8f8a80966cc21bcbf78ef495))


### 🔧 Maintenance

* ai refactorings ([049a4a9](https://github.com/flanksource/clicky/commit/049a4a918a7d6dd28a47ad8584e1543af7228544))
* fix lint errors ([f26f0f1](https://github.com/flanksource/clicky/commit/f26f0f186ed3a62b359960b413bcbdeb09f7e14b))
* fix nested html table output ([2e2a00d](https://github.com/flanksource/clicky/commit/2e2a00db977d216b3be32661f4fb81103234cae3))
* fix nested html tree output ([9045fd9](https://github.com/flanksource/clicky/commit/9045fd9e7a7888372cbb71c61c167c3fee722f44))
* fix test errors ([9f16c6c](https://github.com/flanksource/clicky/commit/9f16c6c47d7dd0271836d8ab616c294a8e0c5d29))
* fix tests ([b28a08b](https://github.com/flanksource/clicky/commit/b28a08bea64d4e91d967827930f8edd744b2d5d4))
* html fixes ([d02e990](https://github.com/flanksource/clicky/commit/d02e990cc8f126aaa27e2e7dab18cbd8af43791f))
* html table fixes ([199e562](https://github.com/flanksource/clicky/commit/199e5629a5cbad617cdffa168a26fefeafa95aa7))
* misc ([77da16b](https://github.com/flanksource/clicky/commit/77da16bed8ce41ec513c16e51811ad308f22e6c2))
* refactor formatters package ([cefe685](https://github.com/flanksource/clicky/commit/cefe685ef1d52b934b5e9d166b54c192f4ee80c7))
* refactor formatters package ([d65dfb3](https://github.com/flanksource/clicky/commit/d65dfb349384dcc6121136a614a5443766bf5fb8))
* review fixes ([7fbbaae](https://github.com/flanksource/clicky/commit/7fbbaaec22794de0c2cf12824b2408aea317c1c2))
* switch tree/table printing back to lipgloss ([52fc4c8](https://github.com/flanksource/clicky/commit/52fc4c8948be721851f7e55276f266a6f38a1fe6))
* uber demo ([7d921d9](https://github.com/flanksource/clicky/commit/7d921d9dc5ec3b77d11c46dc6e0b9b41ebd0697e))
* update examples ([b7fea73](https://github.com/flanksource/clicky/commit/b7fea735f8e683b790aaad7639752cd8bba0ce01))

## [1.11.2](https://github.com/flanksource/clicky/compare/v1.11.1...v1.11.2) (2025-11-21)


### 🐛 Bug Fixes

* bump golangci ([389eb1c](https://github.com/flanksource/clicky/commit/389eb1c6225d29be50e593a52817766c98086617))

## [1.11.1](https://github.com/flanksource/clicky/compare/v1.11.0...v1.11.1) (2025-11-11)


### 🔧 Maintenance

* formatting auto fix ([978f444](https://github.com/flanksource/clicky/commit/978f4440ef601ec17462b535af061d8ded2248d5))

## [1.11.0](https://github.com/flanksource/clicky/compare/v1.10.0...v1.11.0) (2025-11-03)


### ✨ Features

* **ai:** support new Claude API response format ([124659a](https://github.com/flanksource/clicky/commit/124659ac6e93b39324294cddb77307a5c65f3eb4))


### 🔧 Maintenance

* ai fixes ([ea0451c](https://github.com/flanksource/clicky/commit/ea0451c613534d7367bad953f3328d3c313b9cb6))
* misc formatting improvements ([eb8f3fc](https://github.com/flanksource/clicky/commit/eb8f3fc62f16bfc831990d48c0f8b2856ac94955))

## [1.10.0](https://github.com/flanksource/clicky/compare/v1.9.0...v1.10.0) (2025-10-30)


### ♻️ Code Refactoring

* move line processor to text package ([84dadc3](https://github.com/flanksource/clicky/commit/84dadc35a8bd2d9eaddc765fe3c487289ae56173))


### ✅ Tests

* refactor built-in processors to use table-driven tests ([7289f08](https://github.com/flanksource/clicky/commit/7289f08c34fcfa97c59741f066d258890ccfcfd6))


### ✨ Features

* add filtering ([ac773e2](https://github.com/flanksource/clicky/commit/ac773e21c926ec50ea3581e001d789acd89d3a08))
* add line processor middleware framework ([e9eb174](https://github.com/flanksource/clicky/commit/e9eb1740fb077ec740b3c2c00d3969f7eee8ca43))
* add RedactValues processor for known secrets ([d4c812e](https://github.com/flanksource/clicky/commit/d4c812ee933b923bf0dc4d2e8e09dde604e4f1eb))
* exec logging ([45a6bf6](https://github.com/flanksource/clicky/commit/45a6bf6c4ff20a3c5950610007970e72a376b9d4))
* implement tokenizer for RedactSecrets with ANSI support ([01d0869](https://github.com/flanksource/clicky/commit/01d086944c682273f390799e7b3dc66fbc4111d7))
* preserve keys when redacting secrets ([19c9019](https://github.com/flanksource/clicky/commit/19c90195e2c379508e0899597d2bc1c00bea38df))


### 🐛 Bug Fixes

* apply CEL filters to table data in FormatWithOptions ([c4311ce](https://github.com/flanksource/clicky/commit/c4311ceac3897620c9484458f7ad884716986e98))
* correct DescribeTable usage in builtin tests ([2ea0e08](https://github.com/flanksource/clicky/commit/2ea0e080434e68bdfc22bbf83424c33a7372d468))
* ensure consistent alphabetical ordering of map keys in pretty printing ([0df3297](https://github.com/flanksource/clicky/commit/0df329788f7f56466dfe4a2c62468f8900de3548))
* improve command execution and exit code handling ([00e0ea5](https://github.com/flanksource/clicky/commit/00e0ea5db72d59a98f5ab35808693385b435d72d))
* resolve all remaining test failures in exec package ([6e6dcaf](https://github.com/flanksource/clicky/commit/6e6dcafdac16a1cccdc81c56e38a4bc04a1cbaae))


### 🔧 Maintenance

* test refactor ([cfba7a0](https://github.com/flanksource/clicky/commit/cfba7a05f08f432e405454badb71d84a0fb64daa))

## [1.9.0](https://github.com/flanksource/clicky/compare/v1.8.0...v1.9.0) (2025-10-27)


### ✨ Features

* new formatting helpers ([01b9ab0](https://github.com/flanksource/clicky/commit/01b9ab0c7d8e6fe10de256716acaace8c7f8620a))


### 🐛 Bug Fixes

* **formatters:** improve tree formatter error message with detailed info ([f6f425e](https://github.com/flanksource/clicky/commit/f6f425eb68fe0b1e51c3a6143a4fea163516a6f7))
* **formatters:** support slices of pointers in ToPrettyData ([51a3ecc](https://github.com/flanksource/clicky/commit/51a3ecc2a3cdcedbe3e8a91273dd1d83b4e37532))


### 🔧 Maintenance

* exec improvements ([19158a0](https://github.com/flanksource/clicky/commit/19158a0e5180b78c299371b0f78bfe71de5cc7a4))
* filter api ([f4b07bf](https://github.com/flanksource/clicky/commit/f4b07bfcd2c5c06e8d06ab26e6bd10e0fd845ce1))

## [1.8.0](https://github.com/flanksource/clicky/compare/v1.7.1...v1.8.0) (2025-10-24)


### ✨ Features

* **exec:** add verbosity-based stdout tee-ing to Process.Run ([3ba991e](https://github.com/flanksource/clicky/commit/3ba991e0fc0c5fd9b88ac6b8a44c9c2d191eacb3))
* **task:** add ClearLogs method to Task ([d3f4e23](https://github.com/flanksource/clicky/commit/d3f4e23b08346e3b5931249682f75dc66e96bdcb))


### 🔧 Maintenance

* misc fixes ([7b2616b](https://github.com/flanksource/clicky/commit/7b2616bf72e190452228061449fc46c4ae35eaa0))

## [1.7.1](https://github.com/flanksource/clicky/compare/v1.7.0...v1.7.1) (2025-10-16)


### 🔧 Maintenance

* fix docs ([38987fa](https://github.com/flanksource/clicky/commit/38987fa97e18ee48b34c00da8c003b2530db6c64))

### ✨ Features

* add clicky.CodeBlock/Map/Collapsed ([2fcda9a](https://github.com/flanksource/clicky/commit/2fcda9aef927b6a45fd1c2d84f738732fa90d68c))


### 🐛 Bug Fixes

* formatting issues ([a9cf960](https://github.com/flanksource/clicky/commit/a9cf9604eab846c447724e2b49493a9e0a5e3ebe))
* formatting issues ([df0fffe](https://github.com/flanksource/clicky/commit/df0fffe8db3c406073bee305e94ccc476c362aa0))


### 🔧 Maintenance

* add pdf build tag ([44117f5](https://github.com/flanksource/clicky/commit/44117f55e152d75a7b673de2917f2723dbb736cc))
* fix nested rendering ([9fec288](https://github.com/flanksource/clicky/commit/9fec2886491b426da8adc8d79528fd2debc90437))
* fix task rendering ([72a0aa6](https://github.com/flanksource/clicky/commit/72a0aa6bd971c80cdb79f30c2266f13ff786b4ce))
* fix test report ([f3e879a](https://github.com/flanksource/clicky/commit/f3e879ab60a35be17adce03a53cccd52ee0c49eb))

## [1.5.0](https://github.com/flanksource/clicky/compare/v1.4.0...v1.5.0) (2025-10-03)


### ✅ Tests

* add comprehensive uber-demo integration tests for all formats ([ce4da59](https://github.com/flanksource/clicky/commit/ce4da59ec233984ad33f20240d3ab8556213bb52))


### ✨ Features

* add array loading from file/stdin in args parser ([1d2a0a4](https://github.com/flanksource/clicky/commit/1d2a0a4f012456a49f8dd9927215a9bca7f432a2))
* add cobra command ([f98f64e](https://github.com/flanksource/clicky/commit/f98f64e1489a8dc7ab87adf3ba3b149e04dae8b8))


### 🐛 Bug Fixes

* populate TableOptions.Fields for HTML table rendering ([04b593a](https://github.com/flanksource/clicky/commit/04b593a68ea1a9dfa4197f767fcc3e0a9d1cdf80))
* preserve pretty tag formatting for nested structs in slices/maps ([ac064a5](https://github.com/flanksource/clicky/commit/ac064a52dd10de05ace67fc8bbedced517ad3aaa))

## [1.4.0](https://github.com/flanksource/clicky/compare/v1.3.1...v1.4.0) (2025-09-30)


### ✨ Features

* add api.Code with chroma syntax highlighting ([978e839](https://github.com/flanksource/clicky/commit/978e83955532cb54e91a87505bee0faed58865eb))


### 🐛 Bug Fixes

* default yaml to use json tags ([5c11b99](https://github.com/flanksource/clicky/commit/5c11b993d29c19cf8ca3e57d57b82f65a50916fd))
* tree printing in html ([179551a](https://github.com/flanksource/clicky/commit/179551adaa04898ab2756532fedb08e255fde743))


### 🔧 Maintenance

* add icons ([a48d833](https://github.com/flanksource/clicky/commit/a48d8339afb8480b10f63123e2a9a10f03932eba))
* include iconify script ([9bcd758](https://github.com/flanksource/clicky/commit/9bcd758f85320603d5f820883dfd927e5405acf5))

## [1.3.1](https://github.com/flanksource/clicky/compare/v1.3.0...v1.3.1) (2025-09-29)


### 🔧 Maintenance

* release ([c7c7fe8](https://github.com/flanksource/clicky/commit/c7c7fe83a5a3ac395ee7387fdbd4346c5b3c56b3))

## [1.3.0](https://github.com/flanksource/clicky/compare/v1.2.1...v1.3.0) (2025-09-29)


### ✨ Features

* add echo middleware helper ([61fa1a4](https://github.com/flanksource/clicky/commit/61fa1a43872da3b695267b08a4d3539649d277fe))
* add type-safe unit conversions for PDF font metrics and margins ([dbb575c](https://github.com/flanksource/clicky/commit/dbb575cce577a5f45bd17ffe47060a2ba2b6d88d))


### 🐛 Bug Fixes

* correct drawCellText method calls in table tests ([00983e5](https://github.com/flanksource/clicky/commit/00983e515744164b7ce7dbbbfd53689c4c21a933))
* pdf formatting fixes ([9fdce42](https://github.com/flanksource/clicky/commit/9fdce42ad45f9979fb93bcaf6c64b64df3701e4f))


### 🔧 Maintenance

* misc ([805185c](https://github.com/flanksource/clicky/commit/805185ccf2bed46fc3976092f8addfff6717ef69))
* tailwind improvement ([b177b9b](https://github.com/flanksource/clicky/commit/b177b9b327a87a93cb8f801d385374b635fcc000))
* tailwind style improvements ([7c6876a](https://github.com/flanksource/clicky/commit/7c6876a0e60ea2bf86047b5821aa5e2495c7908d))
* update tailwind tests ([c4304af](https://github.com/flanksource/clicky/commit/c4304afbda6ec3dbfe6a8c4b354d752b9854a2e0))

## [1.2.1](https://github.com/flanksource/clicky/compare/v1.2.0...v1.2.1) (2025-09-28)


### 🐛 Bug Fixes

* task failures ([177a1e6](https://github.com/flanksource/clicky/commit/177a1e69df137833e3699db9ee5caadd0c506312))

## [1.2.0](https://github.com/flanksource/clicky/compare/v1.1.1...v1.2.0) (2025-09-22)


### ♻️ Code Refactoring

* extract MCP logic into generic RPC package ([03950c3](https://github.com/flanksource/clicky/commit/03950c32db238c89d1114b099db44f096d06bcea))


### ✨ Features

* add args parser ([bca5105](https://github.com/flanksource/clicky/commit/bca510575faec7053b3dc17f10c9cbec215b1b46))
* add openapi server ([b4cfac3](https://github.com/flanksource/clicky/commit/b4cfac3cf426c3ddb0119a12af7df757fa163035))
* add openapi server ([879897b](https://github.com/flanksource/clicky/commit/879897b7b6bc5913d99b7286b945ea77b1d3ef51))


### 🐛 Bug Fixes

* linting issues ([b622469](https://github.com/flanksource/clicky/commit/b622469f80cd046c0b075ec38e601c674ed4644a))


### 🔧 Maintenance

* add openapi html ([a3833bc](https://github.com/flanksource/clicky/commit/a3833bc93fa704abd78cd082c00d8cb381627a61))
* bump golangci-lint action ([e8ce19d](https://github.com/flanksource/clicky/commit/e8ce19db9d797801ef3a1fa940d810b8e3fbc7aa))
* go mod tidy ([3656a9f](https://github.com/flanksource/clicky/commit/3656a9f49f688572df3e42794f7f4a54b088aa86))
* update lint workflows ([b2b34fe](https://github.com/flanksource/clicky/commit/b2b34fe585a1b92125205b06917293f5d630eb81))

## [1.1.1](https://github.com/flanksource/clicky/compare/v1.1.0...v1.1.1) (2025-09-16)


### 🐛 Bug Fixes

* html table labels ([27cefa6](https://github.com/flanksource/clicky/commit/27cefa6317380ca48d111a318556c3dd9de7d348))


### 🔧 Maintenance

* fix race conditions ([91a4cc4](https://github.com/flanksource/clicky/commit/91a4cc406802fbe209f8854f2bd0a9c54157130e))
* fix tests ([8a61fe7](https://github.com/flanksource/clicky/commit/8a61fe783e53350f87875ebf03dab524790db903))
* pdf builder improvements ([39e9004](https://github.com/flanksource/clicky/commit/39e90044874181ec9a728c836cc60cf633a6cab6))

## [1.1.0](https://github.com/flanksource/clicky/compare/v1.0.0...v1.1.0) (2025-09-01)


### ✨ Features

* add task group concurrency ([a93beb0](https://github.com/flanksource/clicky/commit/a93beb036d272e07dac6fa21c55200be9f859c3f))


### 🔧 Maintenance

* fix formatting ([a4ca282](https://github.com/flanksource/clicky/commit/a4ca28264e1a703194500820ccea41cac5fe5a58))
* fix tests ([df8d060](https://github.com/flanksource/clicky/commit/df8d060d6a85086548292196872f00a10ec56abc))
* gofmt ([9ec904e](https://github.com/flanksource/clicky/commit/9ec904e67dd5685ed71383409313175828478681))
* make task manager private ([e6604e6](https://github.com/flanksource/clicky/commit/e6604e6169aa0801a76277740416cc39152c149a))

## 1.0.0 (2025-08-27)


### ♻️ Code Refactoring

* implement FormatManager interface and move struct tag parsing to shared parser ([6a5c4b6](https://github.com/flanksource/clicky/commit/6a5c4b618d9a13048a86cc4f8148c1a5173ef9e8))
* migrate PDF generation from fpdf to Maroto v2 ([ab91366](https://github.com/flanksource/clicky/commit/ab913664f111b0e3d3e8098dfe3f284d6a7e30a9))
* replace go-rod with go-playwright for PDF generation ([e0bbb34](https://github.com/flanksource/clicky/commit/e0bbb3473180b95e70523765edcf870bbd48159b))


### ✨ Features

* add --dump-schema flag for debugging formatting issues ([a1cda79](https://github.com/flanksource/clicky/commit/a1cda798d4da04b3019aebf3447ec35445bb00cc))
* add built-in task deduplication and dependency scanner with caching ([c8b1575](https://github.com/flanksource/clicky/commit/c8b157546c8a165f036d680ba274b87e8f1168a1))
* add semantic release workflow and fix GitHub CI/CD ([d064bd3](https://github.com/flanksource/clicky/commit/d064bd3dca222f8edf20f18853b3ba8321d39b36))
* implement api.ResolveStyles and comprehensive PDF widget system ([42add34](https://github.com/flanksource/clicky/commit/42add34a63583a7e90fff6f73870f12a2d92e856))
* implement PDF text extraction and error detection ([efd5ad6](https://github.com/flanksource/clicky/commit/efd5ad6a858a97cd166607c6d7275fd2293f7e52))
* integrate SVG conversion directly into Image widget ([414dbb3](https://github.com/flanksource/clicky/commit/414dbb34bf393670b747f0cecb89a8974d5ea681))


### 🐛 Bug Fixes

* apply --no-progress and --no-color flags correctly, use less aggressive screen clearing ([d739fc5](https://github.com/flanksource/clicky/commit/d739fc56acf4a382b5e1b14b966fe8212091447e))
* markdown ([aef12eb](https://github.com/flanksource/clicky/commit/aef12ebe342d3bca1f961bfdb979200a8c42ed2f))
* remove mutex lock from Duration() method to prevent deadlock ([61da792](https://github.com/flanksource/clicky/commit/61da792e071d7a54f812075e9acce2a605e3d039))
* simplify schema.go formatting to delegate to format manager ([fda030b](https://github.com/flanksource/clicky/commit/fda030bcad7a0bdb21aa0797065419ac7e41c4c4))


### 🔧 Maintenance

* build fixes ([ece7cb0](https://github.com/flanksource/clicky/commit/ece7cb028372c66f78f373186e5271b23aa0cc84))
