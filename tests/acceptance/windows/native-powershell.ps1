[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$SlipwayExe,

    [ValidateSet('PowerShell', 'Cmd')]
    [string]$Mode = 'PowerShell'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)

function Fail([string]$Message) {
    throw "native Windows acceptance ($Mode) failed: $Message"
}

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) { Fail $Message }
}

function Write-Utf8([string]$Path, [string]$Text) {
    [System.IO.File]::WriteAllText($Path, $Text, $script:Utf8NoBom)
}

function Join-NativeOutput($Output) {
    return (($Output | ForEach-Object { $_.ToString() }) -join [Environment]::NewLine)
}

function Invoke-SlipwayDirect {
    param(
        [string[]]$CommandArgs,
        [string]$StdinText,
        [switch]$UseStdin
    )
    if ($UseStdin) {
        $output = $StdinText | & $script:Exe @CommandArgs 2>&1
    } else {
        $output = & $script:Exe @CommandArgs 2>&1
    }
    $exitCode = $LASTEXITCODE
    $text = Join-NativeOutput $output
    if ($exitCode -ne 0) {
        Fail "command exited $exitCode: $($CommandArgs -join ' '); output: $text"
    }
    return $text
}

function Quote-PowerShellLiteral([string]$Value) {
    return "'" + $Value.Replace("'", "''") + "'"
}

function Invoke-SlipwayViaCmd {
    param([string[]]$CommandArgs)
    $parts = New-Object System.Collections.Generic.List[string]
    $parts.Add('&')
    $parts.Add((Quote-PowerShellLiteral $script:Exe))
    foreach ($value in $CommandArgs) {
        $parts.Add((Quote-PowerShellLiteral $value))
    }
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
        [string]$Status,
        [string]$Summary,
        $Pause,
        $Suggestions
    )
    return [ordered]@{
        contract_version = 1
        action_id = $ActionId
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

$tempName = 'slipway native % ! & ^ 界 ' + [Guid]::NewGuid().ToString('N')
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) $tempName
$repo = Join-Path $tempRoot 'repository with spaces % ! & ^ 界'
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

    $doctor = (Invoke-SlipwayDirect -CommandArgs @('doctor', '--root', $repo, '--json')) | ConvertFrom-Json
    Assert-True ($doctor.contract_version -eq 1) 'doctor did not return contract_version 1'
    Assert-True ($null -ne $doctor.checks) 'doctor checks are missing'

    $goal = "spaces `"double`" and 'single' 界`r`npercent % bang ! amp & caret ^"
    $start = (Invoke-SlipwayDirect -CommandArgs @('run', $goal, '--root', $repo, '--budget', '12', '--json')) | ConvertFrom-Json
    Assert-True ($start.kind -eq 'orient') 'ad-hoc start did not return Orient'
    Assert-True ($start.goal -eq $goal) 'special-character goal did not preserve exact text'

    $pause = [ordered]@{
        reason = 'decision_required'
        question = 'Choose the exact Windows value.'
        destructive_request = $null
    }
    $decisionOutcome = New-Outcome -ActionId $start.action_id -Status 'needs_input' -Summary 'One Windows decision is required.' -Pause $pause -Suggestions @()
    $outcomePath = Join-Path $tempRoot 'outcome file % ! & ^ 界.json'
    Write-Utf8 $outcomePath (($decisionOutcome | ConvertTo-Json -Depth 20 -Compress) + "`r`n")
    $paused = (Invoke-ResolvedArgv -CommandArgs @('run', 'submit', '--root', $repo, '--run', $start.run_id, '--action', $start.action_id, '--outcome-file', $outcomePath)) | ConvertFrom-Json
    Assert-True ($paused.state -eq 'paused') 'Outcome file did not pause the Run'
    Assert-True ($paused.next.operation -eq 'answer') 'decision pause did not return structured answer next'

    $specialAnswer = "answer spaces `"double`" and 'single' 界`r`npercent % bang ! amp & caret ^"
    $orientedText = Invoke-NextVariant -Next $paused.next -VariantId 'answer-decision' -InputValues @{ text = $specialAnswer }
    $oriented = $orientedText | ConvertFrom-Json
    Assert-True ($oriented.kind -eq 'orient') 'structured answer did not fresh-Orient'
    $answeredStatus = (Invoke-SlipwayDirect -CommandArgs @('status', $start.run_id, '--root', $repo, '--json')) | ConvertFrom-Json
    Assert-True ($answeredStatus.answers[-1].text -eq $specialAnswer) 'journaled answer did not preserve exact special characters or CRLF'
    $normalizedAnswer = $specialAnswer.Replace("`r`n", "`n")
    Assert-True ($oriented.context.Contains($normalizedAnswer)) 'bounded context did not contain the normalized special-character answer'

    $implementSuggestion = [ordered]@{ kind = 'implement'; brief = 'Exercise Windows Outcome transport.' }
    $orientOutcome = New-Outcome -ActionId $oriented.action_id -Status 'completed' -Summary 'Windows argv observed.' -Pause $null -Suggestions @($implementSuggestion)
    $orientJson = ($orientOutcome | ConvertTo-Json -Depth 20 -Compress) + "`r`n"
    if ($Mode -eq 'PowerShell') {
        $implementedText = Invoke-SlipwayDirect -CommandArgs @('run', 'submit', '--root', $repo, '--run', $start.run_id, '--action', $oriented.action_id, '--outcome-stdin') -StdinText $orientJson -UseStdin
    } else {
        $secondOutcomePath = Join-Path $tempRoot 'cmd outcome file.json'
        Write-Utf8 $secondOutcomePath $orientJson
        $implementedText = Invoke-SlipwayViaCmd -CommandArgs @('run', 'submit', '--root', $repo, '--run', $start.run_id, '--action', $oriented.action_id, '--outcome-file', $secondOutcomePath)
    }
    $implemented = $implementedText | ConvertFrom-Json
    Assert-True ($implemented.kind -eq 'implement') 'Outcome transport did not return Implement'

    $stopDisplay = Invoke-SlipwayDirect -CommandArgs @('stop', $start.run_id, '--root', $repo)
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

    $sourcePath = Join-Path $tempRoot 'source file % ! & ^ 界.json'
    $sourceInitial = New-SourceEnvelope -RequirementText 'Keep the initial Windows requirement.' -UpdatedAt '2026-07-12T09:00:00Z'
    Write-Utf8 $sourcePath (($sourceInitial | ConvertTo-Json -Depth 20 -Compress) + "`r`n")
    $sourceStart = (Invoke-SlipwayDirect -CommandArgs @('run', 'issue-bound Windows', '--root', $repo, '--source-file', $sourcePath, '--budget', '8', '--json')) | ConvertFrom-Json
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

    $status = (Invoke-SlipwayDirect -CommandArgs @('status', $sourceStart.run_id, '--root', $repo, '--json')) | ConvertFrom-Json
    Assert-True (-not ($status.PSObject.Properties.Name -contains 'source_candidate')) 'candidate remained current after adopt'
    Assert-True ($status.last_source_choice.candidate_id -eq $candidateId) 'status lost candidate choice identity'
    Assert-True ($status.last_source_choice.choice -eq 'adopt') 'status lost candidate choice'

    Write-Output "native Windows acceptance ($Mode): ok"
} finally {
    if ((Test-Path -LiteralPath $tempRoot) -and ($env:SLIPWAY_KEEP_WINDOWS_FIXTURE -ne '1')) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
