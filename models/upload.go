package models

import (
	"time"
)

type Upload struct {
	UploadId  string    `gorm:"type:text"`
	UserId    int       `gorm:"type:integer"`
	Name      string    `gorm:"type:text"`
	PartNo    int       `gorm:"type:integer"`
	Url       string    `gorm:"type:text"`
	Size      int64     `gorm:"type:bigint"`
	CreatedAt time.Time `gorm:"default:timezone('utc'::text, now())"`
}
