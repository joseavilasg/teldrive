package services

import (
	"errors"
	"net/http"
	"time"

	"github.com/divyam234/drive/mapper"
	"github.com/divyam234/drive/schemas"
	"github.com/divyam234/drive/types"

	"github.com/divyam234/drive/utils"

	"github.com/divyam234/drive/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UploadService struct {
	Db *gorm.DB
}

func (us *UploadService) GetUploadFileById(c *gin.Context) ([]schemas.UploadPartOut, *types.AppError) {
	uploadId := c.Param("id")
	parts := []schemas.UploadPartOut{}
	config := utils.GetConfig()
	if err := us.Db.Model(&models.Upload{}).Order("part_no").Where("upload_id = ?", uploadId).
		Where("created_at >= ?", time.Now().UTC().AddDate(0, 0, -config.UploadRetention)).
		Find(&parts).Error; err != nil {
		return nil, &types.AppError{Error: errors.New("failed to fetch from db"), Code: http.StatusInternalServerError}
	}

	return parts, nil
}

func (us *UploadService) DeleteUploadFile(c *gin.Context) *types.AppError {
	uploadId := c.Param("id")
	if err := us.Db.Where("upload_id = ?", uploadId).Delete(&models.Upload{}).Error; err != nil {
		return &types.AppError{Error: errors.New("failed to delete upload"), Code: http.StatusInternalServerError}
	}

	return nil
}

func (us *UploadService) CreateUploadPart(c *gin.Context) (*schemas.UploadPartOut, *types.AppError) {

	userId := getUserId(c)

	uploadId := c.Param("id")

	var payload schemas.UploadPart

	if err := c.ShouldBindJSON(&payload); err != nil {
		return nil, &types.AppError{Error: errors.New("invalid request payload"), Code: http.StatusBadRequest}
	}

	partUpload := &models.Upload{
		Name:     payload.Name,
		UploadId: uploadId,
		Url:      payload.Url,
		Size:     payload.Size,
		PartNo:   payload.PartNo,
		UserId:   userId,
	}

	if err := us.Db.Create(partUpload).Error; err != nil {
		return nil, &types.AppError{Error: err, Code: http.StatusInternalServerError}
	}

	out := mapper.MapUploadSchema(partUpload)

	return out, nil
}
