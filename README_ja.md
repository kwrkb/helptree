# helptree

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

CLIツールの `--help` 出力を、インタラクティブなツリー構造で可視化するTUIビュアーです。

> **課題:** `docker`, `kubectl`, `gh`, `ffmpeg` といった現代のCLIツールは、サブコマンドやオプションが膨大です。`--help` の出力は「テキストの壁」をスクロールするだけの体験になりがちです。
>
> **helptree** はその壁を、ターミナルから離れることなく探索できるインタラクティブな階層マップに変換します。

## 特徴

- **ツリー表示** — サブコマンドを展開/折りたたみ可能なツリーとして閲覧
- **詳細ペイン** — 選択したコマンドのオプション・使い方・説明を表示
- **遅延ロード** — サブコマンドのヘルプは展開時にオンデマンド取得（初回の待ち時間なし）
- **インクリメンタルサーチ** — `/` キーでコマンドを即座に絞り込み
- **Vim風ナビゲーション** — `j`/`k`, `g`/`G`, `h`/`l` で快適に操作

## インストール

```bash
go install github.com/kwrkb/helptree@latest
```

## 使い方

```bash
helptree <コマンド名>
```

```bash
helptree docker
helptree gh
helptree kubectl
helptree go
```

## キーバインド

| キー | 操作 |
|------|------|
| `j` / `↓` | 下に移動 |
| `k` / `↑` | 上に移動 |
| `Enter` / `l` | 展開 / サブコマンド読み込み |
| `h` | 折りたたみ |
| `Space` | 展開/折りたたみ切り替え |
| `g` / `G` | 先頭 / 末尾にジャンプ |
| `/` | 検索 |
| `n` / `N` | 次 / 前の検索結果 |
| `Ctrl+d` / `Ctrl+u` | 詳細ペインのスクロール |
| `?` | キーバインドヘルプ表示 |
| `q` | 終了 |

## 仕組み

1. `<コマンド> --help` を実行し、出力をパースしてサブコマンドとオプションを抽出
2. 2ペインTUI（ツリー + 詳細）として表示
3. ノード展開時に再帰的に `--help` を実行し、より深い階層を構築（遅延ロード）

GNU形式、Go形式（タブ区切り）、kubectl形式（カッコ付きセクションヘッダー）のヘルプ出力に対応しています。

## ソースからビルド

```bash
git clone https://github.com/kwrkb/helptree.git
cd helptree
go build -o helptree .
```

## ライセンス

[MIT](LICENSE)
