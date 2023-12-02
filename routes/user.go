package routes

import (
	"net/http"

	"github.com/divyam234/drive/database"
	"github.com/divyam234/drive/services"

	"github.com/gin-gonic/gin"
)

func addUserRoutes(rg *gin.RouterGroup) {
	r := rg.Group("/users")
	r.Use(Authmiddleware)
	userService := services.UserService{Db: database.DB}

	r.GET("/stats", func(c *gin.Context) {
		res, err := userService.Stats(c)

		if err != nil {
			c.AbortWithError(err.Code, err.Error)
			return
		}
		c.JSON(http.StatusOK, res)
	})
}
