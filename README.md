# helptree

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Interactive TUI viewer that visualizes CLI `--help` output as a navigable tree structure.

> **The problem:** Modern CLI tools (`docker`, `kubectl`, `gh`, `ffmpeg`, ...) have hundreds of subcommands and options. Reading `--help` output is like scrolling through a wall of text.
>
> **helptree** turns that wall into an interactive, hierarchical map you can explore without leaving the terminal.

## Features

- **Tree view** — Browse subcommands as an expandable/collapsible tree
- **Detail pane** — See options, usage, and description for the selected command
- **Lazy loading** — Subcommand help is fetched on demand (no upfront delay)
- **Incremental search** — Press `/` to filter commands instantly
- **Vim-style navigation** — `j`/`k`, `g`/`G`, `h`/`l` — feels like home

## Install

```bash
go install github.com/kwrkb/helptree@latest
```

## Usage

```bash
helptree <command>
```

```bash
helptree docker
helptree gh
helptree kubectl
helptree go
```

## Keybindings

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` / `l` | Expand / Load subcommands |
| `h` | Collapse |
| `Space` | Toggle expand/collapse |
| `g` / `G` | Jump to top / bottom |
| `/` | Search |
| `n` / `N` | Next / Previous match |
| `Ctrl+d` / `Ctrl+u` | Scroll detail pane |
| `?` | Show keybinding help |
| `q` | Quit |

## How it works

1. Runs `<command> --help` and parses the output to extract subcommands and options
2. Displays the result as a two-pane TUI (tree + detail)
3. On expand, recursively runs `--help` on subcommands to build deeper levels (lazy loading)

Supports GNU-style, Go-style (tab-separated), and kubectl-style (parenthesized section headers) help formats.

## Build from source

```bash
git clone https://github.com/kwrkb/helptree.git
cd helptree
go build -o helptree .
```

## License

[MIT](LICENSE)
