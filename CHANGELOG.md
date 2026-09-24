# Changelog

## [0.3.0](https://github.com/Heliopsy/tix/compare/v0.2.0...v0.3.0) (2026-09-24)


### Features

* **core:** a theme a tenant can name, and completion that installs itself ([343af15](https://github.com/Heliopsy/tix/commit/343af15744ee4ae1fc43982e0781a9c0c3a6bdf0))
* **query:** negation and weak matching, a resumable watch, and tenant switching ([dabfb2d](https://github.com/Heliopsy/tix/commit/dabfb2d3026f1c8339423f2b99b8608ba82ae841))
* statistics on every surface, and a theme that reaches all of them ([65261dd](https://github.com/Heliopsy/tix/commit/65261ddae0f688ea4b5f32fbcdba5dbffba31d4c))
* **tui:** say which claimed cards are yours ([ac60743](https://github.com/Heliopsy/tix/commit/ac607439e024d79999a0ff86d11f674a7e2526e8))
* **tui:** switch tenant without leaving, and filter the activity tail ([2c65c19](https://github.com/Heliopsy/tix/commit/2c65c19ea937fd3a4a34157c8ab525a95b478dca))
* **web:** make import and export one screen that says what it does ([2d4c534](https://github.com/Heliopsy/tix/commit/2d4c5344eafa47f571a137d7e2416f0a56d26690))
* **web:** make the screens people actually use less embarrassing ([e6685e0](https://github.com/Heliopsy/tix/commit/e6685e065bb0d633344156dfac98201f8bd80fa7))
* **web:** say what sync does, and show what it has done ([8c2e1a1](https://github.com/Heliopsy/tix/commit/8c2e1a17d80e9f89cc3c59703dfbb2ad7b14358e))
* **web:** show the running version, and say when a newer one exists ([9e1bc55](https://github.com/Heliopsy/tix/commit/9e1bc55b47e110a2ada78bae83ccbf8a25eda131))
* **web:** show what sits under a tenant ([2d3daca](https://github.com/Heliopsy/tix/commit/2d3dacaa346607f71a9c201873be5c2fdaff4ee2))


### Bug Fixes

* **ci:** make the parity gate actually run the parity tests ([4b0abfd](https://github.com/Heliopsy/tix/commit/4b0abfd12c586c3d5ae3d6cedec130d2ceda3e83))
* **ci:** unchecked writes in the stats table, and two markdown lint errors ([e612d98](https://github.com/Heliopsy/tix/commit/e612d981ae3f386d427928d6994db8db63650d2d))
* **web:** one filter parser, so the browser stops answering a different question ([babb4b1](https://github.com/Heliopsy/tix/commit/babb4b1147669cc4eeefb28ca0e51347acb63a7a))

## [0.2.0](https://github.com/Heliopsy/tix/compare/v0.1.0...v0.2.0) (2026-09-23)


### Features

* **log:** give logs somewhere to go and a size to stay under ([dbdb337](https://github.com/Heliopsy/tix/commit/dbdb337436f56b8941a9c2af97ae071329cadb36))


### Bug Fixes

* report a real version from go install, and keep fork code off our runner ([343eda8](https://github.com/Heliopsy/tix/commit/343eda8b7b22a4d076d7c00f7bea4d03be0db7df))

## 0.1.0 (2026-09-23)


### ⚠ BREAKING CHANGES

* **ssh:** `tix ssh` now serves enrolled keys by default. The sandbox that provisioned an ephemeral tenant for any key is behind `--demo` (`ssh.demo`, `TIX_SSH_DEMO`). Add `--demo` to keep the old behaviour; otherwise enrol keys with `tix user key add --actor <handle>`.

### Features

* bootstrap tix with full v1 specification ([2a39f1f](https://github.com/Heliopsy/tix/commit/2a39f1fc342b909a83458cf646ddb6ce42bde735))
* **bundle:** share workflows, fields, tags and templates between installs ([eec6217](https://github.com/Heliopsy/tix/commit/eec6217bb439fa0906cc2559606cd8bfaa084c50))
* **cli:** colour the output by default, with a way to turn it off ([4440ced](https://github.com/Heliopsy/tix/commit/4440cedf431a27ab885bd7679176cb6d03ce69ba))
* **clock,id:** add the injectable time source and identifier generator ([ad93275](https://github.com/Heliopsy/tix/commit/ad93275583a6b200e964c826440e95b560530f6b))
* close the wave seven gaps ([f48d4da](https://github.com/Heliopsy/tix/commit/f48d4da99cfca02fed23389e681c477aadbc2ae5))
* **core,output:** make bulk paths streamable ([2cabf06](https://github.com/Heliopsy/tix/commit/2cabf06b914111b7548c22c2d95611571119f36b))
* **core:** add domain error taxonomy ([8f8eb6a](https://github.com/Heliopsy/tix/commit/8f8eb6a3ec989b4037376de96e8297b8de2c538a))
* **core:** add the domain model and Service contract ([448086d](https://github.com/Heliopsy/tix/commit/448086df0faaa072d8a6f9a53b05f7e7d56b2ac2))
* **core:** define the component-sharing contract ([1b121df](https://github.com/Heliopsy/tix/commit/1b121df25fc5b568d4acd9a88c702bd4aafde306))
* **httpapi:** add the shared wire contract ([fc73cba](https://github.com/Heliopsy/tix/commit/fc73cbab45ed0b7eddd089309113d41247995c2c))
* **httpapi:** add the web mount seam ([6942632](https://github.com/Heliopsy/tix/commit/6942632c87804d91f33f9f1bcf7cd35b95917c78))
* implement Wave 2 (config, output, authz, auth, sqlite store) ([f68eed3](https://github.com/Heliopsy/tix/commit/f68eed3f4f31bdc42d242dbc83111e0f1bbc8b24))
* implement Wave 4 and rename labels to tags ([0c22a6b](https://github.com/Heliopsy/tix/commit/0c22a6b9f3a521f59df4c959f47fd97ebb6ef6e0))
* implement Wave 5 (web UI, TUI, Postgres, transfer, external sync) ([5271a1c](https://github.com/Heliopsy/tix/commit/5271a1c7e1bc9c165a447b58cc8be57fa03fd654))
* make the shipped features work, and the web interface usable ([4e92a78](https://github.com/Heliopsy/tix/commit/4e92a78042f43f28978d37801f98a27a38842f30))
* move to heliopsy, name people instead of identifiers, and settle the interface ([41dc15d](https://github.com/Heliopsy/tix/commit/41dc15d4e7ed6f64825b121dd8b3cd3afba0e818))
* **output:** configurable date format and timezone ([1dec83f](https://github.com/Heliopsy/tix/commit/1dec83ff8152df4970f8cfb7df44da3b89053234))
* **project:** give projects a colour and an icon ([564c72e](https://github.com/Heliopsy/tix/commit/564c72eefe993e849991d82406f41be4939e364f))
* revoke credentials on account removal, and spec component sharing ([dbfbf46](https://github.com/Heliopsy/tix/commit/dbfbf464d44a82721bb35fe72971521081a42a9d))
* **server:** mount the WebSocket event stream ([3d0bbb6](https://github.com/Heliopsy/tix/commit/3d0bbb6b4f5be313345844cef593afc831801c28))
* **service,outbox:** add the service core, audit capture and event outbox ([34e37db](https://github.com/Heliopsy/tix/commit/34e37dba7e47aa9c3101ce30a3e6d142a6797bc9))
* **service:** implement user, session, token and webhook management ([2d6f3bb](https://github.com/Heliopsy/tix/commit/2d6f3bb69cf437b44ab2d623cb823ed70d3b050a))
* **service:** implement Wave 3, the service layer ([a085b71](https://github.com/Heliopsy/tix/commit/a085b71fb73d711ed283cc6b574e89670147d9ac))
* **ssh:** enrol real keys, and see who is connected ([4f0a39f](https://github.com/Heliopsy/tix/commit/4f0a39f82551d12bfad8b1e62647856e32e3c3aa))
* **ssh:** serve the terminal interface over ssh ([bd6409f](https://github.com/Heliopsy/tix/commit/bd6409f028fae711795b481fccfee40ed3c6d84a))
* **ssh:** session limits, keepalive and configurable listener ([fecbb93](https://github.com/Heliopsy/tix/commit/fecbb933b27f8809e3a6f47299d09c814f22c8b7))
* **store:** add the persistence contract, scoped query builder and schema ([29ac190](https://github.com/Heliopsy/tix/commit/29ac1904ae5b63cc19644c95415a08b4a1549358))
* **store:** refuse a database on a network filesystem ([2e12b7e](https://github.com/Heliopsy/tix/commit/2e12b7e9978886868e6c9338daf30cb3c1eda500))
* **task:** delete a subtree with cascade instead of refusing ([3c58f18](https://github.com/Heliopsy/tix/commit/3c58f1897d16773bd46cc1518f29e3a43611baea))
* **tasks:** sort by urgency, priority then deadline, by default ([13ad657](https://github.com/Heliopsy/tix/commit/13ad657b2ec99aaae956e11e818c25b70c170d48))
* **tui:** rebuild the terminal interface ([035dab4](https://github.com/Heliopsy/tix/commit/035dab4576b508d07e7740d23200fa4920b8c18f))
* **web:** a real board, a settings page, and readable history ([2dc15f2](https://github.com/Heliopsy/tix/commit/2dc15f2e950d9e37a547bdef92b428b0e957e841))
* **web:** add a low contrast scheme and show the task identifier ([52218a8](https://github.com/Heliopsy/tix/commit/52218a83baefe5df5fc9d18da87c5d3120d9df8c))
* **web:** draw the tick properly and give the list a voice ([31c1afa](https://github.com/Heliopsy/tix/commit/31c1afac83d25098803a2f923b39ef2a60277889))
* **web:** keyboard shortcuts, readable history, and colour fixes ([7546847](https://github.com/Heliopsy/tix/commit/7546847c8b48b0d70b3688bd3dc3a4bdb98500c3))
* **web:** let each listing show the columns its reader wants ([55cddec](https://github.com/Heliopsy/tix/commit/55cddec3e87b372b7bce2526f569e1e2de779121))
* **web:** rebuild the browser interface around a usable layout ([59e0450](https://github.com/Heliopsy/tix/commit/59e0450aaef5415ea9f95cd4ff76cd285fc04e13))


### Bug Fixes

* add the ResolveDomain route and align the output format lists ([d27372e](https://github.com/Heliopsy/tix/commit/d27372e25cca97235ccce5be954905e5acaa83db))
* **auth,ci:** resolve the security scan findings ([e2e7033](https://github.com/Heliopsy/tix/commit/e2e70330740c8e7824dfb2656117c7b62cb6ca4f))
* **auth:** let a signed-in browser actually reach its pages ([03927bf](https://github.com/Heliopsy/tix/commit/03927bf435b41fbb975e4e1b87c60210223ee56e))
* carry project appearance everywhere, and bring the docs up to date ([b8163fb](https://github.com/Heliopsy/tix/commit/b8163fb05c6f86aed97743b05b4bad9b6e11acc0))
* **ci:** make every lint gate pass, and fix the glob that was hiding findings ([987854f](https://github.com/Heliopsy/tix/commit/987854fc06bb324712bb8acbf59727bbccdc34ca))
* **ci:** make the pipeline green on a private repository ([d8962aa](https://github.com/Heliopsy/tix/commit/d8962aac954d77a3af0cc37b5d3f6249ca8a2b3a))
* close the three findings the linter caught ([6616e8f](https://github.com/Heliopsy/tix/commit/6616e8fc0cbeac3846ab82e63de8555603328c11))
* **container:** use the numeric distroless uid ([2f21af2](https://github.com/Heliopsy/tix/commit/2f21af2557b2d2c5d15fc39197ec865ad3b72847))
* **events:** stop the stream handing administrative detail to any subscriber ([38e7349](https://github.com/Heliopsy/tix/commit/38e7349862803ba2839252181ea3f6e8821c34b7))
* give three vocabularies one owner each ([e520586](https://github.com/Heliopsy/tix/commit/e5205865e63150655c05fc1e8ccdee5f8e4cd9ed))
* honour retention configuration, the drain mode, and show a new secret ([33110fb](https://github.com/Heliopsy/tix/commit/33110fb97cc16090d4e6c0280dd405fc1da859a7))
* let the database wait, and let CI run the checks it has ([0b353da](https://github.com/Heliopsy/tix/commit/0b353dac85d923304b6c233347dec674cbce55f5))
* make a connect test hermetic and quiet gosec on outgoing cookies ([eccec06](https://github.com/Heliopsy/tix/commit/eccec065c372dddcb47c8bf6c89f9858fbe7aebc))
* **output:** use reflect.Pointer rather than the deprecated alias ([b955505](https://github.com/Heliopsy/tix/commit/b955505d5770ab9c8a6b66c5362fabfdc4d64734))
* **security:** close cross-tenant, lifecycle and concurrency defects ([a0c78eb](https://github.com/Heliopsy/tix/commit/a0c78eb91283100cff234bbe386929418025545f))
* **service:** refuse forged targets, cycles and silent data loss ([37d4195](https://github.com/Heliopsy/tix/commit/37d419547644284790366737c5d98acf6dcffed0))
* **spec,build:** tidy blank lines and widen just check ([f8a120f](https://github.com/Heliopsy/tix/commit/f8a120fcf0cee0e3a66997127066502a3ebc4c68))
* **storeloc:** an octal escape above 0377 is not one byte ([2b049d4](https://github.com/Heliopsy/tix/commit/2b049d4a3f565e9afdc1cdf2231026a575fc3d38))
* **store:** reject renewing or releasing an expired lease ([0688cce](https://github.com/Heliopsy/tix/commit/0688cce71da551e3ec81bb0bcec3c084efcd0854))
* **transfer:** stop import losing and resurrecting work ([e6db342](https://github.com/Heliopsy/tix/commit/e6db342e20f9535a2772b20053e0dd6e3697d320))
* **tui:** advertise the action that opens a task ([333954f](https://github.com/Heliopsy/tix/commit/333954f92bda9baf9d8c602ae1b02bd1dca42de3))
* **web,service:** close an open redirect and two flaky tests ([b9a0124](https://github.com/Heliopsy/tix/commit/b9a01245c174aa6b0e981bf745549cb06a7118f7))
* **web:** make the preference buttons actually change the page ([2fa5c11](https://github.com/Heliopsy/tix/commit/2fa5c11f513ad87e7d4af34da5ae61f330f8c418))
* **web:** make the tick a toggle, and fix the dark palette ([9203ddc](https://github.com/Heliopsy/tix/commit/9203ddc466a996c29dea3932b8219b4a06c94caa))


### Dependencies

* add install-local recipe ([938b5ed](https://github.com/Heliopsy/tix/commit/938b5ed2e87a505e34e4c4cda2918b2e1e989003))
* move everything to latest, pinned exact ([e120e76](https://github.com/Heliopsy/tix/commit/e120e764019d0b5797ec0d81eebd6667f13aecc0))
* run the gate tools in a container again, where there is one ([c67df2c](https://github.com/Heliopsy/tix/commit/c67df2c628e1235745c5c4ee5f6fbd6d1b695f86))


### Documentation

* describe the status a first release actually has ([7a290b4](https://github.com/Heliopsy/tix/commit/7a290b436dfc59a5684aa79146b8575c9769ce06))
