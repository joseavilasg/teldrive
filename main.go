package main

import (
	"fmt"
	"mime"
	"time"

	"github.com/divyam234/drive/database"
	"github.com/divyam234/drive/routes"
	"github.com/divyam234/drive/ui"
	"github.com/divyam234/drive/utils"

	"github.com/divyam234/cors"
	"github.com/divyam234/drive/utils/cache"
	"github.com/gin-gonic/gin"
)

func main() {

	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	utils.InitConfig()

	utils.InitializeLogger()

	database.InitDB()

	cache.InitCache()

	router.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Length", "Content-Type"},
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		MaxAge: 12 * time.Hour,
	}))

	mime.AddExtensionType(".js", "application/javascript")

	router.Use(gin.ErrorLogger())

	routes.AddRoutes(router)

	ui.AddRoutes(router)

	config := utils.GetConfig()
	router.Run(fmt.Sprintf(":%d", config.Port))
}
