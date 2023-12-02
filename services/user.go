package services

import (
	"errors"
	"net/http"

	"github.com/divyam234/drive/models"
	"github.com/divyam234/drive/schemas"
	"github.com/divyam234/drive/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserService struct {
	Db *gorm.DB
}

func (us *UserService) Stats(c *gin.Context) (*schemas.AccountStats, *types.AppError) {
	userId := getUserId(c)
	var res []schemas.AccountStats
	if err := us.Db.Model(&models.File{}).Select("count(*) as total_size ", "sum(size) as total_files").Where("user_id = ?", userId).
		Where("type = ?", "file").Where("status = ?", "active").Find(&res).Error; err != nil {
		return nil, &types.AppError{Error: errors.New("failed to get stats"), Code: http.StatusInternalServerError}
	}
	return &res[0], nil
}
