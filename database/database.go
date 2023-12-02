package database

import (
	"embed"
	"log"
	"os"
	"time"

	"github.com/divyam234/drive/utils"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS
var DB *gorm.DB

func InitDB() {

	var err error

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Silent,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)

	DB, err = gorm.Open(postgres.Open(utils.GetConfig().DatabaseUrl), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "drive.",
			SingularTable: false,
		},
		PrepareStmt: false,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		Logger: newLogger,
	})
	if err != nil {
		panic(err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	sqlDB.SetConnMaxLifetime(time.Hour)
	go func() {
		//DB.Exec(`create collation if not exists numeric (provider = icu, locale = 'en@colnumeric=yes');`)
		if utils.GetConfig().RunMigrations {
			migrate()
		}
	}()
}

func migrate() {

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}
	db, _ := DB.DB()
	if err := goose.Up(db, "migrations"); err != nil {
		panic(err)
	}
}
