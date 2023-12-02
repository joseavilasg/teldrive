package models

import (
	"time"
)

type User struct {
	Id        int       `gorm:"type:serial;primaryKey"`
	FullName  string    `gorm:"type:text"`
	UserName  string    `gorm:"type:text"`
	Email     string    `gorm:"type:text"`
	Password  string    `gorm:"type:text"`
	UpdatedAt time.Time `gorm:"default:timezone('utc'::text, now())"`
	CreatedAt time.Time `gorm:"default:timezone('utc'::text, now())"`
}
