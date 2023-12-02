package routes

import (
	"github.com/gin-gonic/gin"
)

func AddRoutes(router *gin.Engine) {
	api := router.Group("/api")

	addAuthRoutes(api)
	addFileRoutes(api)
	addUploadRoutes(api)
	addUserRoutes(api)
}
