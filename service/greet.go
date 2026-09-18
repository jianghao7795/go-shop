package service

import "fmt"

// GreetService 提供一个简单的问候方法，作为 Wails 绑定的示例服务。
type GreetService struct{}

// Greet 返回针对 name 的问候语。
func (g *GreetService) Greet(name string) string {
	return fmt.Sprintf("Hello, %s!, It's show time", name)
}
