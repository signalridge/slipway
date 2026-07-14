# Windows rendering and durability (non-normative)

> This is a non-normative platform guide. The [Chinese product contract](../zh/reference/product-contract.md) and [machine schema](machine-protocol.schema.json) are authoritative.

## Structured argv is authority

Recovery and pause responses expose `next.operation`, a workspace identity, and typed variants. Each variant has a complete `base_argv` and ordered inputs. Resolve in schema order by inserting each input flag and its exact, unquoted value immediately before the sole `--` separator when present, or appending it when absent. Never reconstruct a command from display prose and never substitute literal `<answer>` or `<file>` placeholders.

Slipway renders a complete resolved argv separately for POSIX, `cmd.exe`, and PowerShell. Rendering is for display/copy only and is not journaled. It must preserve spaces, quotes, Unicode, CR/LF, `%`, `!`, `&`, and `^` in root paths, Issue URLs, source/outcome files, answers, and recovery choices. Because `%` and `!` can expand in cmd, the renderer may use a PowerShell UTF-16LE `EncodedCommand` or an equivalent safe argv path. Native tests must capture the actual process argv under both `cmd.exe /v:on` and PowerShell; a Linux cross-build proves only compilation.

The CI matrix runs both native assets after building `slipway.exe`; the same commands can be run from a Developer Command Prompt or PowerShell:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tests\acceptance\windows\native-powershell.ps1 -SlipwayExe C:\path\slipway.exe
cmd.exe /d /v:on /c tests\acceptance\windows\native-cmd.cmd C:\path\slipway.exe
```

The scripts cover doctor, initial Orient, issue source import, Outcome file/stdin where supported, decision answer, ad-hoc resume, current-candidate keep/adopt recovery, and special argv. See the [acceptance matrix](../../tests/acceptance/README.md). Workflow wiring remains only a W collector. [Run 29197908671, Windows job 86664073429](https://github.com/signalridge/slipway/actions/runs/29197908671/job/86664073429) completed both native assets against source `4c1741ae35b42d903fa1ccc4ec5ae32469aaca47` and records W for that source, binary, and asset set. Later relevant changes require a new completed collection; syntax checks and cross-builds remain non-W evidence.

## Symbolic-link transaction boundary

Windows file transactions inspect symbolic links through anchored reparse-point handles, but recreating either a file or directory link can require a privilege that was not needed to inspect or move the existing object. Slipway therefore rejects every pre-existing Windows symbolic link with a typed error before the first transaction mutation. It never moves a link and then depends on symbolic-link creation privilege to roll it back. A non-skippable policy test proves that fail-closed decision without creating a link; native link fixtures supplement it when the runner can create them.

## Crash durability

Journal, lock, and projection files are flushed. Windows does not currently provide the same run-directory fsync guarantee used on Unix. Doctor reports `level:"file_fsync_only"`, `directory_sync:false`, and `limitation:"directory_fsync_unsupported"`. A successful file flush therefore does not claim equal crash durability for a newly created or renamed directory entry.

Run directories should be protected by the current user's ACL, but inherited ACLs, administrators, backup agents, malware, and other same-account processes remain outside that claim. Review ACLs and backups for sensitive repositories. Deleting a run directory removes recovery capability only; it is not secure erase, backup deletion, or key destruction.
