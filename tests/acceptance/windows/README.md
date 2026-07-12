# Native Windows acceptance assets

These scripts are the native Windows entry points wired into the `windows-latest` CI matrix. Executable assets and workflow wiring are not W evidence by themselves: W is recorded only when one workflow execution successfully runs both scripts against the binary it built. No native W execution is claimed by this local change.

Build `slipway.exe`, then run both from the repository root:

```powershell
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File tests\acceptance\windows\native-powershell.ps1 -SlipwayExe .\slipway.exe
cmd.exe /d /v:on /c tests\acceptance\windows\native-cmd.cmd .\slipway.exe
```

Both modes create a disposable Git repository whose path contains spaces, Unicode, `%`, `!`, `&`, and `^`. They exercise:

- versioned `doctor` output and initial Orient;
- exact goal/answer argv containing spaces, double/single quotes, Unicode, CRLF, `%`, `!`, `&`, and `^`;
- structured `answer-decision` resolution;
- Outcome file in both modes and Outcome stdin in PowerShell;
- stop plus inputless ad-hoc resume using Slipway's displayed encoded recovery command;
- strict CRLF source envelope import;
- material source refresh, current candidate ID, and structured `adopt` recovery;
- final status readback.

`native-cmd.cmd` intentionally delegates JSON creation/assertions to the stdlib PowerShell available on supported Windows systems. In `Cmd` mode, the driver launches resolved special-character argv and the rendered recovery command through `cmd.exe /d /v:on`; it is not merely the PowerShell mode under another filename.

A failure throws or exits non-zero with `native Windows acceptance (...) failed`. Set `SLIPWAY_KEEP_WINDOWS_FIXTURE=1` only for local diagnosis; otherwise fixtures are removed. Use only disposable test data.

A Linux/macOS syntax scan or `GOOS=windows` build cannot establish W. The CI workflow is a collector, not pre-recorded evidence: retain the completed `windows-latest` run URL, native OS and shell versions, binary revision, script digests, output, and evaluator notes before changing matrix status from `not collected`.
