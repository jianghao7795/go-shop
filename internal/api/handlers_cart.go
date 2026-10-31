package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"shop/internal/model"
)

const maxCartQuantity = 999
const maxCartLines = 100

type cartInput struct {
	ProductID int `json:"productId"`
	SkuID     int `json:"skuId"`
	Quantity  int `json:"quantity"`
}

type cartItemView struct {
	cartInput
	Specs     string      `json:"specs"`
	Name      string      `json:"name"`
	Price     model.Price `json:"price"`
	Emoji     string      `json:"emoji"`
	Color     string      `json:"color"`
	Stock     int         `json:"stock"`
	Available bool        `json:"available"`
	Reason    string      `json:"reason"`
}

type cartView struct {
	Version uint64         `json:"version"`
	Items   []cartItemView `json:"items"`
}

type shopError struct {
	status  int
	message string
}

func (e *shopError) Error() string { return e.message }
func badCart(message string) error { return &shopError{http.StatusBadRequest, message} }
func cartConflict() error {
	return &shopError{http.StatusConflict, "购物车已更新，请刷新后重试"}
}
func writeShopError(c *gin.Context, err error) {
	var problem *shopError
	if errors.As(err, &problem) {
		c.JSON(problem.status, gin.H{"message": problem.message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"message": "服务暂不可用，请重试"})
}

func validateCartInput(items []cartInput) error {
	if len(items) > maxCartLines {
		return badCart("购物车最多支持100种商品")
	}
	seen := map[[2]int]bool{}
	for _, it := range items {
		if it.ProductID <= 0 || it.SkuID < 0 || it.Quantity <= 0 || it.Quantity > maxCartQuantity {
			return badCart("商品或数量不合法（数量须为1至999）")
		}
		key := [2]int{it.ProductID, it.SkuID}
		if seen[key] {
			return badCart("商品规格重复")
		}
		seen[key] = true
	}
	return nil
}

func ensureCartState(db *gorm.DB, user string) error {
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.CartState{UserID: user}).Error
}

func advanceCart(tx *gorm.DB, user string, version uint64) error {
	result := tx.Model(&model.CartState{}).Where("user_id = ? AND version = ?", user, version).UpdateColumn("version", gorm.Expr("version + 1"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return cartConflict()
	}
	return nil
}

// Keep deleted product names/specs available for explaining invalid cart rows.
func cartViews(db *gorm.DB, items []cartInput) ([]cartItemView, error) {
	views := make([]cartItemView, 0, len(items))
	if len(items) == 0 {
		return views, nil
	}
	ids := make([]int, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ProductID)
	}
	var products []model.Product
	if err := db.Unscoped().Where("id IN ?", ids).Find(&products).Error; err != nil {
		return nil, err
	}
	var skus []model.ProductSku
	if err := db.Unscoped().Where("product_id IN ?", ids).Find(&skus).Error; err != nil {
		return nil, err
	}
	ps := map[int]model.Product{}
	ss := map[int]model.ProductSku{}
	hasSkus := map[int]bool{}
	for _, p := range products {
		ps[p.ID] = p
	}
	for _, s := range skus {
		ss[s.ID] = s
		if !s.DeletedAt.Valid {
			hasSkus[s.ProductID] = true
		}
	}
	for _, it := range items {
		p, exists := ps[it.ProductID]
		v := cartItemView{cartInput: it, Name: p.Name, Price: p.Price, Emoji: p.Emoji, Color: p.Color, Stock: p.Stock}
		if v.Name == "" {
			v.Name = "已失效商品"
		}
		sku, skuExists := ss[it.SkuID]
		if it.SkuID > 0 && skuExists && sku.ProductID == it.ProductID {
			v.Specs, v.Price, v.Stock = sku.Specs, sku.Price, sku.Stock
		}
		switch {
		case !exists || p.DeletedAt.Valid:
			v.Reason = "商品已删除"
		case !p.OnShelf:
			v.Reason = "商品已下架"
		case it.SkuID > 0 && (!skuExists || sku.ProductID != it.ProductID || sku.DeletedAt.Valid):
			v.Reason = "商品规格已失效"
		case it.SkuID == 0 && hasSkus[it.ProductID]:
			v.Reason = "请重新选择商品规格"
		case v.Stock <= 0:
			v.Reason = "商品已售罄"
		case it.Quantity > v.Stock:
			v.Reason = "库存不足，请调整数量"
		}
		v.Stock = max(0, v.Stock)
		v.Available = v.Reason == ""
		views = append(views, v)
	}
	return views, nil
}

func readCart(db *gorm.DB, user string) (cartView, error) {
	var state model.CartState
	if err := db.Where("user_id = ?", user).First(&state).Error; err != nil {
		return cartView{}, err
	}
	var rows []model.CartItem
	if err := db.Where("user_id = ?", user).Order("id ASC").Find(&rows).Error; err != nil {
		return cartView{}, err
	}
	items := make([]cartInput, 0, len(rows))
	for _, row := range rows {
		items = append(items, cartInput{row.ProductID, row.SkuID, row.Quantity})
	}
	views, err := cartViews(db, items)
	return cartView{Version: state.Version, Items: views}, err
}

func getCart(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		user := currentUser(c)
		if err := ensureCartState(db, user); err != nil {
			writeShopError(c, err)
			return
		}
		var result cartView
		err := db.Transaction(func(tx *gorm.DB) error { var err error; result, err = readCart(tx, user); return err })
		if err != nil {
			writeShopError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// Read-only preview refreshes guest carts without persisting them.
func previewCart(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		var items []cartInput
		if c.ShouldBindJSON(&items) != nil {
			writeShopError(c, badCart("请求格式错误"))
			return
		}
		if err := validateCartInput(items); err != nil {
			writeShopError(c, err)
			return
		}
		views, err := cartViews(db, items)
		if err != nil {
			writeShopError(c, err)
			return
		}
		c.JSON(http.StatusOK, views)
	}
}

func putCart(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		var req struct {
			Version *uint64     `json:"version"`
			Items   []cartInput `json:"items"`
		}
		if c.ShouldBindJSON(&req) != nil || req.Version == nil || req.Items == nil {
			writeShopError(c, badCart("请提供购物车版本和商品列表"))
			return
		}
		if err := validateCartInput(req.Items); err != nil {
			writeShopError(c, err)
			return
		}
		user := currentUser(c)
		if err := ensureCartState(db, user); err != nil {
			writeShopError(c, err)
			return
		}
		var result cartView
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := advanceCart(tx, user, *req.Version); err != nil {
				return err
			}
			var old []model.CartItem
			if err := tx.Where("user_id = ?", user).Find(&old).Error; err != nil {
				return err
			}
			existing := map[[2]int]int{}
			for _, row := range old {
				existing[[2]int{row.ProductID, row.SkuID}] = row.Quantity
			}
			views, err := cartViews(tx, req.Items)
			if err != nil {
				return err
			}
			for _, v := range views {
				// Retain invalid existing rows so they remain visible and removable.
				if !v.Available && v.Quantity > existing[[2]int{v.ProductID, v.SkuID}] {
					return badCart(v.Name + "：" + v.Reason)
				}
			}
			if err := tx.Where("user_id = ?", user).Delete(&model.CartItem{}).Error; err != nil {
				return err
			}
			for _, it := range req.Items {
				if err := tx.Create(&model.CartItem{UserID: user, ProductID: it.ProductID, SkuID: it.SkuID, Quantity: it.Quantity}).Error; err != nil {
					return err
				}
			}
			result = cartView{Version: *req.Version + 1, Items: views}
			return nil
		})
		if err != nil {
			writeShopError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
