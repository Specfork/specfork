# Changelog

## 1.0.0 (2026-10-28)


### ⚠ BREAKING CHANGES

* rename SpecHub to Specfork ([#6](https://github.com/Specfork/specfork/issues/6))

### Features

* **api:** specfork backend: GitHub sign-in, scores, claims ([#26](https://github.com/Specfork/specfork/issues/26)) ([3cdeb24](https://github.com/Specfork/specfork/commit/3cdeb24adc9f51f9c44d7c8fa9a3575268d03f05))
* **cli:** install the Specfork CLI with npx, brew or curl ([#24](https://github.com/Specfork/specfork/issues/24)) ([4a3dd69](https://github.com/Specfork/specfork/commit/4a3dd6965a096ea27f4879afd1a0de8285edb1d6))
* **cli:** spechub search, info, resolve, lint, mcp ([5394a72](https://github.com/Specfork/specfork/commit/5394a72105e5b57db9f68b48639b4b8c183bab25))
* Go CLI, MCP server, crawler, registry claims, Pages site ([0e7e721](https://github.com/Specfork/specfork/commit/0e7e721e36c1ad6aaacc4dad7dbcce4b307d7622))
* **index:** crawler and Pages site ([a5177eb](https://github.com/Specfork/specfork/commit/a5177eb8b9106a93f226828320a78ae290a64fe0))
* **mcp:** hosted MCP server with usage counters ([#22](https://github.com/Specfork/specfork/issues/22)) ([60a36be](https://github.com/Specfork/specfork/commit/60a36be52876072664a5433acfafdb19ae82811c))
* **mcp:** stdio MCP server with search, get and resolve ([55276e3](https://github.com/Specfork/specfork/commit/55276e3eeddc208bac68bd6597c50fc79b3f2ebe))
* **registry:** names.json and claim checks ([13e72d0](https://github.com/Specfork/specfork/commit/13e72d0c3befaa1053d8872ce9e369e5c0e36a83))
* **resolve:** version selection, lockfile, bundles ([40b483d](https://github.com/Specfork/specfork/commit/40b483d703811e766ce46b1de91547bd3c650fec))
* serve Specfork at specfork.dev ([#9](https://github.com/Specfork/specfork/issues/9)) ([eb4c4ca](https://github.com/Specfork/specfork/commit/eb4c4cad7f3c16937a7ab49e102b28bf57c5f21c))
* **site:** feedback page, links and issue forms ([#11](https://github.com/Specfork/specfork/issues/11)) ([821b2eb](https://github.com/Specfork/specfork/commit/821b2eb23b659c60a43f0dfaf908d0dc35c4ea0c))
* **site:** specfork.dev v2 with spec content, filters, docs and SEO ([#10](https://github.com/Specfork/specfork/issues/10)) ([1a6ddf6](https://github.com/Specfork/specfork/commit/1a6ddf6ed002d7b32f2fdb0662c36612fbcd4525))
* **skills:** add extract-spec skill ([c4e8d24](https://github.com/Specfork/specfork/commit/c4e8d244b319368e807efb0f2299884f78bc057a))
* **spec:** front matter parsing and lint ([97494e4](https://github.com/Specfork/specfork/commit/97494e4ee46d90f2a08b47344a399f4568da40d5))
* SpecHub specs, extract-spec skill, scheduling seed from Cal.com ([01de4c7](https://github.com/Specfork/specfork/commit/01de4c7fc2031addfb5ce0b84e25f082c7711f2c))
* **specs:** Specs good enough for cheap models to build ([#18](https://github.com/Specfork/specfork/issues/18)) ([4c0a81d](https://github.com/Specfork/specfork/commit/4c0a81d7e4640ae5c37319b323955c425beea4c6))


### Bug Fixes

* **auth:** add avatar and bio to users ([916662f](https://github.com/Specfork/specfork/commit/916662fb3618f6dcf20388e1ffd3a597ca764905))
* **auth:** define session tokens ([d32db03](https://github.com/Specfork/specfork/commit/d32db03700a4755e27c87effedc260a8f17cdb11))
* **auth:** list sign-up fields ([0afa4d1](https://github.com/Specfork/specfork/commit/0afa4d164315c15b8adc5021fd9464e9eabd8444))
* **auth:** make the common-password check testable ([2afee6a](https://github.com/Specfork/specfork/commit/2afee6a24dbf591c1d3cb4fec5ea2629cc82f93f))
* **auth:** require same-origin state changes ([f74fc7f](https://github.com/Specfork/specfork/commit/f74fc7f9e02ea9ceb92434cc6f9a0ede2505bc6b))
* **auth:** tell locked only after a correct password ([40d6fdd](https://github.com/Specfork/specfork/commit/40d6fddc14a8fca2b9004fe789326e1445c822cb))
* **index:** track rate limits per API resource ([#2](https://github.com/Specfork/specfork/issues/2)) ([a1ac388](https://github.com/Specfork/specfork/commit/a1ac388c371363b82a12af85bb14ac4d10db8dd2))
* **scheduling:** add a stable calendar UID to bookings ([b7a4e49](https://github.com/Specfork/specfork/commit/b7a4e49d38efaca6ea2d70509e15980e62654368))
* **scheduling:** add rescheduled flag to bookings ([fe52d2f](https://github.com/Specfork/specfork/commit/fe52d2fbe46b955ac25773bac1d92373e02c6551))
* **scheduling:** add the time zone lock PUB-2 refers to ([4bf5340](https://github.com/Specfork/specfork/commit/4bf5340bef992c6daf9a12faa3b1e47a655f3473))
* **scheduling:** align slots from free ranges after busy time ([1985fe1](https://github.com/Specfork/specfork/commit/1985fe172bd13020c02a28a8ecea435e28cd7962))
* **scheduling:** align SLT-18 with the half-open query range ([08d1de3](https://github.com/Specfork/specfork/commit/08d1de321c4158bb40e88c36a5a2a384e0fd9d15))
* **scheduling:** allow empty overrides, not empty rules ([05c31f9](https://github.com/Specfork/specfork/commit/05c31f9facd76a3589162c219e252a6c096a33bd))
* **scheduling:** cancel and reschedule flags bind bookers only ([8cf94d3](https://github.com/Specfork/specfork/commit/8cf94d3f727e3a9c1210c434e9e62ef636e2d3c2))
* **scheduling:** compare limits with every longer period ([4f18745](https://github.com/Specfork/specfork/commit/4f18745496e14e7e758a2bef432545315e4166b2))
* **scheduling:** define booker identity and hold rules ([413b8f2](https://github.com/Specfork/specfork/commit/413b8f2c875a03c458f437a10578c7743996e46c))
* **scheduling:** define confirm conflicts and link rules ([2aec932](https://github.com/Specfork/specfork/commit/2aec932014126ccd69ab51c2d7edf86a5fb06ad1))
* **scheduling:** define rolling window counting ([d59bef6](https://github.com/Specfork/specfork/commit/d59bef6d4fc89555781b7e0fcd06012ab498404e))
* **scheduling:** define slot query range ([4440f22](https://github.com/Specfork/specfork/commit/4440f22bedc1180e94e33b5af7a3e1a199d20d33))
* **scheduling:** define time-off dates and reason ([c7fe721](https://github.com/Specfork/specfork/commit/c7fe721441b904d7fd60158227e68386a4f8f4ac))
* **scheduling:** drop the booker from guests, name guests ([677831d](https://github.com/Specfork/specfork/commit/677831dc20769a09639676f154ca1f7b33accda1))
* **scheduling:** fall back to a plain link without a video app ([b405936](https://github.com/Specfork/specfork/commit/b40593630806ebf12d77f827c73ee8d52f7d592b))
* **scheduling:** give booking fields a key ([8aab535](https://github.com/Specfork/specfork/commit/8aab5355fb1a44aadd664b27a19d3a8f264d0330))
* **scheduling:** keep name and email visible, hidden never required ([e761816](https://github.com/Specfork/specfork/commit/e761816c877dbe6dd8b3c21770e6d6bfdaa5a491))
* **scheduling:** let owners pick the default schedule ([9878399](https://github.com/Specfork/specfork/commit/9878399dbc55aadf89d82c0c8bd8f3e66e06ed15))
* **scheduling:** match a host attendee by account email ([3eb5e87](https://github.com/Specfork/specfork/commit/3eb5e87b2227571ffbf359987e196bd7be6ce0c9))
* **scheduling:** name booking re-check error codes ([46addad](https://github.com/Specfork/specfork/commit/46addad64f62d2fe593a1f7d4d54b3ba1fcbc103))
* **scheduling:** offer a slot only if it fits the limit ([f7046a8](https://github.com/Specfork/specfork/commit/f7046a83f8e46df4476c2428d9ce4d5c43dd4cbd))
* **scheduling:** refuse confirming past a limit, define valid email ([63a1882](https://github.com/Specfork/specfork/commit/63a1882f3fec9b8fa1a185d37518fb9472eefa0d))
* **scheduling:** refuse reschedule once started ([6bf4acd](https://github.com/Specfork/specfork/commit/6bf4acd97e0b752ab0f3b12c8d8e7632de46aaa2))
* **scheduling:** require a location pick when several exist ([afb23cb](https://github.com/Specfork/specfork/commit/afb23cb171fdb08ede080a884c905e9584548ee7))
* **scheduling:** reserve route names in the page namespace ([e3fbc6d](https://github.com/Specfork/specfork/commit/e3fbc6db6f80c6942eb184b7b71bc4ab6279abfd))
* **scheduling:** resolve DST-skipped and repeated wall times ([191b942](https://github.com/Specfork/specfork/commit/191b94246861cc34a75a14498e806c5480cca4e7))
* **scheduling:** rolling_available fallback, length bound, position order ([27f8910](https://github.com/Specfork/specfork/commit/27f891083e2d966cff6fc25fb2cc5fdeecf8b887))
* **scheduling:** say what a reschedule carries over ([14469d2](https://github.com/Specfork/specfork/commit/14469d26ce1597c6dfcb8b71f979b6e5b3b87f6c))
* **scheduling:** say which schedule becomes default on delete ([4191f77](https://github.com/Specfork/specfork/commit/4191f77d045bc03c459e99c854bb5dd1d09db911))
* **scheduling:** say whose flag makes pending bookings busy ([8988b1c](https://github.com/Specfork/specfork/commit/8988b1c8cc8bf41364e2b62085918a3b5450e3f2))
* **scheduling:** settle which zone each window uses ([a6081a6](https://github.com/Specfork/specfork/commit/a6081a68e0b9e5c23172ca54b3143c7fba6c431b))
* **scheduling:** store host no-show and limit marking to accepted ([06466f2](https://github.com/Specfork/specfork/commit/06466f2df23709c4b3105cb4e931df47c15f883a))
* **site:** fingerprint CSS and JS so deploys aren't hidden by caching ([#13](https://github.com/Specfork/specfork/issues/13)) ([8b25df8](https://github.com/Specfork/specfork/commit/8b25df850922037cd12ee8eae03d466f2f703358))
* **site:** hide cards filtered out by search or tag ([#12](https://github.com/Specfork/specfork/issues/12)) ([4568546](https://github.com/Specfork/specfork/commit/4568546fc1ed4e7735a124a7a5945bfcc65dfb06))


### Code Refactoring

* rename SpecHub to Specfork ([#6](https://github.com/Specfork/specfork/issues/6)) ([64611e5](https://github.com/Specfork/specfork/commit/64611e5379a9c1f8ba6a2582933917db7d39c8b7))
