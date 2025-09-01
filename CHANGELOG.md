# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
