# Tasks

## 1. Spec

- [x] 1.1 Delta spec for the connect screen's tabs, credential step, SSH tab and help links

## 2. Server

- [ ] 2.1 Read `tab`, `install`, `client` and `skill`; build each tab's href and selection in connect.go
- [ ] 2.2 onboard: `tix login --tenant`, `tix token rotate` and the ssh command snippets
- [ ] 2.3 Rework templates/connect.html into a credential step, the tab strip and five panels, each with a
  help link

## 3. Script and style

- [ ] 3.1 tasktabs.js: link tabs, `data-tab-param` address rewrite without storage, nested boxes
- [ ] 3.2 connect.js keeps only the token filler
- [ ] 3.3 610-connect-agent.css: the strip as links, the step numbers

## 4. Tests

- [ ] 4.1 Web: each tab renders with its commands; links work without scripting; unknown values fall back
- [ ] 4.2 jstest: address rewrite, nested boxes, link click handling
- [ ] 4.3 Bulgarian strings

## 5. Docs

- [ ] 5.1 docs/web-ui.md and docs/agents.md describe the tabs and the SSH commands
