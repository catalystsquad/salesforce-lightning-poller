## [1.3.4](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.3.3...v1.3.4) (2024-02-06)


### Bug Fixes

* Add list of missing dependencies to the error when dependencies are missing ([#32](https://github.com/catalystsquad/salesforce-lightning-poller/issues/32)) ([ba7d27a](https://github.com/catalystsquad/salesforce-lightning-poller/commit/ba7d27adb7dbed4580655d83de31710d129dd01d))

## [1.3.3](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.3.2...v1.3.3) (2023-07-14)


### Bug Fixes

* handle invalid query locator error ([#28](https://github.com/catalystsquad/salesforce-lightning-poller/issues/28)) ([ad5b031](https://github.com/catalystsquad/salesforce-lightning-poller/commit/ad5b031c86905458837bf4fe85b33c1449957089))

## [1.3.2](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.3.1...v1.3.2) (2023-05-09)


### Bug Fixes

* save nextUrl when soqlResponse is not done ([#27](https://github.com/catalystsquad/salesforce-lightning-poller/issues/27)) ([57744ac](https://github.com/catalystsquad/salesforce-lightning-poller/commit/57744acaaf7f4ea10d9874b080eae92c9a3ff5e9))

## [1.3.1](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.3.0...v1.3.1) (2023-03-29)


### Bug Fixes

* always remove matching previously polled records ([#25](https://github.com/catalystsquad/salesforce-lightning-poller/issues/25)) ([680d295](https://github.com/catalystsquad/salesforce-lightning-poller/commit/680d29567416d6c34b51f122a876c399fecc86dd))

# [1.3.0](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.2.3...v1.3.0) (2023-03-28)


### Features

* refactor last queried records  ([#24](https://github.com/catalystsquad/salesforce-lightning-poller/issues/24)) ([70f680a](https://github.com/catalystsquad/salesforce-lightning-poller/commit/70f680ad31a1b7476cf435584839d903f7d1a739))

## [1.2.3](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.2.2...v1.2.3) (2023-03-27)


### Bug Fixes

* check dependencies during the poll loop ([#19](https://github.com/catalystsquad/salesforce-lightning-poller/issues/19)) ([edd3474](https://github.com/catalystsquad/salesforce-lightning-poller/commit/edd34743c08c970ffe7693248f7bf6e5defe913c))

## [1.2.2](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.2.1...v1.2.2) (2023-03-03)


### Bug Fixes

* implement soql queryall endpoint ([#20](https://github.com/catalystsquad/salesforce-lightning-poller/issues/20)) ([be896fd](https://github.com/catalystsquad/salesforce-lightning-poller/commit/be896fd3abf15b73c90a89aabf3c69d28ff6f21c))

## [1.2.1](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.2.0...v1.2.1) (2023-01-30)


### Bug Fixes

* refactor depends on and sync map ([#18](https://github.com/catalystsquad/salesforce-lightning-poller/issues/18)) ([7eeb359](https://github.com/catalystsquad/salesforce-lightning-poller/commit/7eeb3598c4a5199de28f7cdadc4f4f7fd9bf5db2))

# [1.2.0](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.6...v1.2.0) (2023-01-17)


### Features

* support dependencies in queries ([#17](https://github.com/catalystsquad/salesforce-lightning-poller/issues/17)) ([2f8de32](https://github.com/catalystsquad/salesforce-lightning-poller/commit/2f8de327c2c7105a2411775bd62b1d8f3f90611b))

## [1.1.6](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.5...v1.1.6) (2022-11-09)


### Bug Fixes

* set poll status to true ([#16](https://github.com/catalystsquad/salesforce-lightning-poller/issues/16)) ([c5960f1](https://github.com/catalystsquad/salesforce-lightning-poller/commit/c5960f1c91f172feefde378b07c47c854ac68c3f)), closes [#6](https://github.com/catalystsquad/salesforce-lightning-poller/issues/6)

## [1.1.5](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.4...v1.1.5) (2022-10-27)


### Bug Fixes

* automatically detect expired sessions and attempt reauthentication ([#15](https://github.com/catalystsquad/salesforce-lightning-poller/issues/15)) ([a013e1e](https://github.com/catalystsquad/salesforce-lightning-poller/commit/a013e1e164689dde5991cd0f4472e19717fca8d8))

## [1.1.4](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.3...v1.1.4) (2022-10-13)


### Bug Fixes

* stringToTimeMap func, return empty map if string is empty ([#14](https://github.com/catalystsquad/salesforce-lightning-poller/issues/14)) ([f36df3f](https://github.com/catalystsquad/salesforce-lightning-poller/commit/f36df3fc26d7a5b5e531fd3de4645ebf3c7393bd))

## [1.1.3](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.2...v1.1.3) (2022-10-13)


### Bug Fixes

* implement a starting position override configuration ([#13](https://github.com/catalystsquad/salesforce-lightning-poller/issues/13)) ([2fb8261](https://github.com/catalystsquad/salesforce-lightning-poller/commit/2fb8261192a53f30bbe9d9a637b2a5e133cc483e))

## [1.1.2](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.1...v1.1.2) (2022-09-14)


### Bug Fixes

* dedupe record logic ([#11](https://github.com/catalystsquad/salesforce-lightning-poller/issues/11)) ([5893bbe](https://github.com/catalystsquad/salesforce-lightning-poller/commit/5893bbe238679923a3dc8649b8e79ed6f72e34ca))

## [1.1.1](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.1.0...v1.1.1) (2022-09-13)


### Bug Fixes

* save previeous record ids in position, remove dupe records ([#10](https://github.com/catalystsquad/salesforce-lightning-poller/issues/10)) ([7f73904](https://github.com/catalystsquad/salesforce-lightning-poller/commit/7f73904ecd2692134ab02b200c69726273833bdb))

# [1.1.0](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.0.3...v1.1.0) (2022-09-13)


### Features

* implement usage of NextRecordsUrl in SOQL response ([#8](https://github.com/catalystsquad/salesforce-lightning-poller/issues/8)) ([be0364f](https://github.com/catalystsquad/salesforce-lightning-poller/commit/be0364f24e244a6db4a55446a800c12e993f1806))

## [1.0.3](https://github.com/catalystsquad/salesforce-lightning-poller/compare/v1.0.2...v1.0.3) (2022-08-29)


### Bug Fixes

* date comparison ([#7](https://github.com/catalystsquad/salesforce-lightning-poller/issues/7)) ([b8ec412](https://github.com/catalystsquad/salesforce-lightning-poller/commit/b8ec41297288c4fee5f14d70b16f9ccf36b81016))

## [1.0.2](https://github.com/catalystsquad/repo-name/compare/v1.0.1...v1.0.2) (2022-06-24)


### Bug Fixes

* Refactor to handle queries in parallel and on independent poll statuses ([#6](https://github.com/catalystsquad/repo-name/issues/6)) ([b9aa75f](https://github.com/catalystsquad/repo-name/commit/b9aa75f27fb29b4a212d32f47d60fd00145c6435))

## [1.0.1](https://github.com/catalystsquad/repo-name/compare/v1.0.0...v1.0.1) (2022-06-07)


### Bug Fixes

* Fix position tracking edge case ([#5](https://github.com/catalystsquad/repo-name/issues/5)) ([bf4b671](https://github.com/catalystsquad/repo-name/commit/bf4b671ffc2d36ade768892eb3fba1a2263b165a))

# 1.0.0 (2022-05-11)


### Bug Fixes

* Add persistence and better handling of offsets and dates to ensure we don't miss or duplicate data ([#4](https://github.com/catalystsquad/repo-name/issues/4)) ([43a5aa1](https://github.com/catalystsquad/repo-name/commit/43a5aa1bf236ca8e9e2c708c4724ac163149360f))
* Initial release ([#1](https://github.com/catalystsquad/repo-name/issues/1)) ([865bdd1](https://github.com/catalystsquad/repo-name/commit/865bdd198042d1988bb88c393fc3f3afbac14890))
* Refactor query to a function, instead of a string. ([#3](https://github.com/catalystsquad/repo-name/issues/3)) ([e433e9f](https://github.com/catalystsquad/repo-name/commit/e433e9ff81dfceb7175e6af6e9f8450d04c0eede))
