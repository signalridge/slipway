# インストール

主要なインストール経路は Go、[GitHub Releases](https://github.com/signalridge/slipway/releases) の直接アーカイブと Linux package、GHCR のコンテナ、および repository の Nix flake です。Homebrew、Scoop、AUR は任意チャネルです。

## Go

```bash
go install github.com/signalridge/slipway@latest
```

## 直接アーカイブ

OS/architecture に合う archive と `checksums.txt` を GitHub Releases から取得し、checksum を検証して展開後、binary を `PATH` 上に配置します。Linux `amd64` の例:

```bash
release_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' https://github.com/signalridge/slipway/releases/latest)"
tag="${release_url##*/}"
version="${tag#v}"
archive="slipway_${version}_linux_amd64.tar.gz"
base_url="https://github.com/signalridge/slipway/releases/download/${tag}"
curl -fLO "${base_url}/${archive}"
curl -fLO "${base_url}/checksums.txt"
grep -F " ${archive}" checksums.txt | sha256sum --check -
tar -xzf "${archive}"
sudo install -m 0755 slipway /usr/local/bin/slipway
```

macOS は `darwin` 用 `.tar.gz`、Windows は `.zip` を選び、Windows では `Get-FileHash` で `checksums.txt` と照合してから `Expand-Archive` を使用します。

## Linux package

対応する package をダウンロードした directory で、distribution に合うコマンドを実行します。

```bash
sudo apt install ./slipway*.deb
sudo dnf install ./slipway*.rpm
sudo apk add --allow-untrusted ./slipway*.apk
```

## コンテナ

Versioned image は [GHCR](https://github.com/signalridge/slipway/pkgs/container/slipway) に公開されます。

```bash
docker pull ghcr.io/signalridge/slipway:<version>
docker run --rm ghcr.io/signalridge/slipway:<version> --version
```

コンテナイメージには Git が含まれます。Linux で mount した worktree に capability や journal を書く場合は、ホストの UID/GID を渡します。

```bash
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/workspace" -w /workspace ghcr.io/signalridge/slipway:<version> install --tool claude
```

## Nix

```bash
nix run github:signalridge/slipway
nix profile install github:signalridge/slipway
```

## 任意の package-manager チャネル

Homebrew、Scoop、AUR は独立した best-effort channel です。Core release は三つを明示的に skip し、archive、Linux package、checksum、SBOM、provenance、container を先に独立して公開します。`GH_PAT`（Homebrew/Scoop）または AUR SSH key がある場合だけ、失敗可能な別 publish/verify job を実行し、reproducible rebuild の archive checksum が公開済み core release と完全一致した場合だけ channel を更新します。Publisher、checksum verification、channel verification の失敗は core release を block も無効化もしないため、欠落・遅延する場合があります。利用前に表示 version を確認してください。

```bash
brew install --cask signalridge/tap/slipway
yay -S slipway-bin
```

```powershell
scoop bucket add signalridge https://github.com/signalridge/scoop-bucket
scoop install signalridge/slipway
```

## ホスト capability のインストール

```bash
slipway install --tool claude
slipway install --tool kiro --surface ide  # または --surface cli
```

`claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、`opencode`、`pi`、`qwen`、`windsurf` をサポートします。`--tool` は複数指定でき、`--tool all` も使えます。省略時は検出したホストだけを対象にします。Kiro の初回 install は `--surface ide|cli` が必須で、manifest が選択を記録し、後続の refresh/uninstall は同じ surface を自動的に使います。他ホストでの `--surface` と、既存 Kiro surface の暗黙切替は拒否されます。

Skill host はネイティブ skill UI から `slipway-<name>` を呼び出します（Codex は `$slipway-<name>`、Pi は `/skill:slipway-<name>`）。Copilot は agent picker から custom agent を選び、Kilo、OpenCode、Windsurf は `/slipway-<name>`、Kiro IDE は手動 include の `#slipway-<name>`、Kiro CLI は `kiro-cli chat --agent slipway-<name>` を使います。Copilot auto-detection は `.github/copilot`、`.github/prompts`、`.github/skills` のいずれかを認識し、custom agent は `.github/copilot/agents` に配置します。

```bash
slipway list
slipway doctor
slipway install --tool claude --refresh
slipway uninstall --tool claude
```

Refresh と uninstall は current version 2 ownership manifest のハッシュが一致するファイルだけを変更します。他の manifest version はすべて、install、refresh、uninstall がファイルを変更する前に fail closed となります。Read-only の `list` は引き続き実行でき、そのホストを advisory 付きの未インストールとして報告し、filesystem を変更せずに他の全ホストも列挙します。ユーザー変更、未知、範囲外 path、symlink は保持または安全に拒否され、marker-only は ownership を確立せず migration や推論も行いません。ホスト settings は一切変更しません。SessionStart や prompt-submit の自動入口は導入しません。
