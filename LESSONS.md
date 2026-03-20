# LESSONS

## 2026-03-20: 初期実装

### パーサーの正規表現設計

- **セクションヘッダーの多様性**: `Commands:`, `Available Commands:`, `Basic Commands (Beginner):`, `The commands are:` など形式が多い。カッコを含むヘッダー (`[\w)]` で終端) や "The X are:" 形式も考慮が必要
- **タブ vs スペース**: `go --help` はタブ区切り、他はスペース区切り。`\s+` で統一的に扱う。ただし先頭インデントの `\s{2,}` はタブ1文字にマッチしないため `\s+` に緩和した

### Bubble Tea のレシーバ型

- `Init()`, `Update()`, `View()` は value receiver が必要（Bubble Tea の interface 要件）
- 内部ヘルパーは pointer receiver を使ってよい
- 混在は意図的だが、コメントで明記すること

### append のスライス変異

- `append(args, "--help")` は元スライスに余剰容量があると破壊的。`make` + `copy` で防御する

## 2026-03-21: パーサー強化 + レビュー対応

### データレース防止: Bubble Tea の非同期パターン

- バックグラウンド `tea.Cmd` 内で既存の Node ポインタを直接変異させるとデータレースが発生する
- **ルール**: cmd 内では新しいデータを生成し、msg 経由で返す。`Update` 内（メインゴルーチン）でのみツリーを変異させる

### ノードロードのロジック: LoadChildren vs LoadNode

- `LoadChildren(parent)` は `parent.Children` をループするが、未ロードノードの Children は空なので何も起きない
- **ルール**: 未ロードノード展開時はそのノード自身の `--help` を実行する（`LoadNode`）。子の子をロードするのではなく、ノード自身の情報を取得する

### descAppender バッファリング方式

- `--help` 出力の Description は頻繁に次行へ折り返される（特に kubectl, bat）。ステートレスな行単位処理では対応できない
- **ルール**: 直前にパースしたアイテムへのポインタ (`descAppender`) を保持し、次行が continuation かどうかを description 開始カラム位置で判定する

### bare option flag（フラグのみ行）

- bat, uv, fnm 等は `-v, --verbose` だけの行に description が次行以降にインデントされて続く形式
- **ルール**: `optBareShortLongRe` / `optBareLongRe` でフラグのみ行をマッチし、`descAppender` の col=0 で次行を description として取り込む

### ヘッダーなしセクションの推論

- 一部のツールはセクションヘッダーなしでサブコマンドやオプションが並ぶ
- **ルール**: `currentSection = "unknown"` から開始し、subcommand パターンが2行連続マッチしたら `"commands"` に昇格。option パターンは即座に `"options"` に遷移

### カンマ区切りコマンドリスト

- npm は `access, adduser, audit, bugs, ...` のようにカンマ区切りでコマンドを列挙する
- **ルール**: `commaSepListRe` で3個以上のカンマ区切り単語列を検出し、各単語を description なしの Node として生成

### Usage の複数行対応

- gh, kubectl 等は `Usage:` の後に改行し、次行にインデントされた synopsis を置く
- **ルール**: `usageRe` を `(.*)` にして空キャプチャも許可。空の場合は次のインデント行を収集する

### coreutils コマンドの誤検出

- `cp --help`, `mv --help` 等で GNU coreutils の長いオプション説明行がサブコマンドとして誤認される場合がある
- 現時点では未対応。将来的にはオプション行の優先度を上げるか、coreutils 特有のパターンを検出する必要がある
