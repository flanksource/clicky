# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
