# Native Windows acceptance assets

These scripts are the native Windows entry points wired into the `windows-latest` CI matrix. Executable assets and workflow wiring are not W evidence by themselves: W is recorded only when one workflow execution successfully runs both scripts against the binary it built. The recorded execution below satisfies W for its recorded source, binary, and acceptance assets; workflow wiring alone remains insufficient for later changes.

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

A Linux/macOS syntax scan or `GOOS=windows` build cannot establish W. The recorded run below establishes W only for its recorded source revision, binary, and asset digests; later code or acceptance-asset changes require a new completed `windows-latest` job before the matrix can claim current W.

## Recorded W evidence

[Run 29197908671](https://github.com/signalridge/slipway/actions/runs/29197908671) / [Windows job 86664073429](https://github.com/signalridge/slipway/actions/runs/29197908671/job/86664073429) completed the native Windows Go suite and both acceptance modes.

- Source revision: `4c1741ae35b42d903fa1ccc4ec5ae32469aaca47`
- Checkout revision: `d1095684dbff536a792c19701ff3c65cb228fb78`
- OS: Microsoft Windows Server 2025 (`Microsoft Windows NT 10.0.26100.0`)
- Runner image: `win25-vs2026` (`20260628.158.1`)
- PowerShell: Desktop `5.1.26100.32995`
- cmd.exe: `Microsoft Windows [Version 10.0.26100.32995]`
- PowerShell asset SHA-256: `08d4c2c4d83c7c297b2d1a9a70d7ed70856399c129a9552ccf89f0d1147ae6ac`
- cmd asset SHA-256: `4dfa5d74cb69b17ed6b1192bd9e3fc82de6c66f89f800164c5380096d7b5425d`
- Tested binary SHA-256: `081524ac5a2742e3dc240c6d3ca8b0f7d1312990b96debfb8ebecde0e6643e0a`
- PowerShell result: `native Windows acceptance (PowerShell): ok`
- Cmd result: `native Windows acceptance (Cmd): ok`

This records W only for the source, binary, and assets identified above. A later relevant change needs a new completed collection.
