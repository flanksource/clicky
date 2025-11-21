# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
