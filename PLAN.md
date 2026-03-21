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

## 将来の課題（保留）

### 巨大CLI対応（aws, kubectl 等）
- `aws` は第1階層だけで 300+ サービス、各サービスに数十のサブコマンド
- 現状の遅延ロード（展開時にのみ `--help` 実行）で深さ方向の爆発は回避できている
- 残る懸念:
  - 第1階層が巨大な場合のツリー表示 UX（スクロールが大変）
  - 「全展開」操作を入れる場合の並列ロード（goroutine プール）
  - 仮想スクロールやフィルタリングの必要性
- **方針**: 現設計で公開し、実際のフィードバックを元に判断する

### coreutils コマンドの誤検出
- `cp --help`, `mv --help` 等で GNU coreutils の長いオプション説明行がサブコマンドとして誤認される
- オプション行の優先度を上げるか、coreutils 特有のパターンを検出する必要がある

### パーサーの拡張性（構造推定ベースに移行済み）
- `ce92372` で正規表現カタログ（26個）→ 構造推定ベース（カラム検出 + ブロック分類）にリファクタ済み
- 新形式対応時のパターン: ブロック分類の再分類パス追加（brew）、セパレータ種別追加（python3）、ブロック種別のフォールバック（bare-flag）
- スモークテスト 35 CLI で PASS。正規表現は key-only パース（`keyShortLongRe` 等）にのみ使用
- **残る懸念**: ヒューリスティック追加のたびに既存テストへの副作用チェックが必要。マルチエージェントレビューで軽減

## Phase 4: 未対応ヘルプ形式への対応

スモークテストで検出した5つの未対応形式を修正し、パーサーのカバレッジを拡大する。

### 4-1: 全大文字セクションヘッダー (glab) ✅

`uppercaseSectionRe` (`^\s+([A-Z][A-Z ]{3,}[A-Z])\s*$`) を追加。
glab の 38+ サブコマンドをパース可能に。

### 4-2: メタデータトークン付きサブコマンド行 (glab) ✅

`subcommandWithMetaRe` (`^\s+([a-zA-Z][\w-]*)\s+[\[<].*\s{2,}(\S.+)$`) を追加。
`alias [command] [--flags]   Desc` 形式をパース可能に。

### 4-3: バイナリ名プレフィックス除去 (gemini) ✅

`stripBinaryPrefix()` を追加。"commands" セクション内でルートコマンド名プレフィックスを除去。
`gemini mcp → mcp` でサブコマンド認識。gemini children=5。

### 4-4: コロン区切り短オプション (python3) ✅

`colonSepShortOptRe` (`^(-[A-Za-z]{1,2})(?:\s+(\S+))?\s+:\s+(.+)`) を追加。
python3 の `-b : desc` 形式をパース可能に。python3 options=14+。

### 4-5: カラム0始まりオプション (fvm) ✅

`optShortLongRe`, `optBareShortLongRe` の先頭を `^\s{2,}` → `^\s*` に緩和。
fvm options=3 (全オプション認識)。

### 4-6: ブラケット圧縮オプション (npx) ✅

`parseBracketOptions()` を追加。`bracketPipeOptRe` と `standaloneLongOptRe` で
`[--pkg] [-c|--call]` 形式を抽出。npx options=5。

### 4-7: バイナリ名プレフィックス付きコマンド例 (brew) ✅

`classifyBlocks` 後にプレフィックス検出パスを追加。ブロック内の過半数の行が `rootName + " "` で始まる場合（2行以上かつ引数付き行あり）、commands セクションとして再分類。`trimCommandPrefix` で直接的なプレフィックス除去。brew children=11。

> Codex レビューで TestSelfParse の偽陽性（Examples セクション誤分類）を検出。ベアネームのみのブロックを除外するガードを追加。

### 検証

各タスク完了時:
```bash
go test -v -count=1 ./internal/parser/ -run TestSmoke
go test -v -count=1 ./...
```

## 検証方法

- 各フェーズのテスト: `go test ./...`
- 手動検証: `go run . docker`, `go run . gh`, `go run . kubectl` で実際に操作
- パーサーのテスト: 各CLIの `--help` 出力をテストデータとして固定し、ユニットテストで検証
