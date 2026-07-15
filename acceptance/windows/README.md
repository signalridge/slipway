# Native Windows acceptance assets

These scripts are the native Windows entry points used by the `windows-latest` CI matrix. Executable assets and workflow wiring are not W evidence by themselves; W is recorded only when one workflow execution successfully runs both scripts against the binary it built.

Build `slipway.exe`, then run both commands from the repository root:

```powershell
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File acceptance\windows\native-powershell.ps1 -SlipwayExe .\slipway.exe
cmd.exe /d /v:on /c acceptance\windows\native-cmd.cmd .\slipway.exe
```

Both modes create a disposable Git repository whose path contains spaces, Unicode, `%`, `!`, `&`, and `^`. They exercise:

- versioned `doctor` output and initial Orient;
- exact goal/answer argv containing spaces, quotes, Unicode, CRLF, `%`, `!`, `&`, and `^`;
- structured `answer-decision` resolution;
- Outcome file in both modes and Outcome stdin in PowerShell;
- stop and inputless ad-hoc resume through Slipway's rendered recovery command;
- strict CRLF source-envelope import;
- material reads, source refresh, current candidate ID, and structured `adopt` recovery;
- final status readback.

Windows PowerShell 5.1's legacy native-argument binder does not preserve every embedded quote. Both modes therefore start Slipway with `ProcessStartInfo` and CommandLineToArgvW-compatible escaping, testing the executable's actual Windows argv boundary.

`native-cmd.cmd` delegates JSON creation and assertions to the standard PowerShell available on supported Windows systems. In Cmd mode, every non-stdin Slipway invocation still crosses `cmd.exe /d /v:on` through a UTF-16LE `EncodedCommand`. Outcome stdin remains a PowerShell-only assertion; Cmd submits the equivalent Outcome through a file.

A failure throws or exits non-zero with `native Windows acceptance (...) failed`. Set `SLIPWAY_KEEP_WINDOWS_FIXTURE=1` only for local diagnosis; otherwise the disposable fixture is removed.

Each mode emits one `native Windows acceptance metadata:` JSON line before the fixture runs. It records OS and shell versions, source and checkout revisions, run URL, and SHA-256 digests for both assets and the tested binary. Retain that line with the completed job.

A Linux/macOS syntax scan or `GOOS=windows` build cannot establish W. A completed native run applies only to its recorded source, binary, and asset digests. Historical evidence records live under [`../evidence/windows/`](../evidence/windows/); the matrix must mark current W as not collected whenever the relevant source or asset digests differ.
