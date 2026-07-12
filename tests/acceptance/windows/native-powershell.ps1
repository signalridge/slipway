[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$SlipwayExe,

    [ValidateSet('PowerShell', 'Cmd')]
    [string]$Mode = 'PowerShell'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
# Windows PowerShell 5.1 reads BOM-less scripts through the legacy code page.
# Keep this source ASCII and make every native stream/probe explicitly UTF-8.
$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
$OutputEncoding = $Utf8NoBom
[Console]::InputEncoding = $Utf8NoBom
[Console]::OutputEncoding = $Utf8NoBom
$UnicodeProbe = [char]0x754C

function Fail([string]$Message) {
    throw "native Windows acceptance ($Mode) failed: $Message"
}

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) { Fail $Message }
}

function Assert-TextEqual {
    param(
        [AllowEmptyString()]
        [string]$Actual,

        [AllowEmptyString()]
        [string]$Expected,

        [string]$Message
    )
    if ([string]::Equals($Actual, $Expected, [StringComparison]::Ordinal)) { return }

    $limit = [Math]::Min($Actual.Length, $Expected.Length)
    $difference = $limit
    for ($index = 0; $index -lt $limit; $index++) {
        if ($Actual[$index] -cne $Expected[$index]) {
            $difference = $index
            break
        }
    }
    $actualUnit = if ($difference -lt $Actual.Length) { 'U+{0:X4}' -f [int]$Actual[$difference] } else { '<end>' }
    $expectedUnit = if ($difference -lt $Expected.Length) { 'U+{0:X4}' -f [int]$Expected[$difference] } else { '<end>' }
    $actualJson = ConvertTo-Json -InputObject $Actual -Compress
    $expectedJson = ConvertTo-Json -InputObject $Expected -Compress
    Fail "$Message; first_difference=$difference expected_unit=$expectedUnit actual_unit=$actualUnit expected_length=$($Expected.Length) actual_length=$($Actual.Length) expected=$expectedJson actual=$actualJson"
}

function Write-Utf8([string]$Path, [string]$Text) {
    [System.IO.File]::WriteAllText($Path, $Text, $script:Utf8NoBom)
}

function Get-Sha256([string]$Path) {
    $stream = [System.IO.File]::OpenRead($Path)
    $sha256 = $null
    try {
        $sha256 = [System.Security.Cryptography.SHA256]::Create()
        $digest = $sha256.ComputeHash($stream)
        return ([BitConverter]::ToString($digest)).Replace('-', '').ToLowerInvariant()
    }
    finally {
        if ($null -ne $sha256) { $sha256.Dispose() }
        $stream.Dispose()
    }
}

function Join-NativeOutput($Output) {
    return (($Output | ForEach-Object { $_.ToString() }) -join [Environment]::NewLine)
}

function ConvertTo-WindowsCommandLineArgument {
    param(
        [AllowEmptyString()]
        [string]$Value
    )
    if ($Value.Length -eq 0) { return '""' }

    $hasSpace = ($Value.IndexOf([char]0x20) -ge 0) -or ($Value.IndexOf([char]0x09) -ge 0)
    $builder = New-Object System.Text.StringBuilder
    if ($hasSpace) { [void]$builder.Append([char]0x22) }

    $backslashes = 0
    foreach ($character in $Value.ToCharArray()) {
        if ($character -eq [char]0x5c) {
            $backslashes++
            [void]$builder.Append($character)
            continue
        }
        if ($character -eq [char]0x22) {
            for ($index = 0; $index -lt $backslashes; $index++) {
                [void]$builder.Append([char]0x5c)
            }
            [void]$builder.Append([char]0x5c)
            [void]$builder.Append($character)
            $backslashes = 0
            continue
        }
        $backslashes = 0
        [void]$builder.Append($character)
    }

    if ($hasSpace) {
        for ($index = 0; $index -lt $backslashes; $index++) {
            [void]$builder.Append([char]0x5c)
        }
        [void]$builder.Append([char]0x22)
    }
    return $builder.ToString()
}

function Join-WindowsCommandLine([string[]]$Values) {
    $escaped = @(
        $Values | ForEach-Object {
            ConvertTo-WindowsCommandLineArgument -Value ([string]$_)
        }
    )
    return ($escaped -join ' ')
}

function ConvertTo-PowerShellUtf8Expression([string]$Value) {
    $encoded = [Convert]::ToBase64String($script:Utf8NoBom.GetBytes($Value))
    return "[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('$encoded'))"
}

function Invoke-ExactNativeProcess {
    param(
        [string]$FileName,
        [string[]]$CommandArgs,
        [string]$StdinText,
        [switch]$UseStdin
    )
    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.FileName = $FileName
    $startInfo.Arguments = Join-WindowsCommandLine -Values $CommandArgs
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $startInfo.StandardOutputEncoding = $script:Utf8NoBom
    $startInfo.StandardErrorEncoding = $script:Utf8NoBom
    if ($UseStdin) {
        # Windows PowerShell 5.1 runs on .NET Framework, whose redirected
        # stdin writer inherits Console.InputEncoding configured above.
        $startInfo.RedirectStandardInput = $true
    }

    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $startInfo
    try {
        if (-not $process.Start()) { Fail "could not start native process: $FileName" }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if ($UseStdin) {
            $process.StandardInput.Write($StdinText)
            $process.StandardInput.Close()
        }
        $process.WaitForExit()
        return [pscustomobject]@{
            ExitCode = $process.ExitCode
            Stdout = $stdoutTask.Result
            Stderr = $stderrTask.Result
        }
    } finally {
        $process.Dispose()
    }
}

function Invoke-SlipwayDirect {
    param(
        [string[]]$CommandArgs,
        [string]$StdinText,
        [switch]$UseStdin
    )
    if ($Mode -eq 'Cmd') {
        Fail 'Cmd mode attempted to bypass cmd.exe for a Slipway invocation'
    }
    $result = Invoke-ExactNativeProcess -FileName $script:Exe -CommandArgs $CommandArgs -StdinText $StdinText -UseStdin:$UseStdin
    $combined = $result.Stdout
    if ($result.Stderr.Length -gt 0) {
        $combined += [Environment]::NewLine + $result.Stderr
    }
    if ($result.ExitCode -ne 0) {
        Fail "command exited $($result.ExitCode): $($CommandArgs -join ' '); output: $combined"
    }
    return $combined
}

function Invoke-SlipwayViaCmd {
    param([string[]]$CommandArgs)
    $argumentLine = Join-WindowsCommandLine -Values $CommandArgs
    $parts = New-Object System.Collections.Generic.List[string]
    $parts.Add('$ErrorActionPreference = ''Stop'';')
    $parts.Add('$innerUtf8NoBom = New-Object System.Text.UTF8Encoding($false);')
    $parts.Add('$OutputEncoding = $innerUtf8NoBom;')
    $parts.Add('[Console]::InputEncoding = $innerUtf8NoBom;')
    $parts.Add('[Console]::OutputEncoding = $innerUtf8NoBom;')
    $parts.Add('$startInfo = New-Object System.Diagnostics.ProcessStartInfo;')
    $parts.Add('$startInfo.UseShellExecute = $false;')
    $parts.Add('$startInfo.CreateNoWindow = $true;')
    $parts.Add('$startInfo.FileName = ' + (ConvertTo-PowerShellUtf8Expression $script:Exe) + ';')
    $parts.Add('$startInfo.Arguments = ' + (ConvertTo-PowerShellUtf8Expression $argumentLine) + ';')
    $parts.Add('$startInfo.RedirectStandardOutput = $true;')
    $parts.Add('$startInfo.RedirectStandardError = $true;')
    $parts.Add('$startInfo.StandardOutputEncoding = $innerUtf8NoBom;')
    $parts.Add('$startInfo.StandardErrorEncoding = $innerUtf8NoBom;')
    $parts.Add('$process = New-Object System.Diagnostics.Process;')
    $parts.Add('$process.StartInfo = $startInfo;')
    $parts.Add('if (-not $process.Start()) { exit 1 };')
    $parts.Add('$stdoutTask = $process.StandardOutput.ReadToEndAsync();')
    $parts.Add('$stderrTask = $process.StandardError.ReadToEndAsync();')
    $parts.Add('$process.WaitForExit();')
    $parts.Add('$stdout = $stdoutTask.Result;')
    $parts.Add('$stderr = $stderrTask.Result;')
    $parts.Add('$exitCode = $process.ExitCode;')
    $parts.Add('$process.Dispose();')
    $parts.Add('[Console]::Out.Write($stdout);')
    $parts.Add('[Console]::Error.Write($stderr);')
    $parts.Add('exit $exitCode')
    $encoded = [Convert]::ToBase64String(
        [Text.Encoding]::Unicode.GetBytes(($parts -join ' '))
    )
    $safeCommand = "powershell.exe -NoLogo -NoProfile -NonInteractive -EncodedCommand $encoded"
    $output = & $env:ComSpec '/d' '/v:on' '/s' '/c' $safeCommand 2>&1
    $exitCode = $LASTEXITCODE
    $text = Join-NativeOutput $output
    if ($exitCode -ne 0) {
        Fail "cmd.exe /v:on invocation exited $exitCode; output: $text"
    }
    return $text
}

function Invoke-ResolvedArgv {
    param([string[]]$CommandArgs)
    if ($Mode -eq 'Cmd') {
        return Invoke-SlipwayViaCmd -CommandArgs $CommandArgs
    }
    return Invoke-SlipwayDirect -CommandArgs $CommandArgs
}

function Invoke-NextVariant {
    param(
        $Next,
        [string]$VariantId,
        [hashtable]$InputValues
    )
    $matches = @($Next.variants | Where-Object { $_.id -eq $VariantId })
    Assert-True ($matches.Count -eq 1) "expected exactly one next variant '$VariantId'"
    $variant = $matches[0]
    Assert-True ($variant.base_argv[0] -eq 'slipway') "variant executable must be slipway"
    Assert-True (-not (($variant.base_argv | ConvertTo-Json -Compress) -match '<answer>|<file>')) "variant contains a placeholder"

    $resolved = New-Object System.Collections.Generic.List[string]
    for ($index = 1; $index -lt $variant.base_argv.Count; $index++) {
        $resolved.Add([string]$variant.base_argv[$index])
    }
    foreach ($input in $variant.inputs) {
        $inputName = [string]$input.name
        Assert-True ($InputValues.ContainsKey($inputName)) "missing typed value for $inputName"
        $resolved.Add([string]$input.flag)
        $resolved.Add([string]$InputValues[$inputName])
    }
    return Invoke-ResolvedArgv -CommandArgs $resolved.ToArray()
}

function New-Outcome {
    param(
        [string]$ActionId,
        [string]$ActionKind,
        [string]$Status,
        [string]$Summary,
        $Pause,
        $Suggestions
    )
    return [ordered]@{
        contract_version = 1
        action_id = $ActionId
        action_kind = $ActionKind
        status = $Status
        summary = $Summary
        observations = @()
        known_issues = @()
        suggested_actions = @($Suggestions)
        pause = $Pause
        implementation = $null
        review = $null
    }
}

function New-SourceEnvelope([string]$RequirementText, [string]$UpdatedAt) {
    $body = @(
        '<!-- slipway-level: change/v1 -->',
        '',
        '## Outcome',
        'Exercise native Windows source and recovery.',
        '',
        '## Requirements',
        $RequirementText,
        '',
        '## Acceptance examples',
        'Structured recovery preserves the selected candidate.',
        '',
        '## Constraints',
        'No network and no real user data.',
        '',
        '## Non-goals',
        'This script is not live GitHub evidence.',
        '',
        '## Implementation checklist',
        '- [ ] Native fixture only',
        ''
    ) -join "`r`n"
    return [ordered]@{
        source_version = 1
        provider = 'github'
        host = 'github.com'
        repository_id = 'R_windowsAcceptanceRepository'
        issue_id = 'I_windowsAcceptanceIssue'
        issue_number = 434
        canonical_url = 'https://github.com/example/windows-acceptance/issues/434'
        updated_at = $UpdatedAt
        fetched_at = '2026-07-12T10:01:00Z'
        title = '[Change] Native Windows acceptance'
        body = $body
        labels = @('level:change', 'kind:maintenance')
    }
}

$resolvedExe = (Resolve-Path -LiteralPath $SlipwayExe).Path
Assert-True (Test-Path -LiteralPath $resolvedExe -PathType Leaf) "SlipwayExe is not a file: $SlipwayExe"
Assert-True ($null -ne (Get-Command git.exe -ErrorAction SilentlyContinue)) 'git.exe is required'
Assert-True ($null -ne $env:ComSpec) 'COMSPEC is required'

$revision = [Environment]::GetEnvironmentVariable('GITHUB_SHA')
$serverUrl = [Environment]::GetEnvironmentVariable('GITHUB_SERVER_URL')
$repositoryName = [Environment]::GetEnvironmentVariable('GITHUB_REPOSITORY')
$runId = [Environment]::GetEnvironmentVariable('GITHUB_RUN_ID')
$eventPath = [Environment]::GetEnvironmentVariable('GITHUB_EVENT_PATH')
$runUrl = 'local'
if (-not [string]::IsNullOrWhiteSpace($serverUrl) -and
    -not [string]::IsNullOrWhiteSpace($repositoryName) -and
    -not [string]::IsNullOrWhiteSpace($runId)) {
    $runUrl = "$serverUrl/$repositoryName/actions/runs/$runId"
}
if ([string]::IsNullOrWhiteSpace($revision)) { $revision = 'local' }
$sourceRevision = $revision
if (-not [string]::IsNullOrWhiteSpace($eventPath) -and (Test-Path -LiteralPath $eventPath -PathType Leaf)) {
    $event = [System.IO.File]::ReadAllText($eventPath, [Text.Encoding]::UTF8) | ConvertFrom-Json
    if (($event.PSObject.Properties.Name -contains 'pull_request') -and $null -ne $event.pull_request) {
        $sourceRevision = [string]$event.pull_request.head.sha
    }
}
$cmdVersionOutput = & $env:ComSpec '/d' '/c' 'ver' 2>&1
if ($LASTEXITCODE -ne 0) { Fail 'cmd.exe version probe failed' }
$cmdAsset = Join-Path $PSScriptRoot 'native-cmd.cmd'
$collectorMetadata = [ordered]@{
    evidence_version = 1
    mode = $Mode
    os_version = [Environment]::OSVersion.VersionString
    runner_image = [Environment]::GetEnvironmentVariable('ImageOS')
    runner_image_version = [Environment]::GetEnvironmentVariable('ImageVersion')
    powershell_edition = [string]$PSVersionTable.PSEdition
    powershell_version = $PSVersionTable.PSVersion.ToString()
    cmd_version = (Join-NativeOutput $cmdVersionOutput).Trim()
    source_revision = $sourceRevision
    checkout_revision = $revision
    script_sha256 = (Get-Sha256 $PSCommandPath)
    cmd_asset_sha256 = (Get-Sha256 $cmdAsset)
    binary_sha256 = (Get-Sha256 $resolvedExe)
    run_url = $runUrl
}
Write-Output ('native Windows acceptance metadata: ' + ($collectorMetadata | ConvertTo-Json -Compress))

$tempName = 'slipway native % ! & ^ ' + $UnicodeProbe + ' ' + [Guid]::NewGuid().ToString('N')
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) $tempName
$repo = Join-Path $tempRoot ('repository with spaces % ! & ^ ' + $UnicodeProbe)
$tools = Join-Path $tempRoot 'tools'
$script:Exe = Join-Path $tools 'slipway.exe'

try {
    New-Item -ItemType Directory -Path $repo -Force | Out-Null
    New-Item -ItemType Directory -Path $tools -Force | Out-Null
    Copy-Item -LiteralPath $resolvedExe -Destination $script:Exe -Force
    $env:PATH = $tools + [IO.Path]::PathSeparator + $env:PATH

    & git.exe -C $repo init -q
    if ($LASTEXITCODE -ne 0) { Fail 'git init failed' }
    & git.exe -C $repo config user.email acceptance@example.invalid
    & git.exe -C $repo config user.name 'Slipway Windows Acceptance'
    Write-Utf8 (Join-Path $repo 'README.md') "# Windows acceptance`r`n"
    & git.exe -C $repo add README.md
    & git.exe -C $repo commit -qm initial
    if ($LASTEXITCODE -ne 0) { Fail 'initial git commit failed' }

    $doctor = (Invoke-ResolvedArgv -CommandArgs @('doctor', '--root', $repo, '--json')) | ConvertFrom-Json
    Assert-True ($doctor.contract_version -eq 1) 'doctor did not return contract_version 1'
    Assert-True ($null -ne $doctor.checks) 'doctor checks are missing'

    $goal = "spaces `"double`" and 'single' ${UnicodeProbe}`r`npercent % bang ! amp & caret ^"
    $start = (Invoke-ResolvedArgv -CommandArgs @('run', $goal, '--root', $repo, '--budget', '12', '--json')) | ConvertFrom-Json
    Assert-True ($start.kind -eq 'orient') 'ad-hoc start did not return Orient'
    Assert-TextEqual -Actual ([string]$start.goal) -Expected $goal -Message 'special-character goal did not preserve exact text'
    $startStatus = (Invoke-ResolvedArgv -CommandArgs @('status', $start.run_id, '--root', $repo, '--json')) | ConvertFrom-Json
    Assert-TextEqual -Actual ([string]$startStatus.goal) -Expected $goal -Message 'journaled goal did not preserve exact special characters or CRLF'

    $pause = [ordered]@{
        reason = 'decision_required'
        question = 'Choose the exact Windows value.'
        destructive_request = $null
    }
    $decisionOutcome = New-Outcome -ActionId $start.action_id -ActionKind $start.kind -Status 'needs_input' -Summary 'One Windows decision is required.' -Pause $pause -Suggestions @()
    $outcomePath = Join-Path $tempRoot ('outcome file % ! & ^ ' + $UnicodeProbe + '.json')
    Write-Utf8 $outcomePath (($decisionOutcome | ConvertTo-Json -Depth 20 -Compress) + "`r`n")
    $paused = (Invoke-ResolvedArgv -CommandArgs @('run', 'submit', '--root', $repo, '--run', $start.run_id, '--action', $start.action_id, '--outcome-file', $outcomePath)) | ConvertFrom-Json
    Assert-True ($paused.state -eq 'paused') 'Outcome file did not pause the Run'
    Assert-True ($paused.next.operation -eq 'answer') 'decision pause did not return structured answer next'

    $specialAnswer = "answer spaces `"double`" and 'single' ${UnicodeProbe}`r`npercent % bang ! amp & caret ^"
    $orientedText = Invoke-NextVariant -Next $paused.next -VariantId 'answer-decision' -InputValues @{ text = $specialAnswer }
    $oriented = $orientedText | ConvertFrom-Json
    Assert-True ($oriented.kind -eq 'orient') 'structured answer did not fresh-Orient'
    $answeredStatus = (Invoke-ResolvedArgv -CommandArgs @('status', $start.run_id, '--root', $repo, '--json')) | ConvertFrom-Json
    Assert-TextEqual -Actual ([string]$answeredStatus.answers[-1].text) -Expected $specialAnswer -Message 'journaled answer did not preserve exact special characters or CRLF'
    $normalizedAnswer = $specialAnswer.Replace("`r`n", "`n")
    $indentedAnswer = '  ' + $normalizedAnswer.Replace("`n", "`n  ")
    Assert-True ($oriented.context.Contains($indentedAnswer)) 'bounded context did not contain the normalized and structurally indented special-character answer'

    $implementSuggestion = [ordered]@{ kind = 'implement'; brief = 'Exercise Windows Outcome transport.' }
    $orientOutcome = New-Outcome -ActionId $oriented.action_id -ActionKind $oriented.kind -Status 'completed' -Summary 'Windows argv observed.' -Pause $null -Suggestions @($implementSuggestion)
    $orientJson = ($orientOutcome | ConvertTo-Json -Depth 20 -Compress) + "`r`n"
    if ($Mode -eq 'PowerShell') {
        # stdin transport is intentionally PowerShell-only. Cmd mode exercises
        # the same Outcome through an outcome-file argv that crosses cmd.exe.
        $implementedText = Invoke-SlipwayDirect -CommandArgs @('run', 'submit', '--root', $repo, '--run', $start.run_id, '--action', $oriented.action_id, '--outcome-stdin') -StdinText $orientJson -UseStdin
    } else {
        $secondOutcomePath = Join-Path $tempRoot 'cmd outcome file.json'
        Write-Utf8 $secondOutcomePath $orientJson
        $implementedText = Invoke-ResolvedArgv -CommandArgs @('run', 'submit', '--root', $repo, '--run', $start.run_id, '--action', $oriented.action_id, '--outcome-file', $secondOutcomePath)
    }
    $implemented = $implementedText | ConvertFrom-Json
    Assert-True ($implemented.kind -eq 'implement') 'Outcome transport did not return Implement'

    $stopDisplay = Invoke-ResolvedArgv -CommandArgs @('stop', $start.run_id, '--root', $repo)
    $resumeLines = @($stopDisplay -split "`r?`n" | Where-Object { $_ -like '- resume-ad-hoc:*' })
    Assert-True ($resumeLines.Count -eq 1) 'human stop output lacks one resume-ad-hoc command'
    $rendered = $resumeLines[0].Substring($resumeLines[0].IndexOf(':') + 1).Trim()
    Assert-True ($rendered -match 'EncodedCommand') 'unsafe %/! root was not rendered through an encoded command'
    Assert-True (-not ($rendered -match '<answer>|<file>')) 'human recovery command contains a placeholder'

    if ($Mode -eq 'Cmd') {
        $resumeOutput = & $env:ComSpec '/d' '/v:on' '/s' '/c' $rendered 2>&1
    } else {
        $renderedParts = $rendered -split ' '
        $resumeOutput = & $renderedParts[0] @($renderedParts[1..($renderedParts.Count - 1)]) 2>&1
    }
    $resumeExit = $LASTEXITCODE
    $resumeText = Join-NativeOutput $resumeOutput
    if ($resumeExit -ne 0) { Fail "rendered recovery command failed: $resumeText" }
    $resumed = $resumeText | ConvertFrom-Json
    Assert-True ($resumed.kind -eq 'orient') 'rendered recovery command did not return fresh Orient'

    $sourcePath = Join-Path $tempRoot ('source file % ! & ^ ' + $UnicodeProbe + '.json')
    $sourceInitial = New-SourceEnvelope -RequirementText 'Keep the initial Windows requirement.' -UpdatedAt '2026-07-12T09:00:00Z'
    Write-Utf8 $sourcePath (($sourceInitial | ConvertTo-Json -Depth 20 -Compress) + "`r`n")
    $sourceStart = (Invoke-ResolvedArgv -CommandArgs @('run', 'issue-bound Windows', '--root', $repo, '--source-file', $sourcePath, '--budget', '8', '--json')) | ConvertFrom-Json
    Assert-True ($sourceStart.kind -eq 'orient') 'source-file start did not Orient'
    Assert-True ($sourceStart.source.kind -eq 'change_issue') 'source identity is missing'
    Assert-True ($sourceStart.requirements.requirements_markdown -match 'initial Windows') 'accepted Requirements missing from Action'

    $sourceAmended = New-SourceEnvelope -RequirementText 'Keep the materially amended Windows requirement.' -UpdatedAt '2026-07-12T10:00:00Z'
    Write-Utf8 $sourcePath (($sourceAmended | ConvertTo-Json -Depth 20 -Compress) + "`r`n")
    $candidate = (Invoke-ResolvedArgv -CommandArgs @('run', 'resume', $sourceStart.run_id, '--root', $repo, '--source-file', $sourcePath, '--budget', '20')) | ConvertFrom-Json
    Assert-True ($candidate.state -eq 'paused') 'material source refresh did not pause'
    Assert-True ($candidate.budget_applied -eq $false) 'candidate creation applied replacement budget'
    Assert-True ($candidate.source_candidate.candidate_id.Length -gt 0) 'current candidate ID missing'
    $candidateId = [string]$candidate.source_candidate.candidate_id

    $adoptedText = Invoke-NextVariant -Next $candidate.next -VariantId 'adopt' -InputValues @{}
    $adopted = $adoptedText | ConvertFrom-Json
    Assert-True ($adopted.kind -eq 'orient') 'current-candidate adopt did not Orient'
    Assert-True ($adopted.requirements.requirements_markdown -match 'materially amended Windows') 'candidate adoption did not update Requirements'

    $status = (Invoke-ResolvedArgv -CommandArgs @('status', $sourceStart.run_id, '--root', $repo, '--json')) | ConvertFrom-Json
    Assert-True (-not ($status.PSObject.Properties.Name -contains 'source_candidate')) 'candidate remained current after adopt'
    Assert-True ($status.last_source_choice.candidate_id -eq $candidateId) 'status lost candidate choice identity'
    Assert-True ($status.last_source_choice.choice -eq 'adopt') 'status lost candidate choice'

    Write-Output "native Windows acceptance ($Mode): ok"
} finally {
    if ((Test-Path -LiteralPath $tempRoot) -and ($env:SLIPWAY_KEEP_WINDOWS_FIXTURE -ne '1')) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
