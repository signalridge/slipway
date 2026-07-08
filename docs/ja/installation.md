# インストール

Slipway は通常、公開されたリリース成果物、またはリリースに裏付けられたパッケージチャネルからインストールします。`go install`、Nix、ローカルソースビルドといった開発者向けのパスは、未リリース版が必要なとき、まだパッケージ化されていないプラットフォーム向けのパスが必要なとき、あるいは再現可能な開発環境が必要なときに利用できます。

`vX.Y.Z` は使いたいリリースタグに置き換えてください。未リリースの作業には、ローカルチェックアウトからのビルドパスを使います。

## 公式の入手元

Slipway プロジェクトが管理する、ドキュメント化されたリリース入手元を使ってください。`signalridge/slipway` の GitHub Releases、`ghcr.io/signalridge/slipway` のコンテナイメージ、`signalridge/tap` の Homebrew Cask エントリ、`signalridge/scoop-bucket` の Scoop マニフェスト、そしてそのチャネルが公開済みであれば AUR の `slipway-bin` です。AI ツールが別のレジストリで同名のパッケージを見つけた場合は、インストールする前に必ず作業を止めて所有者を確認してください。

## 前提条件

- リポジトリの初期化とガバナンス対象作業のための Git。
- ソースからビルドする場合や `go install` を使う場合は、`go.mod` に合致する Go。
- 任意: flake パッケージを使う場合の Nix。
- 任意: コンテナイメージを使う場合の Docker などの OCI ランタイム。
- 任意: ローカルでドキュメントをビルドする場合の Astro Starlight。
- 任意: `slipway init --tools` がサポートする 1 つ以上の AI コーディングツール。

## インストールの順序

通常のインストールでは次の順序を使ってください。

1. プラットフォームに合った公式リリースアーカイブ、またはリリースに裏付けられたパッケージチャネルを優先します。
2. ホストにバイナリをインストールせずに Slipway を実行したい場合は、コンテナイメージを使います。
3. リリースパッケージが入手できない場合や、Go が管理するバイナリを明示的に使いたい場合は、`go install` を使います。
4. 開発や未リリースの変更には、Nix またはローカルソースビルドを使います。

## リリースインストール対応表

| プラットフォーム | リリース成果物 | パッケージチャネル | その他のパス |
| --- | --- | --- | --- |
| macOS amd64 | `slipway_<version>_darwin_amd64.tar.gz` | 公開済みの場合は Homebrew Cask | Go install、Nix、ソースビルド |
| macOS arm64 | `slipway_<version>_darwin_arm64.tar.gz` | 公開済みの場合は Homebrew Cask | Go install、Nix、ソースビルド |
| Linux amd64 | `slipway_<version>_linux_amd64.tar.gz`、`.deb`、`.rpm`、`.apk` | 公開済みの場合は AUR `slipway-bin` | Go install、Nix、コンテナイメージ、ソースビルド |
| Linux arm64 | `slipway_<version>_linux_arm64.tar.gz`、`.deb`、`.rpm`、`.apk` | 公開済みの場合は AUR `slipway-bin` | Go install、Nix、コンテナイメージ、ソースビルド |
| Windows amd64 | `slipway_<version>_windows_amd64.zip` | 公開済みの場合は Scoop | Go install、ソースビルド |
| Windows arm64 | `slipway_<version>_windows_arm64.zip` | 公開済みの場合は Scoop | Go install、ソースビルド |

リリースワークフローが完了すると、GoReleaser は `checksums.txt`、アーカイブの SBOM、チェックサム署名、コンテナ署名も公開します。パッケージマネージャーのチャネルは任意の公開クレデンシャルを利用するため、あるリリースに対してチャネルが存在しない場合は、`go install`、Nix、ローカルチェックアウトのパスにフォールバックする前に、まず直接のリリースアーカイブを優先してください。

## 直接のリリースアーカイブ

パッケージマネージャーを介さずに公開済みバイナリを入手したい場合は、直接のリリースアーカイブを使います。以下のプラットフォーム別セクションに、macOS、Linux、Windows のコマンドを示します。

## パッケージマネージャー

該当するチャネルがそのリリース向けに公開済みの場合は、リリースに裏付けられたパッケージマネージャーを使ってください。

- macOS: `signalridge/tap` 経由の Homebrew Cask。
- Linux: `.deb`、`.rpm`、`.apk`、または AUR `slipway-bin`。
- Windows: `signalridge/scoop-bucket` 経由の Scoop。

## Go Install によるフォールバック

Go が利用でき、開発者向けのフォールバックや `PATH` 上に Go 管理のバイナリが欲しい場合は、このパスを使います。

```bash
go install github.com/signalridge/slipway@latest
slipway --version
```

特定のリリースを指定する場合:

```bash
go install github.com/signalridge/slipway@vX.Y.Z
slipway --version
```

## ソースからのビルド

このパスは、Slipway を開発する場合や未リリースの変更をテストする場合にのみ使ってください。

```bash
go build -o ./bin/slipway .
./bin/slipway --version
./bin/slipway --help
```

`./bin/slipway` を直接使うか、`./bin` を `PATH` に追加してください。

## macOS

Homebrew Cask:

```bash
brew install --cask signalridge/tap/slipway
slipway --version
```

直接のアーカイブ:

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
ARCH="$(uname -m)"
case "$ARCH" in
  arm64) SLIPWAY_ARCH=arm64 ;;
  x86_64) SLIPWAY_ARCH=amd64 ;;
  *) echo "unsupported macOS arch: $ARCH" >&2; exit 1 ;;
esac
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/slipway_${VERSION}_darwin_${SLIPWAY_ARCH}.tar.gz"
tar xzf "slipway_${VERSION}_darwin_${SLIPWAY_ARCH}.tar.gz"
install -m 0755 slipway /usr/local/bin/slipway
slipway --version
```

## Linux

直接のアーカイブ:

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
ARCH="$(uname -m)"
case "$ARCH" in
  aarch64|arm64) SLIPWAY_ARCH=arm64 ;;
  x86_64) SLIPWAY_ARCH=amd64 ;;
  *) echo "unsupported Linux arch: $ARCH" >&2; exit 1 ;;
esac
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/slipway_${VERSION}_linux_${SLIPWAY_ARCH}.tar.gz"
tar xzf "slipway_${VERSION}_linux_${SLIPWAY_ARCH}.tar.gz"
sudo install -m 0755 slipway /usr/local/bin/slipway
slipway --version
```

Debian または Ubuntu:

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
case "$(uname -m)" in
  aarch64|arm64) ARCH=arm64 ;;
  x86_64) ARCH=amd64 ;;
  *) echo "unsupported Linux arch: $(uname -m)" >&2; exit 1 ;;
esac
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/slipway_${VERSION}_linux_${ARCH}.deb"
sudo dpkg -i "slipway_${VERSION}_linux_${ARCH}.deb"
slipway --version
```

Fedora、RHEL、または互換性のある RPM 系システム:

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
case "$(uname -m)" in
  aarch64|arm64) ARCH=arm64 ;;
  x86_64) ARCH=amd64 ;;
  *) echo "unsupported Linux arch: $(uname -m)" >&2; exit 1 ;;
esac
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/slipway_${VERSION}_linux_${ARCH}.rpm"
sudo rpm -i "slipway_${VERSION}_linux_${ARCH}.rpm"
slipway --version
```

Alpine:

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
case "$(uname -m)" in
  aarch64|arm64) ARCH=arm64 ;;
  x86_64) ARCH=amd64 ;;
  *) echo "unsupported Linux arch: $(uname -m)" >&2; exit 1 ;;
esac
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/slipway_${VERSION}_linux_${ARCH}.apk"
sudo apk add --allow-untrusted "slipway_${VERSION}_linux_${ARCH}.apk"
slipway --version
```

パッケージが公開済みの場合は、AUR 経由の Arch Linux:

```bash
yay -S slipway-bin
slipway --version
```

コンテナイメージ:

```bash
TAG=vX.Y.Z
VERSION="${TAG#v}"
docker run --rm ghcr.io/signalridge/slipway:${VERSION} --version
```

コンテナから現在のリポジトリを操作するには:

```bash
docker run --rm -v "$PWD:/workspace" -w /workspace ghcr.io/signalridge/slipway:${VERSION} status --json
```

## Windows

バケットが公開済みの場合の Scoop:

```powershell
scoop bucket add signalridge https://github.com/signalridge/scoop-bucket
scoop install slipway
slipway --version
```

直接の zip:

```powershell
$Tag = "vX.Y.Z"
$Version = $Tag.TrimStart("v")
$Arch = "amd64"
$Asset = "slipway_${Version}_windows_${Arch}.zip"
Invoke-WebRequest "https://github.com/signalridge/slipway/releases/download/${Tag}/${Asset}" -OutFile $Asset
Expand-Archive $Asset -DestinationPath .
.\slipway.exe --version
```

Windows arm64 では、そのリリースアセットが存在する場合に `amd64` の代わりに `arm64` を使ってください。

## Nix

チェックアウトから:

```bash
nix build .#slipway
./result/bin/slipway --version
```

GitHub から:

```bash
nix run github:signalridge/slipway#slipway -- --help
```

## リリースダウンロードの検証

成果物の整合性チェックが必要な環境では、アセットと一緒にリリースのチェックサムファイルをダウンロードし、インストール前に検証してください。

```bash
TAG=vX.Y.Z
curl -LO "https://github.com/signalridge/slipway/releases/download/${TAG}/checksums.txt"
sha256sum -c checksums.txt --ignore-missing
```

macOS で GNU の `sha256sum` が利用できない場合は、`shasum -a 256` を使ってください。

## リポジトリの初期化

対象リポジトリ、またはその中の子ディレクトリで `init` を実行します。

```bash
slipway init
```

これにより、リポジトリの `.slipway.yaml` 設定に加えて、`.gitignore` に
「# Slipway local state (managed)」の管理ブロックが書き込まれ（`.slipway-tmp/`、
バンドルローカルの `events/`、`verification/`、レガシーの変更ごとの `evidence/`、`.worktrees/` の各パスを無視）、
リポジトリローカルの `.git/slipway/` ランタイム領域が作成されます。ランタイムのタスク証跡は
`.git/slipway/runtime/changes/<slug>/evidence/` 以下に記録されます。一時的なタスク結果 JSON は
`slipway evidence task --result-file` 用に `.slipway-tmp/` に置いてください。このディレクトリは
無視され、scope-contract から除外される scratch 領域です。`--tools` を渡さないかぎり、
AI ツールのサーフェスは一切生成されません。

```bash
slipway init --tools claude
slipway init --tools codex,opencode
slipway init --tools copilot,pi,qwen,windsurf
slipway init --tools all
slipway init --tools none
```

サポートされるツール ID は `claude`、`codex`、`copilot`、`cursor`、
`kilo`、`kiro`、`opencode`、`pi`、`qwen`、`windsurf` です。

生成される代表的なアダプターディレクトリには、`.claude/skills`、
`.codex/skills`、`.github/skills`、`.cursor/skills`、
`.kilocode/skills`、`.kiro/skills`、`.opencode/skills`、`.pi/skills`、
`.qwen/skills`、`.windsurf/skills` があります。Copilot はさらに `.github/prompts`
以下にコマンドプロンプトを書き込み、生成された所有状態を
`.github/copilot/slipway` 以下に保持します。

Slipway 管理のアダプターファイルを再生成するには `--refresh` を使います。

```bash
slipway init --tools opencode --refresh
```

リフレッシュ時に `--tools` を省略すると、Slipway は以前に生成したアダプターを検出し、
それらの管理対象サーフェスをリフレッシュします。リフレッシュはまた、ユーザー所有のフックを
保持しつつ、Slipway が所有するレガシーのシェルフックランチャーと設定エントリを削除します。

## AI ツール向けインストールプロンプト

現在のリポジトリ向けに Slipway をインストールして初期化させたいときは、これを AI コーディングツールに貼り付けてください。貼り付ける前に内容を読み、エージェントの実行中は監督してください。このプロンプトは意図的に短くしてあります。エージェントをこのページへ誘導し、以下の正規ガイダンスを一箇所にまとめておくためです。

```text
Install Slipway for this repository.

Read https://signalridge.github.io/slipway/installation/ — specifically the
"AI Tool Installation Prompt" section — and follow it.

Before installing, detect the operating system and CPU architecture, and run
`slipway --version` to see if Slipway is already on PATH. Prefer documented
release sources owned by the Slipway project (the `signalridge` org). Do NOT
install same-name packages from unrelated registries. If no documented path
applies, stop and report.

After installing, run `slipway --version`, `slipway status --json`, and
`git status --short --branch`. Report which install path succeeded and what
files were generated (especially `.slipway.yaml` and adapter directories for
the selected tool IDs).
```

このセクションの残りは、エージェントがこのページを取得した後に読む正規ガイダンスです。

### 探索

- リポジトリのルートを調べ、`.slipway.yaml` がすでに存在するかどうかを確認します。
- このマシンのオペレーティングシステムと CPU アーキテクチャを検出します。
- `slipway --version` を実行します。バージョンが表示されれば Slipway はすでに `PATH` 上にあるので、**検証**へスキップします。そうでなければ**インストール**へ進みます。

### インストール（優先順に試し、最初に成功した時点で止める）

1. この OS とアーキテクチャ向けに、Slipway プロジェクト（`signalridge`）が管理する、ドキュメント化されたリリース成果物、またはリリースに裏付けられたパッケージチャネル。該当する成果物が存在しない場合は、無関係なレジストリの同名パッケージにフォールバックせず、次のステップへ進みます。
2. **macOS:** `brew` が利用でき、`signalridge/tap` の cask が公開済みであれば、`brew install --cask signalridge/tap/slipway` を実行します。そうでなければ、該当する `darwin_amd64` または `darwin_arm64` のリリースアーカイブを使います。
3. **Linux:** 該当する `linux_amd64` または `linux_arm64` のリリースアーカイブ、あるいはそのチャネルが利用できる場合は該当する `.deb`、`.rpm`、`.apk`、AUR `slipway-bin`、`ghcr.io/signalridge/slipway` コンテナイメージのいずれかを選びます。
4. **Windows:** 設定済みであれば Scoop（`signalridge/scoop-bucket`）を使います。そうでなければ、該当する `windows_amd64` または `windows_arm64` のリリース zip を使います。
5. リリースに裏付けられたチャネルが利用できないが Go がインストール済みの場合は、`go install github.com/signalridge/slipway@latest` を実行します。
6. このリポジトリが Slipway のソースチェックアウトそのものであり、ローカルの未リリース版が意図的に必要な場合は、`go build -o ./bin/slipway .` を実行して `./bin/slipway` を使います。
7. ドキュメント化されたパスがいずれも機能しない場合は、作業を止めて、試したパスとそれぞれを阻んだ要因を報告します。インストーラーを勝手に作らず、無関係なレジストリの同名パッケージも取得しないでください。

### 初期化

- このリポジトリがどの AI ツールアダプターを使うのか不明な場合は確認します。サポートされるツール ID は `claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、`opencode`、`pi`、`qwen`、`windsurf` です。
- `slipway init --tools <tool-id>`、`slipway init --tools claude,codex,opencode`、`slipway init --tools copilot,kiro,pi,qwen,windsurf,kilo`、`slipway init --tools all` のいずれかを実行します。
- Slipway が生成したアダプターファイルがすでに存在する場合は、代わりに `slipway init --tools <detected-tools> --refresh` を使います。
- 無関係なユーザー所有の AI ツールファイルを上書きしないでください。生成されるパスがユーザー所有のコンテンツと衝突する場合は、上書きせずに作業を止めて報告します。

### 検証

- `slipway --version`
- `slipway status --json`
- `git status --short --branch`

### 報告

- どのインストールパスが成功し、それより前のどのパスがスキップまたは失敗したか。
- 新たに生成されたファイル。特に `.slipway.yaml` と、選択したアダプターディレクトリ（`.claude/skills`、`.codex/skills`、`.github/skills`、`.cursor/skills`、`.kilocode/skills`、`.kiro/skills`、`.opencode/skills`、`.pi/skills`、`.qwen/skills`、`.windsurf/skills` など）。
- ユーザーが知っておくべき未解決のフォローアップ（たとえば、このプラットフォーム向けのリリースが存在しない、あるいは人による判断がまだ必要な `slipway init` の選択など）。

OpenCode に固有のものとして、生成されるプロジェクトサーフェスは次のとおりです。

- `.opencode/skills/slipway-*/SKILL.md`
- `.opencode/commands/slipway-*.md`
- `.opencode/hooks/slipway-session-start`
- `.opencode/hooks/slipway-session-start.ps1`
- `.opencode/hooks/slipway-session-start.cmd`

OpenCode のコマンドは `/slipway-new`、`/slipway-next`、`/slipway-run` のようなスラッシュ＋ハイフンの表記を使います。OpenCode の一部のビルドでは、コマンドピッカーでプロジェクトコマンドにプロジェクトのプレフィックスを付けて表示します。安定した契約となるのは、生成されたファイルのパスです。

生成されたフックランチャーを使うアダプター（Cursor と OpenCode を含む）は、
POSIX、PowerShell、`cmd.exe` 向けのネイティブランチャーファイルを、それぞれの
`hooks/` ディレクトリ以下に受け取ります。設定対応のフックホスト（Claude と Qwen）は
代わりに、ベアなインラインの `slipway hook ...` コマンドを `settings.json` に直接登録し、
ランチャーファイルは持ちません。Pi の設定はフックではなくスキルとプロンプトを登録します。
Pi のセッション開始ブリッジは、プロジェクトローカルの
`.pi/extensions/slipway-hooks.ts` 拡張として生成され、Pi でプロジェクトを信頼した後にだけ読み込まれます。
生成されたフックは bash、Python、`jq`、`gh` を必要としません。リリースモードの生成では
`PATH` 上の `slipway` バイナリを解決します。一方で、`slipway init` を Slipway のソース checkout 内で
実行した場合、dogfood がその checkout を追跡できるよう、管理対象のフックコマンドが意図的に
`go -C <checkout> run .` を使うことがあります。

生成されたスキルヘルパーは、生成されたスクリプトのペイロードではなく `slipway tool ...`
を介して実行されます。手動のヘルパーは、GitHub ヘルパー向けの `gh` や Go テストの汚染追跡向けの
`go` のように、明示的に認証されたバックエンドやドメインツールを依然として必要とする場合があり、
それらが利用できないときはフェイルクローズドになり、修復方法を提示します。

## インストールの検証

```bash
slipway --version
slipway status --json
git status --short --branch
```

アダプターを使って初期化したリポジトリでは、生成されたファイルを確認します。

```bash
find .claude .codex .github/skills .github/prompts .github/copilot .cursor .kilocode .kiro .opencode .pi .qwen .windsurf -maxdepth 3 -type f 2>/dev/null
```

Codex のコマンドサーフェスは、`.codex/skills/slipway-<command>/SKILL.md` 以下のスキルとして
生成されます。Codex のリフレッシュが管理するのはプロジェクトローカルの `.codex/` アダプターツリーのみで、
ホストグローバルの `$CODEX_HOME/prompts/` や `~/.codex/prompts/` のファイルには触れません。
フック対応のアダプターでは、`--refresh` が Slipway 所有の廃止されたフックランチャーを削除します。
設定対応のホストは、廃止されたランチャーパスの設定エントリをベアなインラインの
`slipway hook ...` コマンドへ移行します。Cursor と OpenCode は、パス指定のセッション開始ランチャーを
ファイルとして保持します。Pi はプロジェクトローカルの `.pi/extensions/` セッション開始ブリッジを保持します。
