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

### スモークテストで発見した未対応ヘルプ形式（5パターン） — 対応済み

上記5パターンは `0aab70c` で全て対応済み。

## 2026-03-21: パーサーリファクタ（正規表現ベース → 構造推定ベース）

### カラム検出ベースの構造推定が正規表現カタログより拡張性が高い

- 正規表現は「行の見た目」で判断するため、新形式ごとに追加が必要（26個まで膨張した）
- 空白カラムの整列（2+ space gap）はほぼ全てのCLIヘルプ形式に共通する構造的特徴
- **ルール**: 新形式対応時はまず `findGap` / `detectColumns` の動作を確認し、構造検出で対応可能かを先に検討する。正規表現は key-only パースにのみ使う

### stripBinaryPrefix とカラム位置のずれ

- `stripBinaryPrefix` はプレフィックスを除去して行の長さを変える。ブロックの `DescCol` は元の行から計算されるため、strip後の行に `splitAtColumn(line, b.DescCol)` を適用するとカラム位置がずれる
- **ルール**: カラム分割は strip 前に行い、key 部分だけに prefix strip を適用する

### keyMultiFlagRe と keyShortLongRe の優先順序

- `-D, --debug` は multi-flag regex（`(-\w|--[\w-]+)(,...)` 2+ flags）にもマッチする。先に multi-flag をチェックすると short/long のペアリングが壊れる
- **ルール**: `keyShortLongRe` を最優先でチェックし、multi-flag は 3+ flags の場合のみ適用

### "other" ヘッダーの伝搬を止める

- `categorizeSection("Create")` = `"other"`。`"other"` ヘッダーの section を後続ブロックに伝搬すると、`"Create:"` 配下のオプションブロックがスキップされる（tar の問題）
- **ルール**: `classifyBlocks` で `"other"` 分類のヘッダーは伝搬せず、後続ブロックは内容推論にフォールバックさせる

## 2026-03-21: brew 形式対応 + マルチエージェントレビュー

### バイナリ名プレフィックス付きコマンド例の再分類

- brew の `--help` は "Example usage:" 等のセクション内に `brew search TEXT|/REGEX/` のようなコマンド例を並べる形式。ヘッダーに "command" を含まないため `categorizeSection` で "other" になる
- **ルール**: `classifyBlocks` 後にバイナリ名プレフィックス検出パスを追加。`prefixed >= 2 && prefixed*2 > total` で再分類。ただし全行がベアネーム（引数なし）の場合は Examples セクションの可能性が高いため除外する

### Examples セクションの誤分類（Codex レビューで発見）

- `helptree docker` / `helptree kubectl` のような使用例が `helptree ` プレフィックスにマッチし、`docker` 等がサブコマンドとして誤認される
- **ルール**: プレフィックス除去後に全行が単一単語（引数なし）の場合は使用例とみなし、コマンドとして再分類しない。`hasArgs` チェックで最低1行に引数が必要

### マルチエージェントレビューの有効性

- Codex（gpt-5.4）と Gemini を並列でレビューさせると、異なる観点のフィードバックが得られる
- Codex は `TestSelfParse` での偽陽性（P1バグ）を発見、Gemini は可読性・リファクタ観点の改善を提案
- **ルール**: 重要なパーサー変更は自己レビュー + マルチエージェントレビューを実施。特にヒューリスティック追加時は既存テストケースへの副作用を重点的にチェック

### `stripBinaryPrefix` の "  " ハック回避

- `stripBinaryPrefix` はインデント保持のための関数で、既にトリム済みの文字列に `"  "` を付加して呼ぶのは可読性が低い
- **ルール**: トリム済み文字列のプレフィックス除去には専用の `trimCommandPrefix` を使う。`stripBinaryPrefix` はカラム位置を保持する必要がある場面（BlockTable のキー分割）でのみ使用

### Clap/Rust CLI の bare-flag 形式は構造推定で自動対応できる

- bat, rg, fd, fnm 等は bare flag（`-A, --show-all` のみの行）+ indented description 形式
- カラム検出でテーブルにならない（BlockSingle/BlockProse）が、`inferSectionFromContent` で `-` 始まりの行を数えて options 分類し、`parseOptionBlockBare` で flag 行 + continuation 行として処理
- **ルール**: 新 CLI 形式で options=0 になったら、まずブロック分類（`classifyBlocks`）の結果をデバッグし、section が正しく `"options"` になっているか確認する

## 2026-03-21: TUI 3ペイン化 + ダッシュ区切りパーサー修正

### 3ペインレイアウトで見切れ問題を根本解決

- 2ペイン構成（ツリー+詳細）ではツリーに説明文をインライン表示していたため、横方向も縦方向も表示領域が不足していた
- **ルール**: ツリーペインは名前のみ表示し、説明・Usage はサマリーペイン、サブコマンド・オプション一覧はディテールペインに分離する。ツリー幅はコンテンツから自動計算する

### ツリーペイン幅の自動計算

- 固定比率（40%）では短いコマンド名に無駄なスペース、長い名前に不足が生じる
- **ルール**: `treeLineWidth()` で全アイテムの最長行幅を計算し、ツリーペイン幅を自動調整する。上限はターミナル幅の50%

### ダッシュ区切りコマンドの固定カラム分割バグ

- apt の `"  list - description"` 形式では `" - "` の位置がコマンド名の長さにより変動する。`detectColumns` の modal カラム位置で固定分割すると、長い名前（`reinstall`）が途中で切断される
- **ルール**: `SepDash` ブロックでは `splitAtColumn(line, b.DescCol)` を使わず、各行ごとに `" - "` で直接分割する

### スクロールインジケーターの重要性

- ツリーペインにスクロール可能な項目が上下にあっても、ユーザーにはそれが見えない
- **ルール**: 表示範囲外にアイテムがある場合は `▲ N more above` / `▼ N more below` を表示する。ディテールペインも同様に上下のインジケーターを出す

## 2026-03-21: パーサーカバレッジ改善（86.1% → 88.0%）

### ダッシュ区切りブロックのカラム不整列への対処

- apt のように各行の `" - "` 位置がコマンド名の長さにより異なるブロックは、`detectColumns` のモーダルカラム判定で `BlockTable` にならず `BlockProse` に分類される
- **ルール**: `detectColumns` でモーダルカラム不一致でも、ダッシュセパレータの行が過半数なら `BlockTable` + `SepDash` として認識する。`SepDash` ブロックは行ごとに分割するため固定カラム不要

### フォールバック関数の適用条件は厳密に

- `extractCategoryCommands`（snap 形式）を `len(node.Children) == 0` だけで発動させると、ls の `values: name, none, time, size, ...` のようなオプション説明内のカンマ区切りリストがサブコマンドとして誤認される
- **ルール**: 構造がまったく見つからない場合（children=0 AND options=0）のみフォールバックを発動させる。既に別の構造（options 等）が見つかっている場合はスキップ

### bracket オプションの `[--name value]` パターン

- `bracketLongRe` (`\[--([\w-]+)\]`) と `bracketLongArgRe` (`\[--([\w-]+)=([\w]+)\]`) は `[--uid id]`（スペース区切り引数）にマッチしない
- **ルール**: `bracketLongSpaceArgRe` (`\[--([\w-]+)\s+(\w+)\]`) を追加し、`bracketLongRe` のマッチから `[--name ` パターンを除外する

### ディスカバリーテストは build tag で隔離する

- `discover_test.go` は 1,200+ コマンドの `--help` を実行するため 2分以上かかり、`go test ./...` がハングする
- **ルール**: 長時間テストには `//go:build discover` タグを付与し、通常の `go test ./...` から除外する。実行時は `go test -tags discover` を指定

### スモークテストのコマンド実行にはタイムアウト必須

- `gemini --help` がハングし、スモークテスト全体がブロックされた
- **ルール**: 外部コマンド実行には `context.WithTimeout` で 10 秒タイムアウトを設定する。ハングするコマンドは skip として扱う

## 2026-03-21: macOS ディスカバリーテスト + パーサー改善（75.4% → 78.2%）

### 巨大入力に対するサイズガードが必須

- `instmodsh --help` (926MB)、`yes --help` (4.3GB) がパーサーに渡され、正規表現処理でハング（PARSE_TIMEOUT）
- `extractUsageOptions()` が全行を1文字列に join してから regex を適用するため、巨大入力で致命的
- **ルール**: `Parse()` 冒頭で入力サイズを制限する（1MB上限）。実際の `--help` 出力は最大でも数十KB

### ディスカバリーテストには進捗ログとパーサータイムアウトを入れる

- macOS で 1,174 コマンドを処理すると数分かかり、途中のハングを検出できない
- **ルール**: コマンドごとに `t.Logf("[N/total] cmd ... -> STATUS")` で進捗出力。パーサーは goroutine + select で 10 秒タイムアウトし、`PARSE_TIMEOUT` としてスキップする

### macOS (BSD) と WSL (GNU) でヘルプ形式が大きく異なる

- WSL: GNU coreutils が主流。`--help` に構造化されたセクション・オプション表を持つ → OK 率 88.0%
- macOS: BSD ツールが主流。`usage:` 行のみの簡素なヘルプ、`illegal option` エラー付き → OK 率 75.4%（改善後 78.2%）
- **ルール**: 両環境でディスカバリーテストを実施して OS 固有のパターンを把握する。BSD 形式の改善は `extractUsageOptions()` のフォールバック正規表現で対応

### 埋め込み usage パターンへの対応

- `top`: `/usr/bin/top usage: /usr/bin/top` — `usage:` が行頭でなく途中に出現
- `zic`: `usage is zic [...]` — コロンなしの `usage is` 形式
- **ルール**: `usageRe` (`^usage:`) でマッチしない場合、`embeddedUsageRe` (`\busage(?:\s+is)?[:\s]`) でフォールバック検出する

### スペース付きブラケットの正規化

- `zic` の `[ --version ]` のように、ブラケット内にスペースパディングがある形式は既存の `bracketLongRe` にマッチしない
- **ルール**: `extractUsageOptions()` で usageText を正規化（`[ ` → `[`、` ]` → `]`）してから正規表現を適用する

### サンドボックスと Go ビルドキャッシュの干渉

- Claude Code のサンドボックスが `TMPDIR` を書き換えるため、Go の work dir (`/tmp/claude`) が見つからずビルドが失敗する
- **ルール**: Go テスト実行時にサンドボックスが干渉する場合は `dangerouslyDisableSandbox: true` で実行する。または `/sandbox` でサンドボックスを無効化する
