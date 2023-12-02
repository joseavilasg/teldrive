package routes

import (
	"net/http"

	"github.com/divyam234/drive/database"
	"github.com/divyam234/drive/services"

	"github.com/gin-gonic/gin"
)

func addUploadRoutes(rg *gin.RouterGroup) {

	r := rg.Group("/uploads")
	r.Use(Authmiddleware)

	uploadService := services.UploadService{Db: database.DB}

	r.GET("/:id", func(c *gin.Context) {

		res, err := uploadService.GetUploadFileById(c)

		if err != nil {
			c.AbortWithError(http.StatusNotFound, err.Error)
			return
		}

		c.JSON(http.StatusOK, res)
	})

	r.POST("/:id", func(c *gin.Context) {
		res, err := uploadService.CreateUploadPart(c)

		if err != nil {
			c.AbortWithError(err.Code, err.Error)
			return
		}

		c.JSON(http.StatusOK, res)
	})

	r.DELETE("/:id", func(c *gin.Context) {
		err := uploadService.DeleteUploadFile(c)

		if err != nil {
			c.AbortWithError(err.Code, err.Error)
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "upload  deleted"})
	})

}
