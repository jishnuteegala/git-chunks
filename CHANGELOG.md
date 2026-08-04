# Changelog

## [0.4.0](https://github.com/jishnuteegala/git-chunks/compare/v0.3.0...v0.4.0) (2026-08-04)


### Features

* align changelog sections with shared template ([#32](https://github.com/jishnuteegala/git-chunks/issues/32)) ([28b2546](https://github.com/jishnuteegala/git-chunks/commit/28b25463b0c4c766d5d60f27c4dfb419a6db8330))
* honest dry-run output with pre-pack estimate labels and additive JSON fields ([#30](https://github.com/jishnuteegala/git-chunks/issues/30)) ([cf0725e](https://github.com/jishnuteegala/git-chunks/commit/cf0725e2adb6a483b41cb3b15f1d228166915fd2))
* randomised push spacing and live ETA ([#31](https://github.com/jishnuteegala/git-chunks/issues/31)) ([a51d501](https://github.com/jishnuteegala/git-chunks/commit/a51d501636550223770f5410776cad52b3a3d695))


### Documentation

* document winget local validation workflow ([#33](https://github.com/jishnuteegala/git-chunks/issues/33)) ([03bb56a](https://github.com/jishnuteegala/git-chunks/commit/03bb56a2a78be09ef7bf13b7951de8c7f786eaa6))


### Build System

* **deps:** bump actions/checkout from 7.0.0 to 7.0.1 ([#24](https://github.com/jishnuteegala/git-chunks/issues/24)) ([74a4c23](https://github.com/jishnuteegala/git-chunks/commit/74a4c23c768c48e1f58b8e991a68346ac38995cd))
* **deps:** bump actions/setup-go from 5.6.0 to 7.0.0 ([#23](https://github.com/jishnuteegala/git-chunks/issues/23)) ([b1e0cb2](https://github.com/jishnuteegala/git-chunks/commit/b1e0cb2c7b27fc7484aa9ce350ef931f04305b8f))
* **deps:** bump googleapis/release-please-action from 4.4.1 to 5.0.0 ([#22](https://github.com/jishnuteegala/git-chunks/issues/22)) ([85f40bf](https://github.com/jishnuteegala/git-chunks/commit/85f40bf819358210344b721ec1246c808b66e848))


### Continuous Integration

* validate conventional commit subjects via shared action ([#34](https://github.com/jishnuteegala/git-chunks/issues/34)) ([31bcd86](https://github.com/jishnuteegala/git-chunks/commit/31bcd867d4cdb3a73cefd85086119ab55f24b5c6))

## [0.3.0](https://github.com/jishnuteegala/git-chunks/compare/v0.2.0...v0.3.0) (2026-07-28)


### Continuous Integration

* authenticate changelog API and allow draft release recovery ([#17](https://github.com/jishnuteegala/git-chunks/issues/17)) ([6924e23](https://github.com/jishnuteegala/git-chunks/commit/6924e23ac9820b6eaa231f26e28eb630e219c343))
* bootstrap npm republish with a temporary token ([#20](https://github.com/jishnuteegala/git-chunks/issues/20)) ([3a82364](https://github.com/jishnuteegala/git-chunks/commit/3a82364cb439390cfe22b6c6fd458a78222a9ace))
* publish npm exclusively through OIDC trusted publishing ([#19](https://github.com/jishnuteegala/git-chunks/issues/19)) ([79d8e02](https://github.com/jishnuteegala/git-chunks/commit/79d8e02caea0582bf8158c8e30356003b95a7b9e))
* restore OIDC-only npm publishing after bootstrap ([#21](https://github.com/jishnuteegala/git-chunks/issues/21)) ([cfab520](https://github.com/jishnuteegala/git-chunks/commit/cfab520d0aa4a734b9428ff7d227769a57d3a669))

## [0.2.0](https://github.com/jishnuteegala/git-chunks/compare/v0.1.0...v0.2.0) (2026-07-27)


### Features

* add verified shell installer ([#15](https://github.com/jishnuteegala/git-chunks/issues/15)) ([b069014](https://github.com/jishnuteegala/git-chunks/commit/b0690144ae90bd6057214d94fe7b671685a90e17))
* publish AUR and Chocolatey packages ([#13](https://github.com/jishnuteegala/git-chunks/issues/13)) ([8174bfe](https://github.com/jishnuteegala/git-chunks/commit/8174bfe134ca3c8655c8343200b86e5f7299f075))


### Bug Fixes

* include maintenance changes in release notes ([b069014](https://github.com/jishnuteegala/git-chunks/commit/b0690144ae90bd6057214d94fe7b671685a90e17))
* make release retries tolerate propagation ([#8](https://github.com/jishnuteegala/git-chunks/issues/8)) ([599bac1](https://github.com/jishnuteegala/git-chunks/commit/599bac16c6121beabfe688e99c6970d92ac3a10a))
* publish release before winget validation ([#12](https://github.com/jishnuteegala/git-chunks/issues/12)) ([a8cd4d1](https://github.com/jishnuteegala/git-chunks/commit/a8cd4d1c0afb2420873cc3e25ac594eec689de5a))
* use stable publisher API responses ([#10](https://github.com/jishnuteegala/git-chunks/issues/10)) ([7d94da9](https://github.com/jishnuteegala/git-chunks/commit/7d94da964c351bfb40b1d9872c3b86881eb16a9d))
* verify publishers through stable APIs ([#11](https://github.com/jishnuteegala/git-chunks/issues/11)) ([041dd9b](https://github.com/jishnuteegala/git-chunks/commit/041dd9bd579129816bc92a6cad6449352be363eb))


### Documentation

* document publishing operations ([#14](https://github.com/jishnuteegala/git-chunks/issues/14)) ([7dfe613](https://github.com/jishnuteegala/git-chunks/commit/7dfe61310258e799d31a4d859cf6df30347aea80))
* document Release Please commit conventions ([b069014](https://github.com/jishnuteegala/git-chunks/commit/b0690144ae90bd6057214d94fe7b671685a90e17))
* record npm trusted publisher configuration status ([#16](https://github.com/jishnuteegala/git-chunks/issues/16)) ([3e26b37](https://github.com/jishnuteegala/git-chunks/commit/3e26b37b4abae2e2e9cc326d9c8b4b2bfa423b10))


### Build System and Dependencies

* **deps:** bump actions/checkout from 4.3.1 to 7.0.0 ([#3](https://github.com/jishnuteegala/git-chunks/issues/3)) ([38db7d3](https://github.com/jishnuteegala/git-chunks/commit/38db7d34b259d1a08a3673d75d44680c6190c27d))
* **deps:** bump actions/download-artifact from 4.3.0 to 8.0.1 ([#4](https://github.com/jishnuteegala/git-chunks/issues/4)) ([4ae106c](https://github.com/jishnuteegala/git-chunks/commit/4ae106c3ac63909adedc83c05335c47595db8295))
* **deps:** bump actions/setup-node from 4.4.0 to 7.0.0 ([#6](https://github.com/jishnuteegala/git-chunks/issues/6)) ([062338d](https://github.com/jishnuteegala/git-chunks/commit/062338d71248a08a6bde17aba2de9d3801b3c133))
* **deps:** bump actions/upload-artifact from 4.6.2 to 7.0.1 ([#7](https://github.com/jishnuteegala/git-chunks/issues/7)) ([34e184b](https://github.com/jishnuteegala/git-chunks/commit/34e184bed5de744bf19ab4db687294f6d31d816e))
* **deps:** bump googleapis/release-please-action from 8b8fd2cc23b2e18957157a9d923d75aa0c6f6ad5 to 5c625bfb5d1ff62eadeeb3772007f7f66fdcf071 ([#5](https://github.com/jishnuteegala/git-chunks/issues/5)) ([336c601](https://github.com/jishnuteegala/git-chunks/commit/336c601acb9aacd180b4f7edfa8b624789510e33))

## 0.1.0 (2026-07-18)


### ⚠ BREAKING CHANGES

* rename project to git-chunks
* rename project to git-chunker (npm name git-chunk was taken)

### Features

* add native Linux packages (deb, rpm, apk, pacman) and scan-timeout docs ([1257ad1](https://github.com/jishnuteegala/git-chunks/commit/1257ad19b24543aa3597740b2615bb8fc8b7af6a))
* initial release ([fc04a75](https://github.com/jishnuteegala/git-chunks/commit/fc04a7594e4823661e3fc99da790cc5b4581f9b1))
* rename project to git-chunker (npm name git-chunk was taken) ([3a28edd](https://github.com/jishnuteegala/git-chunks/commit/3a28edd9e8428cae7b0c75196dcc9d9323f67cfb))
* rename project to git-chunks ([d0fd615](https://github.com/jishnuteegala/git-chunks/commit/d0fd615577808337cb402298cf4e4be301e39a48))


### Bug Fixes

* address pre-release findings ([#2](https://github.com/jishnuteegala/git-chunks/issues/2)) ([dba217b](https://github.com/jishnuteegala/git-chunks/commit/dba217bb023fd6222debe853c3f470eb088c98a1))
