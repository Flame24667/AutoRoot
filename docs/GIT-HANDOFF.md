# Git handoff — AutoRoot A065F source PoC

## Intended scope

Publish as a **guarded SM-A065F PoC**, not universal Samsung automation.
README and test results must accompany source. Preserve lab evidence locally.
The successful operator test included explicit recovery from a launch failure.

## Files to review together

- Backend Go sources/tests, go.mod/go.sum when changed.
- Electron main/preload source (only modified files).
- Frontend source/styles/tests and package manifests/lockfiles when changed.
- Public setup/firmware-db.json metadata.
- Scripts: build-poc, start-poc, test-ui, fetch-samloader, automation-smoke.
- README, docs/ and .gitignore.

Do not stage unrelated .commandcode changes. Review the actual diff rather than
assuming every dirty file belongs to this implementation. No index changes,
commit or push are performed by this cleanup.

## Exclude from new source changes

Firmware ZIP/TAR/AP outputs, PIT and device bindings, runtime sessions/logs,
screenshots with identifiers, private catalog overrides, node_modules, tools
binaries, build/cache directories and local executables/APKs.
setup/firmware-db.local.json keeps the operator's absolute path and is ignored.
Test fixtures use synthetic serials; public documentation omits real identifiers.

## Existing upstream-tracked assets — manual cleanup before public push

Ignore rules do not remove these already-tracked files:

- Magisk-v30.7.apk
- backend/myapp-go.exe
- odin/Odin3.exe (legacy LFS pointer)
- frontend/dist/index.html and frontend/dist/assets/index-BgUxGWBT.js

They have **not** been deleted or removed from the index during cleanup.
Recommended source-PoC policy: remove them from tracking while retaining local
copies, provision trusted dependencies separately, and preserve license/source
information. The operator approved this policy, but this environment denied
writing .git/index.lock. Both attempts stopped before modifying the index;
the following step is therefore still pending in the operator's PowerShell.
Historical copies remain in Git history; changing history requires a distinct
decision.

frontend/dist/index.html has a generated/whitespace-only local diff; it is not
part of the static PoC source handoff. Exclude it from a source commit.

### Manual untracking (not deletion from disk)

Run from the modified project root, not the original checkout. First check
`git diff --cached --name-status` and preserve any unrelated staged work. Then:

```powershell
git rm --cached -- Magisk-v30.7.apk backend/myapp-go.exe odin/Odin3.exe frontend/dist/index.html frontend/dist/assets/index-BgUxGWBT.js
git diff --cached --name-status
```

Expect only those five staged deletions at this step. Their working files must
still exist and be ignored. Do not use plain `git rm`, `git clean`, or force flags
to resolve an unexpected result. This command does not commit, push or alter
history. New clones must provision the trusted APK/engine and build the backend
and frontend using the README; installer packaging remains unvalidated.

Before a later commit, check `git var GIT_AUTHOR_IDENT` and
`git var GIT_COMMITTER_IDENT` locally for the operator's desired Git identity.
Do not share their output if it contains a private email. No AI/bot co-author
trailer is required or added. Existing historical authors are not rewritten.

## Verification before commit

Run the README's offline test/build commands. Check git diff --check on intended
source/docs, parse the public JSON, and inspect staged file names/diff before
committing. Use a dedicated PoC branch only after branch/commit authorization.

The dependency vulnerability report has not been cleared by this cleanup.
Do not claim a security certification; audit npm dependencies separately and
avoid blind npm audit fix upgrades that could invalidate the tested UI adapter.
No clean-room install or package signing test has been completed.

## Deferred work

Firmware Manager/network source verification, A045F support, better transfer
progress and transient UI-dump handling. Do not add them to the tested-feature
list or block this PoC report waiting for future features.
