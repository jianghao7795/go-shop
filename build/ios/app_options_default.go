//go:build ignore

// 本文件是 Wails iOS 脚手架的默认（非 iOS）分支，但当前桌面应用未接入 iOS 构建。
// 它是 package main 却没有 main()，会让 go build ./... 报「function main is undeclared」；
// 根 main.go 也并未调用 modifyOptionsForIOS，故排除出构建。

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// modifyOptionsForIOS is a no-op on non-iOS platforms
func modifyOptionsForIOS(opts *application.Options) {
	// No modifications needed for non-iOS platforms
}