package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// cartItemView 是购物车项视图（含商品快照与 SKU 信息）。
type cartItemView struct {
	ProductID int         `json:"productId"`
	SkuID     int         `json:"skuId"`
	Specs     string      `json:"specs"`
	Quantity  int         `json:"quantity"`
	Name      string      `json:"name"`
	Price     model.Price `json:"price"`
	Emoji     string      `json:"emoji"`
	Color     string      `json:"color"`
}

// getCart 返回当前用户的购物车（含商品快照）。
func getCart(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		var items []model.CartItem
		db.Where("user_id = ?", currentUser(c)).Order("id ASC").Find(&items)
		products := map[int]model.Product{}
		skus := map[int]model.ProductSku{}
		if len(items) > 0 {
			pids := make([]int, 0, len(items))
			sids := make([]int, 0)
			for _, it := range items {
				pids = append(pids, it.ProductID)
				if it.SkuID > 0 {
					sids = append(sids, it.SkuID)
				}
			}
			var ps []model.Product
			db.Where("id IN ?", pids).Find(&ps)
			for _, p := range ps {
				products[p.ID] = p
			}
			if len(sids) > 0 {
				var ss []model.ProductSku
				db.Where("id IN ?", sids).Find(&ss)
				for _, s := range ss {
					skus[s.ID] = s
				}
			}
		}
		views := make([]cartItemView, 0, len(items))
		for _, it := range items {
			p := products[it.ProductID]
			v := cartItemView{
				ProductID: it.ProductID,
				SkuID:     it.SkuID,
				Quantity:  it.Quantity,
				Name:      p.Name,
				Price:     p.Price,
				Emoji:     p.Emoji,
				Color:     p.Color,
			}
			if it.SkuID > 0 {
				if s, ok := skus[it.SkuID]; ok {
					v.Specs = s.Specs
					v.Price = s.Price
				}
			}
			views = append(views, v)
		}
		c.JSON(http.StatusOK, views)
	}
}

// putCart 全量同步购物车：以客户端提交的项替换当前用户的购物车。
func putCart(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		var req []struct {
			ProductID int `json:"productId"`
			SkuID     int `json:"skuId"`
			Quantity  int `json:"quantity"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请求格式错误"})
			return
		}
		user := currentUser(c)
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("user_id = ?", user).Delete(&model.CartItem{}).Error; err != nil {
				return err
			}
			for _, it := range req {
				if it.Quantity <= 0 {
					continue
				}
				if err := tx.Create(&model.CartItem{UserID: user, ProductID: it.ProductID, SkuID: it.SkuID, Quantity: it.Quantity}).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "同步失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "已同步"})
	}
}
