//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// openInBrowser は Windows のシェル（ShellExecuteW）で target を既定ハンドラ
// （ブラウザ）に直接渡して開く。cmd.exe を経由しないため、サブプロセス起動の
// オーバーヘッドやコンソールウィンドウの点滅がなく、エクスプローラーで
// ファイルをダブルクリックするのと同じ最短経路で開く。
func openInBrowser(target string) error {
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}

	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecute := shell32.NewProc("ShellExecuteW")

	const swShowNormal = 1
	// ShellExecuteW は成功時に 32 より大きい値を返す（< 33 はエラーコード）。
	ret, _, callErr := shellExecute.Call(
		0, // hwnd
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		0, // parameters
		0, // directory
		uintptr(swShowNormal),
	)
	if ret <= 32 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return fmt.Errorf("ShellExecute が失敗しました（コード %d）", ret)
	}
	return nil
}
