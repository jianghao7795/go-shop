package model

import (
	"encoding/json"
	"fmt"
	"math"
)

// Price 表示以「分」为单位的金额，底层为整数以避免浮点精度误差。
// JSON 序列化时对外输出「元」的浮点数（如 19900 → 199），输入时反向转换。
type Price int64

// MarshalJSON 把「分」输出为「元」的浮点数。
func (p Price) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(p) / 100)
}

// UnmarshalJSON 把「元」的浮点数解析为「分」。
func (p *Price) UnmarshalJSON(b []byte) error {
	var yuan float64
	if err := json.Unmarshal(b, &yuan); err != nil {
		return err
	}
	*p = Price(int64(math.Round(yuan * 100)))
	return nil
}

// YuanString 返回以元为单位、保留两位小数的字符串（用于支付金额）。
func (p Price) YuanString() string {
	return fmt.Sprintf("%.2f", float64(p)/100)
}
