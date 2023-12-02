package schemas

import (
	"time"

	"github.com/divyam234/drive/models"
)

type PaginationQuery struct {
	PerPage       int    `form:"perPage"`
	NextPageToken string `form:"nextPageToken"`
}

type SortingQuery struct {
	Sort  string `form:"sort"`
	Order string `form:"order"`
}

type FileQuery struct {
	Name      string     `form:"name" mapstructure:"name,omitempty"`
	Search    string     `form:"search" mapstructure:"search,omitempty"`
	Type      string     `form:"type" mapstructure:"type,omitempty"`
	Path      string     `form:"path" mapstructure:"path,omitempty"`
	Op        string     `form:"op" mapstructure:"op,omitempty"`
	Starred   *bool      `form:"starred" mapstructure:"starred,omitempty"`
	MimeType  string     `form:"mimeType" mapstructure:"mime_type,omitempty"`
	ParentID  string     `form:"parentId" mapstructure:"parent_id,omitempty"`
	UpdatedAt *time.Time `form:"updatedAt" mapstructure:"updated_at,omitempty"`
	Status    string     `mapstructure:"status"`
	UserID    int        `mapstructure:"user_id"`
}

type FileIn struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	Parts    *models.Parts `json:"parts,omitempty"`
	MimeType string        `json:"mimeType"`
	Path     string        `json:"path"`
	Size     int64         `json:"size"`
	Starred  *bool         `json:"starred"`
	Depth    *int          `json:"depth,omitempty"`
	Status   string        `json:"status,omitempty"`
	UserID   int           `json:"userId"`
	ParentID string        `json:"parentId"`
}

type FileOut struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	MimeType  string    `json:"mimeType"`
	Path      string    `json:"path,omitempty"`
	Size      int64     `json:"size,omitempty"`
	Starred   *bool     `json:"starred"`
	ParentID  string    `json:"parentId,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type FileResponse struct {
	Results       []FileOut `json:"results"`
	NextPageToken string    `json:"nextPageToken,omitempty"`
}

type FileOutFull struct {
	FileOut
	Parts     *models.Parts `json:"parts,omitempty"`
	ChannelID *int64        `json:"channelId"`
}

type FileOperation struct {
	Files       []string `json:"files"`
	Destination string   `json:"destination,omitempty"`
}

type DirMove struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type MkDir struct {
	Path string `json:"path"`
}

type Copy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Destination string `json:"destination"`
}
