# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
