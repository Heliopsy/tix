# Changelog

## [0.12.0](https://github.com/Heliopsy/tix/compare/v0.11.0...v0.12.0) (2026-09-30)


### Features

* **cli,tui,web:** make due dates reachable and legible on every surface ([1283e9a](https://github.com/Heliopsy/tix/commit/1283e9afcc724921cd16744479ee21d0f46fd969))
* **cli,tui,web:** make due dates reachable and legible on every surface ([96e451f](https://github.com/Heliopsy/tix/commit/96e451fdb8a364d03277ebbbfade634664661fe1))


### Bug Fixes

* **claim:** judge terminal per workflow and guard the sweeper's clear ([af52a5f](https://github.com/Heliopsy/tix/commit/af52a5fb01886d00fc0c82c655913da53652ff3a))
* **service,store:** judge terminal states per workflow and stop the sweeper clearing a live lease ([fd55c35](https://github.com/Heliopsy/tix/commit/fd55c35e4057790703467f1e78b36c006439fa6f))
* **store:** scope tenant_domains and derive the isolation guards from the schema ([d68688b](https://github.com/Heliopsy/tix/commit/d68688b977b048eed53439c3c9d74828a92a9d10))
* **store:** scope tenant_domains and derive the isolation guards from the schema ([1b10318](https://github.com/Heliopsy/tix/commit/1b10318c9c98a67b9b0547e6b50d76da5d018091))
* **tui,output:** measure text in terminal cells, not runes ([c4b28f2](https://github.com/Heliopsy/tix/commit/c4b28f282e45716d27f91407b17f93abc1e5f62d))
* **tui,output:** measure text in terminal cells, not runes ([7422a84](https://github.com/Heliopsy/tix/commit/7422a84d4339e7f42eaabcbd3f7ad2a642fc984b))
* **tui:** the due legend entry read "due due within a week" ([e0449f5](https://github.com/Heliopsy/tix/commit/e0449f52055685484f9b1b9b1d720287477a56a8))
* **web:** keep the workflow fields the editor does not render ([e1acbff](https://github.com/Heliopsy/tix/commit/e1acbfffa3e66e2a3ef9dab2bbe3f47277284f2e))
* **web:** refuse an unreadable terminal field, and spec the merge ([c15a5c2](https://github.com/Heliopsy/tix/commit/c15a5c2b5de553257467480eef1da320709f22ff))
* **web:** the workflow editor keeps the fields it does not show ([5880bfd](https://github.com/Heliopsy/tix/commit/5880bfd212fd501d0d1d85133426e31a1741f71e))


### Dependencies

* give test-postgres the timeout test already has ([4bef3a6](https://github.com/Heliopsy/tix/commit/4bef3a66130ff96b03785720f68b518a22a50b52))
* give test-postgres the timeout test already has ([54b333a](https://github.com/Heliopsy/tix/commit/54b333a87cdce5acaea5c45422bfda90bce70877))
* give test-postgres the timeout test already has ([9f0118a](https://github.com/Heliopsy/tix/commit/9f0118ad4056f6dee854d3e581cb6f90aa741922))

## [0.11.0](https://github.com/Heliopsy/tix/compare/v0.10.0...v0.11.0) (2026-09-30)


### Features

* **api:** accept a route on a task ([ea4b3d4](https://github.com/Heliopsy/tix/commit/ea4b3d43be530c7118ee3d801d3d0abfe8923413))
* **capability:** register applying a route as its own operation ([1372588](https://github.com/Heliopsy/tix/commit/137258837c1c9c69731602e4ea9201fedc99c640))
* **cli:** tix task mv --hops finds the route and prints it first ([10313d9](https://github.com/Heliopsy/tix/commit/10313d902cae0c596147f546a6885e51bea7881f))
* **core:** reachability over a workflow, shared by every surface ([6f6dd9b](https://github.com/Heliopsy/tix/commit/6f6dd9b357037b4e8479d382eb281c86820629ba))
* **service:** apply a route one ordinary transition per hop ([3d96c59](https://github.com/Heliopsy/tix/commit/3d96c59f36e35667ed423e7c6739bec77d347800))
* **tui:** a command palette, from the one list the help overlay already read ([5cbf7b1](https://github.com/Heliopsy/tix/commit/5cbf7b1cfa9e075503ee7628d26e452e411102b9))
* **tui:** edit a task body as prose, and the whole task on one form ([25f8806](https://github.com/Heliopsy/tix/commit/25f88068902d6477ede49a7e4f6621939094e15a))
* **tui:** give every input mode one panel ([b5d57fb](https://github.com/Heliopsy/tix/commit/b5d57fb7789095e66b2bd10b60b212853ffe4666))
* **tui:** one edit key, and p cycles a task's priority ([12f95d1](https://github.com/Heliopsy/tix/commit/12f95d110426c64e1ccc02340477e005ac6ecfcd))
* **tui:** redraw the board around title-led cards ([2a86fec](https://github.com/Heliopsy/tix/commit/2a86fec5428b17cc62dd3d0cc41a68f3ca4313d0))
* **tui:** the transition picker offers routes ([e388057](https://github.com/Heliopsy/tix/commit/e388057bb4268b343cad7150da1bc02f608cc4a5))
* **web:** routes on the task screen and the board ([c722b39](https://github.com/Heliopsy/tix/commit/c722b3950c683c9856ae2beb0aa009e0690c6542))


### Bug Fixes

* **tui:** answer the palette key on the settings and help screens too ([3191b3e](https://github.com/Heliopsy/tix/commit/3191b3e721824f16291927d72bec6e1ad695f8ba))
* **tui:** line the project list up whatever icon a project carries ([7a724bd](https://github.com/Heliopsy/tix/commit/7a724bd5170d70c5ab298f82914c94cc99da956e))
* **tui:** stop guessing why a conflict happened, and clear it on the way out ([7c05ed4](https://github.com/Heliopsy/tix/commit/7c05ed47b33778320ad1934202c63ca0ba4ecb7f))
* **tui:** stop marking a due date on every card ([6fccee5](https://github.com/Heliopsy/tix/commit/6fccee5bf49fe031c77749d82d76c3d498d2242e))
* **tui:** wrap the task body instead of clipping it ([2a186d8](https://github.com/Heliopsy/tix/commit/2a186d8a0ac205d89288c834c2abf488a7e2174b))

## [0.10.0](https://github.com/Heliopsy/tix/compare/v0.9.0...v0.10.0) (2026-09-28)


### Features

* **cli:** add tix status across the command line, the api and the browser ([d1a9136](https://github.com/Heliopsy/tix/commit/d1a913619d02cf0953edb6686c1aef828f725edf))
* **service:** report the installation, its servers and its work ([c1ca1ff](https://github.com/Heliopsy/tix/commit/c1ca1ff9ff9547dc093033ab4ec4e8ef1dad9b48))
* **store:** register server processes in an installation-scoped table ([fb12f2b](https://github.com/Heliopsy/tix/commit/fb12f2b8d6ba7229c1b93a80a3ca3498cc05e410))


### Bug Fixes

* **ci:** stop release-please re-proposing a version already shipped ([184c113](https://github.com/Heliopsy/tix/commit/184c113b3d81e6e6b16e058bd40267951f8393d5))
* **status:** keep trying to register a server the database was too busy for ([6821562](https://github.com/Heliopsy/tix/commit/68215622f9b71a21f1fdebf4ed59d6d7dbc883b0))
* **status:** stop a dead server's uptime climbing, and count in words ([c641d72](https://github.com/Heliopsy/tix/commit/c641d720cca87662b56ecc3277a48a07e54a3807))

## [0.9.0](https://github.com/Heliopsy/tix/compare/v0.8.0...v0.9.0) (2026-09-27)


### Features

* **tui:** administer the tenant from the terminal ([d0d6dc7](https://github.com/Heliopsy/tix/commit/d0d6dc74b91165de5d53584b8cc058bc226b7c42))


### Bug Fixes

* **test:** stop the build cache sizing the mutation timeout ([74072ee](https://github.com/Heliopsy/tix/commit/74072ee11289bd05da7caa6d141db3994b9d73ce))


### Dependencies

* **deps:** bump codeql-action to v4.38.2 and pflag to v1.0.10 ([2870ee5](https://github.com/Heliopsy/tix/commit/2870ee574fc1943fc15a6948eab235edff4ccf9e))

## [0.8.0](https://github.com/Heliopsy/tix/compare/v0.7.0...v0.8.0) (2026-09-27)


### Bug Fixes

* **ci:** attest the image tag that was actually pushed ([8295907](https://github.com/Heliopsy/tix/commit/8295907fc2077221ee42d344a392faa87f16353c))
* **ci:** drop the child-digest check that refused every correct digest ([295494c](https://github.com/Heliopsy/tix/commit/295494cb09c7ee9ee05f79485942639d8792ca21))
* **ci:** stop the CLA bot locking the release pull request ([06de58a](https://github.com/Heliopsy/tix/commit/06de58ae01aaf1eba0415bbad4bfac6a9fc7c9a0))


### Dependencies

* **deps:** move the charm stack to v2 ([59c4481](https://github.com/Heliopsy/tix/commit/59c44819f4d76f4aa0d6f9a1c5341cbb406367a3))


### Documentation

* **theming:** say how colour depth is resolved, and what changed ([e1ac251](https://github.com/Heliopsy/tix/commit/e1ac2516b1c432643bde8cdb1a18dcbca4e4a2a1))

## [0.7.0](https://github.com/Heliopsy/tix/compare/v0.6.0...v0.7.0) (2026-09-27)


### Features

* **config:** make page size configurable per surface ([2bb66dd](https://github.com/Heliopsy/tix/commit/2bb66dd1ae854080856a5e90c97925dd85ceefa4))
* **tui:** add an optional selection pulse ([e0c1b9b](https://github.com/Heliopsy/tix/commit/e0c1b9bdc8e5a48819a908446bc3a6e73a9885e5))


### Bug Fixes

* **ci:** attest the digests the registry actually holds ([9092718](https://github.com/Heliopsy/tix/commit/90927182e45bbf194c3c25403017158d59f154ad))
* **ci:** stop rebuilding a release that is already published ([8b4780c](https://github.com/Heliopsy/tix/commit/8b4780c0d4dc465e65dff1b8c8063299f98c9f77))

## [0.6.0](https://github.com/Heliopsy/tix/compare/v0.5.0...v0.6.0) (2026-09-26)


### Features

* **ci:** publish a container image, and stop shipping a release nobody can download ([b14332b](https://github.com/Heliopsy/tix/commit/b14332b19d2fe5b4fac7c59daba2baf908015f06))
* **tui:** a project screen, and the definitions a project runs on ([00a014b](https://github.com/Heliopsy/tix/commit/00a014b9e99618b07b70b067273a884adf5c6341))
* **tui:** confirm before destroying, and answer a form instead of guessing ([9ce30a7](https://github.com/Heliopsy/tix/commit/9ce30a7b6518da03d487b978ecc441ce636678cb))
* **tui:** history, artifacts, restoring a task, and a directory to assign from ([d6eb189](https://github.com/Heliopsy/tix/commit/d6eb189fc670525829b61d96c487cc815df61f4d))
* **tui:** make the settings screen worth opening ([6fd7de6](https://github.com/Heliopsy/tix/commit/6fd7de610b711d7d7d9eaca73723a3b0f4d60c25))
* **tui:** offer a reader the views their permissions reach ([924d4a3](https://github.com/Heliopsy/tix/commit/924d4a3e618644587488579b60476530d3efe1c2))


### Bug Fixes

* **config:** honour the keys the documentation promises ([17e534c](https://github.com/Heliopsy/tix/commit/17e534cbe41bba04c8b9507415662b6da3b713be))
* **deps:** declare pflag, which a test now imports directly ([27735b7](https://github.com/Heliopsy/tix/commit/27735b7eee24bd64f142e4852e323895591a2d6b))
* **server:** take the live cursor before the connection is acknowledged ([6b7c445](https://github.com/Heliopsy/tix/commit/6b7c4453817a9f293f88dd1efc288213ead629f8))
* **sshd:** spend no capacity on a key that has not been identified ([d774242](https://github.com/Heliopsy/tix/commit/d774242356e8dd4549a06f32382be31ad6a17607))
* **web:** label the value a reader has to copy, and give the history its width ([bfe386d](https://github.com/Heliopsy/tix/commit/bfe386d1595d6927430d638427f6aff7d152c892))

## [0.5.0](https://github.com/Heliopsy/tix/compare/v0.4.0...v0.5.0) (2026-09-25)


### Features

* **demo:** seed a demo that can be signed in to, and an abandoned claim worth showing ([62c4eed](https://github.com/Heliopsy/tix/commit/62c4eed14a9ffd6c5ef2ef3edb4731ad4b226c24))
* seed a backlog worth looking at, and render durations for a reader ([457d1f0](https://github.com/Heliopsy/tix/commit/457d1f0f6ce70cd43ac5f13b2d1428bcb38c06c7))
* **web:** make the list usable, and stop the interface lying about state ([25d0995](https://github.com/Heliopsy/tix/commit/25d0995e4e7245c9f92a56ca94ab4ee822e7f2fd))
* **web:** rows settle towards the row that was acted on ([d325749](https://github.com/Heliopsy/tix/commit/d3257494e05bc4f3e41054ca989f70f77da759cf))


### Bug Fixes

* animate one tick, and stop sync losing its own state ([978801e](https://github.com/Heliopsy/tix/commit/978801e5be8aba452b1a0c66ebd58d8479fe2f3f))
* **cli:** one duration vocabulary, and a failed listing that writes nothing ([7e5a238](https://github.com/Heliopsy/tix/commit/7e5a23828edb73d93f55cb71ed0c78f5d80c2fa9))
* **demo:** spread completions the way work actually lands ([177ba42](https://github.com/Heliopsy/tix/commit/177ba420ed2c08cc7651505c5a3670b11069119c))
* **service:** refuse a filter term that names nothing ([25f4d5f](https://github.com/Heliopsy/tix/commit/25f4d5ff0bdf9d3b759c5337b09d795aa2b49172))
* **store:** record that a claim expired, and translate ALTER TABLE on postgres ([d4a6e43](https://github.com/Heliopsy/tix/commit/d4a6e43bef306351b170bc86e61390a987bfe696))
* **web:** a real pager, a column cookie that survives a new column, and a badge you can see ([10151cc](https://github.com/Heliopsy/tix/commit/10151ccadd9f85b6bf41f8de82f796c9983b4f33))
* **web:** keep the reader's place when a form re-renders its own page ([e4634b3](https://github.com/Heliopsy/tix/commit/e4634b3cc8e9d0df27472496e287048a5e373880))
* **web:** make a tenant's accent survive the dark scheme ([b0163e0](https://github.com/Heliopsy/tix/commit/b0163e07eb8e0fa3ba521e9c79ff9699a2a32a96))
* **web:** make the statistics filter a row, not two full-width dropdowns ([cb28b85](https://github.com/Heliopsy/tix/commit/cb28b85b819bfccf5a358d79f9263b83712bf604))
* **web:** name the listing for what it shows, and write durations the way they are read ([348d249](https://github.com/Heliopsy/tix/commit/348d24926f570ab6eefa6a13e89603550b5c46e4))
* **web:** swap only the ticked row, not the whole page ([cda1902](https://github.com/Heliopsy/tix/commit/cda1902d105c78fdc5366b774d1cb523324faefd))
* **web:** ticking a task no longer moves the page ([fdfa1d9](https://github.com/Heliopsy/tix/commit/fdfa1d995c8db14ec08d58463ac97ad7664ed1a1))

## [0.4.0](https://github.com/Heliopsy/tix/compare/v0.3.0...v0.4.0) (2026-09-25)


### Features

* **cli:** tix update, replacing this binary with a published release ([53a7d13](https://github.com/Heliopsy/tix/commit/53a7d13443e7f84ebda77cb523f969067dec7b7b))

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
