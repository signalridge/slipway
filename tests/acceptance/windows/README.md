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

Windows PowerShell 5.1's legacy native-argument binder does not preserve every
embedded quote. Both modes therefore start Slipway with `ProcessStartInfo` and
CommandLineToArgvW-compatible argument escaping, matching the executable's
actual Windows argv boundary instead of accepting the binder's lossy rewrite.

`native-cmd.cmd` intentionally delegates JSON creation and assertions to the stdlib PowerShell available on supported Windows systems. That PowerShell process is only the assertion driver: in `Cmd` mode, every non-stdin Slipway invocation crosses `cmd.exe /d /v:on` through a UTF-16LE `EncodedCommand`, including doctor, initial Orient, Outcome files, structured answer/adopt/resume argv, status/stop, source import, and the binary's rendered recovery command. The inner encoded PowerShell explicitly configures UTF-8 native input and output before invoking Slipway. Outcome stdin remains a PowerShell-mode-only assertion; Cmd mode submits the equivalent Outcome through an outcome-file argv that crosses `cmd.exe`.

A failure throws or exits non-zero with `native Windows acceptance (...) failed`. Set `SLIPWAY_KEEP_WINDOWS_FIXTURE=1` only for local diagnosis; otherwise fixtures are removed. Use only disposable test data.

Each mode emits one `native Windows acceptance metadata:` JSON line before the
fixture runs. It records the OS, runner image, PowerShell and cmd versions,
source and checkout revisions, run URL, and SHA-256 digests for the PowerShell
asset, cmd asset, and tested binary. Retain that line with the completed job.

A Linux/macOS syntax scan or `GOOS=windows` build cannot establish W. The CI workflow is a collector, not pre-recorded evidence: retain the completed `windows-latest` run URL, native OS and shell versions, binary revision, script digests, output, and evaluator notes before changing matrix status from `not collected`.
