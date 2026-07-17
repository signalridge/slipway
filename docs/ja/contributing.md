# 開発リファレンス

このページは root の[コントリビューション](../../CONTRIBUTING.md)を補い、リポジトリ 固有の layout と documentation check を説明します。

## Repository layout

| Path | 目的 |
| --- | --- |
| `cmd/` | Public CLI command、JSON/human presentation。 |
| `internal/autopilot/` | Run routing、protocol validation、source handling、recovery choice。 |
| `internal/runstore/` | Journal、projection、locking、material、Git observation。 |
| `internal/adapter/` | Host generation、ownership-aware filesystem change。 |
| `internal/tmpl/` | 複数 ホスト 共通の embedded capability instruction。 |
| `internal/fsutil/` | Filesystem safety と transaction。 |
| `docs/{en,zh,ja}/` | 同等の範囲を持つ user、guide、reference、explanation page。 |
| `docs/reference/` | Language-neutral な JSON Schema。 |
| `adr/` | メンテナー向け decision history。User docs ではない。 |
| `acceptance/` | Black-box script、prompt scenario、manual evidence procedure。 |
| `website/` | `docs/` から生成される Starlight site。 |

強制される package direction は[アーキテクチャ](explanation/architecture.md)にあります。

## Local check

通常の Go 変更：

```bash
gofmt -w .
go vet ./...
go run ./internal/testlint/cmd/testlint ./...
go test -timeout=20m ./... -count=1
go build ./...
git diff --check
```

並行処理、locking、ジャーナル、filesystem の変更では race suite も実行します。

```bash
go test -timeout=20m ./... -race -count=1
```

利用可能なら次も実行します。

```bash
golangci-lint run --timeout 5m
goreleaser check
```

## Documentation check

Repository Markdown が website の source です。3つの locale splash page を除き、`website/src/content/docs/` 配下の generated file を手動で編集しないでください。

```bash
python3 -I acceptance/link_check.py --self-test
npm --prefix website ci
npm --prefix website run build
python3 -I acceptance/link_check.py --require-site
git diff --check
```

Documentation の規則：

- PR 手順や可変 planning Issue ではなく、現在の動作を記述する。
- User guide、integration reference、maintainer architecture、ADR rationale、acceptance evidence を分離する。
- English、中文、日本語の page の範囲を同等に保つ。
- いずれか1言語を独立した implementation contract にしない。
- Exact machine shape は JSON Schema で表現し、runtime-only semantic check を明記する。
- Host-side instruction は Go CLI guarantee ではなく ホスト behavior として書く。
- Stable user page に単発 CI run evidence や release history を入れない。
- Page 移動時は source link、website splash link、sidebar slug を同時に更新する。

## 変更種別ごとの test 重点

| 変更 | 重点 |
| --- | --- |
| CLI flag や output | Cobra help、JSON schema test、human rendering、コマンド docs。 |
| Action/Outcome routing | Autopilot contract/service test、machine shell acceptance、protocol docs。 |
| Journal や locking | Replay/adversarial test、race suite、durability diagnostics、recovery docs。 |
| Source handling | Strict parser、hash/size/identity test、source schema、Issue guide、privacy docs。 |
| Adapter/template | Generator test、ownership test、`acceptance/adapters.sh`、アダプター docs。 |
| Release channel | GoReleaser check、artifact validation、installation compatibility wording。 |
| Documentation | Link checker、markdown lint、website build、locale parity review。 |

## Product 制約

変更は、明示的起動、ユーザー制御、質問より先の事実確認、正直な activity 報告、read-only Review、recoverable な ジャーナル、ownership-aware generated file、network/認証情報を持たない core を維持する必要があります。公開 surface の変更時は、code、schema、generated capability、test、3言語の documentation を同時に更新してください。
