# PLAN: helptree

## Context

CLIツールの `--help` 出力をインタラクティブなツリー構造で可視化するTUIビュアーを新規開発する。
VISION.md の3つの価値（俯瞰・発見・ターミナル完結UX）を実現する。
LLM連携・翻訳機能はスコープ外。

## 技術選定

### 結論: **Go + Bubble Tea**

| 観点 | Go + Bubble Tea | Rust + Ratatui | Python + Textual |
|------|----------------|----------------|------------------|
| 配布 | シングルバイナリ | シングルバイナリ | ランタイム必要 |
| ツリー | community (tree-bubble) | community (tui-tree-widget) | 組み込み |
| レイアウト | 手動管理が必要 | Layout API充実 | CSS風で容易 |
| 開発速度 | 中 | 中 | 高 |

**Go を選ぶ理由:**
1. **配布の簡潔さ**: CLIツールとしてシングルバイナリが最も自然。`go install` で即導入可能
2. **サブプロセス実行**: `os/exec` がシンプルで、`--help` の再帰的実行と相性が良い
3. **Charm エコシステム**: lipgloss（スタイリング）、bubbles（コンポーネント）が充実
4. **類似ツール不在**: 調査の結果、`--help` をツリー化するTUIツールは存在しない。新規性がある

**レイアウトの課題**: Bubble Tea での2ペイン管理は手動計算が必要だが、community の panes パッケージや lipgloss の Place/JoinHorizontal で対処可能。

## アーキテクチャ

```
helptree/
├── main.go              # エントリポイント + CLI引数処理
├── go.mod
├── internal/
│   ├── parser/
│   │   ├── parser.go    # --help 出力のパース（サブコマンド・オプション抽出）
│   │   └── parser_test.go
│   ├── runner/
│   │   ├── runner.go    # コマンド実行（--help の再帰的呼び出し）
│   │   └── runner_test.go
│   ├── model/
│   │   └── node.go      # ツリーノードのデータ構造
│   └── tui/
│       ├── app.go       # Bubble Tea Model（メインアプリ）
│       ├── tree.go      # ツリーペイン
│       └── detail.go    # 詳細ペイン
```

## フェーズ

### Phase 1: 基盤 — パーサー + 最小TUI ✅ 完了
- [x] プロジェクト初期化（go mod init, 依存追加）
- [x] データモデル定義（`model/node.go` — コマンド名、説明、サブコマンド、オプション）
- [x] `--help` 出力のパーサー実装（GNU style を優先対応）
- [x] コマンド実行ランナー（`cmd --help` → パース → ツリー構築）
- [x] 最小TUI: ツリー表示 + 詳細ペイン（j/k移動、Enter展開/折りたたみ）
- [x] テスト: `docker`, `gh`, `kubectl` の --help 出力でパーサー検証

#### 実装メモ
- パーサーはGNU style、Go style（タブ区切り）、kubectl style（カッコ付きセクションヘッダー）に対応
- 2ペインレイアウト（ツリー + 詳細）を Phase 1 で前倒し実装
- サブコマンドの遅延ロード対応済み（Enterで --help 再帰実行）

### Phase 2: 詳細ペイン + UX ✅ 完了
- [x] ウィンドウリサイズ対応
- [x] キーバインドヘルプ（`?` キー）
- [x] 詳細ペインのスクロール（Ctrl+d / Ctrl+u）

### Phase 3: 検索 + 仕上げ ✅ 完了
- [x] インクリメンタルサーチ（`/` キーで起動、n/N でジャンプ）
- [x] エラーハンドリング（存在しないコマンド、--help 非対応など）
- [x] README.md 作成、`go install` での導入手順
- [x] バージョン表示（`--version` / `-v`）

## 検証方法

- 各フェーズのテスト: `go test ./...`
- 手動検証: `go run . docker`, `go run . gh`, `go run . kubectl` で実際に操作
- パーサーのテスト: 各CLIの `--help` 出力をテストデータとして固定し、ユニットテストで検証
