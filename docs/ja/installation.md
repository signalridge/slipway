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

Homebrew と Scoop は `GH_PAT` secret、AUR は AUR SSH key に依存します。いずれも GoReleaser の `skip_upload: auto` 対象なので、secret がない release では更新されない場合があります。任意チャネルであり、release gate ではありません。

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
```

`claude`、`codex`、`copilot`、`cursor`、`kilo`、`kiro`、`opencode`、`pi`、`qwen`、`windsurf` をサポートします。`--tool` は複数指定でき、`--tool all` も使えます。省略時は検出したホストだけを対象にします。

```bash
slipway list
slipway doctor
slipway install --tool claude --refresh
slipway uninstall --tool claude
```

Refresh と uninstall は current version 2 ownership manifest のハッシュが一致するファイルだけを変更します。他の manifest version はすべて fail closed で、install、refresh、uninstall、list を認可しません。ユーザー変更、未知、範囲外 path、symlink は保持または安全に拒否され、marker-only は ownership を確立せず migration や推論も行いません。ホスト settings は一切変更しません。SessionStart や prompt-submit の自動入口は導入しません。
