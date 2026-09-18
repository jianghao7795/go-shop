package service

import "github.com/skip2/go-qrcode"

// QRService 提供二维码生成能力，作为 Wails 绑定的示例服务。
type QRService struct{}

// NewQRService 创建一个新的 QRService。
func NewQRService() *QRService {
	return &QRService{}
}

// Generate 根据文本生成 PNG 格式的二维码。
func (s *QRService) Generate(text string, size int) ([]byte, error) {
	qr, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	return qr.PNG(size)
}
