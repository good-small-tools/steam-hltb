package main

import (
	"fmt"
	"os"
)

// tryOpen は openInBrowser を呼び、失敗してもメッセージを出すだけで続行する。
// openInBrowser の実装は OS ごとに open_windows.go / open_unix.go に分かれている。
func tryOpen(target string) {
	if err := openInBrowser(target); err != nil {
		fmt.Fprintf(os.Stderr, "  （ブラウザの自動起動に失敗しました: %v）\n", err)
	}
}
