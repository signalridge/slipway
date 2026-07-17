# インストール

Current repository interface と各 channel の最新 package は一致しない場合があります。他の documentation を読む前に、`slipway --help` が `install`、`uninstall`、`list`、`doctor`、`run`、`status`、`stop` の7 command を表示することを確認してください。

## Current checkout を build する

[`go.mod`](https://github.com/signalridge/slipway/blob/main/go.mod) に指定された Go version（現在は Go 1.26.5 以上）を使います。

```bash
go build -o ./slipway .
./slipway --help
```

Unreleased repository revision を評価する最も確実な方法です。

## Tagged release

Release notes に7-command soft-autopilot interface が含まれる tag を選びます。Core artifact は [GitHub Releases](https://github.com/signalridge/slipway/releases) に公開されます。

- Linux/macOS: `.tar.gz`
- Windows: `.zip`
- Linux packages: `.deb`、`.rpm`、`.apk`
- `checksums.txt` と SBOM
- `ghcr.io/signalridge/slipway` の versioned image

SLSA provenance は独立した post-release job が生成し、その job が成功した場合に release へ追加されます。provenance がないことは、すでに公開された core archive、package、checksum、SBOM が作成されなかったことを意味しません。選択した tag に実際に存在する artifact を検証してください。

Archive と `checksums.txt` を download し、verify してから展開します。`slipway`（Windows は `slipway.exe`）を `PATH` に置きます。

Linux package は download directory で install できます。

```bash
# Debian または Ubuntu
sudo apt install ./slipway*.deb

# Fedora、RHEL、その他 RPM-based distribution
sudo dnf install ./slipway*.rpm

# Alpine
sudo apk add --allow-untrusted ./slipway*.apk
```

Install 後に interface を確認します。

```bash
slipway --version
slipway --help
```

## Go で tag から install する

Latest release がこの interface を含むまでは `@latest` を使わず、compatible tag を pin します。

```bash
go install github.com/signalridge/slipway@vX.Y.Z
```

`go install` build は release linker flag がないため development version metadata を表示する場合があります。Compatibility は pin した module version と command tree で確認してください。

## Container

```bash
docker pull ghcr.io/signalridge/slipway:vX.Y.Z
docker run --rm ghcr.io/signalridge/slipway:vX.Y.Z --help
```

Image は Git を含みます。Linux で mounted worktree に capability や Run data を書く場合は host UID/GID を使います。

```bash
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/workspace" -w /workspace \
  ghcr.io/signalridge/slipway:vX.Y.Z install --tool claude
```

## Nix

Flake を compatible tag に pin します。Tag なし GitHub flake は mutable default branch を追跡します。

```bash
nix run github:signalridge/slipway/vX.Y.Z -- --help
nix profile install github:signalridge/slipway/vX.Y.Z
```

## Optional package-manager channel

Homebrew、Scoop、AUR は secondary publisher であり、core GitHub release より遅れる場合があります。表示 version と `slipway --help` を確認してください。

### Homebrew cask

Release workflow が検証する explicit tap/trust sequence：

```bash
brew tap signalridge/tap
brew trust signalridge/tap
brew install --cask slipway
```

### Scoop

```powershell
scoop bucket add signalridge https://github.com/signalridge/scoop-bucket
scoop install signalridge/slipway
```

### AUR

```bash
yay -S slipway-bin
```

## Host capability を install する

Target Git worktree 内で実行します。

以下の command は current checkout から build した `./slipway` を使います。Compatible な tagged package を install した場合は、代わりに `PATH` 上の `slipway` を使ってください。

```bash
./slipway install --tool claude
./slipway list
./slipway doctor
```

Supported ID は `claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、`opencode`、`pi`、`qwen`、`windsurf` です。複数の host は `--tool` を繰り返すか、`--tool claude,codex` のようにカンマ区切りの値を1つ渡して選択します。

Kiro の初回 install では surface を明示します。

```bash
./slipway install --tool kiro --surface ide   # または: --surface cli
```

Kiro を mixed selection に含める場合、`--surface` は Kiro だけに適用されます。たとえば `--tool claude --tool kiro --surface ide` と `--tool all --surface ide` は有効です。Refresh と uninstall は記録済みの Kiro surface を推論します。

`--tool` を省略すると detected host directory を選びます。Detection は convenience です。複数 host 設定を持つ repository では install 前に `./slipway list` を確認してください。

## Refresh と uninstall

```bash
./slipway install --tool claude --refresh
./slipway uninstall --tool claude
```

Slipway は host ごとの ownership manifest に generated path と hash を記録します。Refresh/uninstall は記録と一致する managed file だけを変更します。Modified、unknown、malformed、out-of-host、symlinked path は preserve または reject して報告します。Host settings は adapter ownership の外です。

Current manifest が以前の release で生成された bytes をまだ claim している場合、refresh と uninstall はその file を preserve し、安全に overwrite/delete できるものとは扱わず stale claim を取り下げます。Preserve された file を確認して退避し、current release に再生成させる場合は `slipway install --refresh` を再実行してください。

Ownership manifest を偽造または編集して installation を復旧しないでください。Current manifest がなく、`.adapter-generated` や generated-looking file が残っている場合は、まず host surface を backup して確認します。Slipway に再生成させたい sentinel と file だけを退避してから、その host に対して `slipway install` を再実行します。残した content は preserve され、ownership に adopt されません。これは manual recovery であり、manifest reconstruction や automatic migration ではありません。

Adapter removal は Run journal を削除しません。Retention は [Run、復旧、プライバシー](guides/runs-and-recovery.md)を参照してください。
