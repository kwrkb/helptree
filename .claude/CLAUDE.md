# helptree — Project Configuration

## Overview

CLIツールの `--help` 出力をインタラクティブなツリー構造で可視化するTUIビュアー。
Go + Bubble Tea で構築。

## Build & Test

```bash
go build -o helptree .
go test ./...
```

## Project Structure

```
internal/
  model/    — データモデル (Node, Option)
  parser/   — --help 出力のパーサー
  runner/   — コマンド実行 (os/exec)
  tui/      — Bubble Tea TUI (app, tree, detail)
main.go     — エントリポイント
```

## Conventions

- パーサーのテストは固定文字列のヘルプ出力を使う（実コマンド依存を避ける）
- ランナーのテストのみ `go --help` を実行する（Go環境なら必ず存在）
- TUI のレイアウト計算は `app.go` の `View()` に集約
- Bubble Tea の `Update`/`View`/`Init` は value receiver（フレームワーク要件）

## Dependencies

- `github.com/charmbracelet/bubbletea` — TUI フレームワーク
- `github.com/charmbracelet/lipgloss` — スタイリング
