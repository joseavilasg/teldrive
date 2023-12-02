package services

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/divyam234/drive/mapper"
	"github.com/divyam234/drive/models"
	"github.com/divyam234/drive/schemas"
	"github.com/divyam234/drive/types"

	"github.com/divyam234/drive/utils"
	"github.com/divyam234/drive/utils/cache"
	"github.com/divyam234/drive/utils/md5"
	"github.com/divyam234/drive/utils/reader"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mitchellh/mapstructure"
	range_parser "github.com/quantumsheep/range-parser"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FileService struct {
	Db *gorm.DB
}

func (fs *FileService) CreateFile(c *gin.Context) (*schemas.FileOut, *types.AppError) {
	userId := getUserId(c)
	var fileIn schemas.FileIn
	if err := c.ShouldBindJSON(&fileIn); err != nil {
		return nil, &types.AppError{Error: errors.New("invalid request payload"), Code: http.StatusBadRequest}
	}

	fileIn.Path = strings.TrimSpace(fileIn.Path)

	if fileIn.Path != "" {
		var parent models.File
		if err := fs.Db.Where("type = ? AND path = ?", "folder", fileIn.Path).First(&parent).Error; err != nil {
			return nil, &types.AppError{Error: errors.New("parent directory not found"), Code: http.StatusNotFound}
		}
		fileIn.ParentID = parent.ID
	}

	if fileIn.Type == "folder" {
		fileIn.MimeType = "drive/folder"
		var fullPath string
		if fileIn.Path == "/" {
			fullPath = "/" + fileIn.Name
		} else {
			fullPath = fileIn.Path + "/" + fileIn.Name
		}
		fileIn.Path = fullPath
		fileIn.Depth = utils.IntPointer(len(strings.Split(fileIn.Path, "/")) - 1)
	} else if fileIn.Type == "file" {
		fileIn.Path = ""
	}

	fileIn.UserID = userId
	fileIn.Starred = utils.BoolPointer(false)
	fileIn.Status = "active"

	fileDb := mapper.MapFileInToFile(fileIn)

	if err := fs.Db.Create(&fileDb).Error; err != nil {
		pgErr := err.(*pgconn.PgError)
		if pgErr.Code == "23505" {
			return nil, &types.AppError{Error: errors.New("file exists"), Code: http.StatusBadRequest}
		}
		return nil, &types.AppError{Error: errors.New("failed to create a file"), Code: http.StatusBadRequest}

	}

	res := mapper.MapFileToFileOut(fileDb)

	return &res, nil
}

func (fs *FileService) UpdateFile(c *gin.Context) (*schemas.FileOut, *types.AppError) {

	fileID := c.Param("fileID")

	var fileUpdate schemas.FileIn

	var files []models.File

	if err := c.ShouldBindJSON(&fileUpdate); err != nil {
		return nil, &types.AppError{Error: errors.New("invalid request payload"), Code: http.StatusBadRequest}
	}

	if fileUpdate.Type == "folder" && fileUpdate.Name != "" {
		if err := fs.Db.Raw("select * from drive.update_folder(?, ?)", fileID, fileUpdate.Name).Scan(&files).Error; err != nil {
			return nil, &types.AppError{Error: errors.New("failed to update the file"), Code: http.StatusInternalServerError}
		}
	} else {
		fileDb := mapper.MapFileInToFile(fileUpdate)
		if err := fs.Db.Model(&files).Clauses(clause.Returning{}).Where("id = ?", fileID).Updates(fileDb).Error; err != nil {
			return nil, &types.AppError{Error: errors.New("failed to update the file"), Code: http.StatusInternalServerError}
		}
	}

	if len(files) == 0 {
		return nil, &types.AppError{Error: errors.New("file not updated"), Code: http.StatusNotFound}
	}

	file := mapper.MapFileToFileOut(files[0])

	key := fmt.Sprintf("files:%s", fileID)

	cache.GetCache().Delete(key)

	return &file, nil

}

func (fs *FileService) GetFileByID(c *gin.Context) (*schemas.FileOutFull, error) {

	fileID := c.Param("fileID")

	var file []models.File

	fs.Db.Model(&models.File{}).Where("id = ?", fileID).Find(&file)

	if len(file) == 0 {
		return nil, errors.New("file not found")
	}

	return mapper.MapFileToFileOutFull(file[0]), nil
}

func (fs *FileService) ListFiles(c *gin.Context) (*schemas.FileResponse, *types.AppError) {

	userId := getUserId(c)

	var pagingParams schemas.PaginationQuery
	pagingParams.PerPage = 200
	if err := c.ShouldBindQuery(&pagingParams); err != nil {
		return nil, &types.AppError{Error: errors.New("invalid params"), Code: http.StatusBadRequest}
	}

	var sortingParams schemas.SortingQuery
	sortingParams.Order = "asc"
	sortingParams.Sort = "name"
	if err := c.ShouldBindQuery(&sortingParams); err != nil {
		return nil, &types.AppError{Error: errors.New("invalid params"), Code: http.StatusBadRequest}
	}

	var fileQuery schemas.FileQuery
	fileQuery.Op = "list"
	fileQuery.Status = "active"
	fileQuery.UserID = userId
	if err := c.ShouldBindQuery(&fileQuery); err != nil {
		return nil, &types.AppError{Error: errors.New("invalid params"), Code: http.StatusBadRequest}
	}

	var (
		pathId string
		err    error
	)
	if fileQuery.Path != "" {
		pathId, err = fs.getPathId(fileQuery.Path)
		if err != nil {
			return nil, &types.AppError{Error: err, Code: http.StatusNotFound}
		}
	}

	query := fs.Db.Model(&models.File{}).Limit(pagingParams.PerPage).
		Where(map[string]interface{}{"user_id": userId, "status": "active"})

	if fileQuery.Op == "list" {
		setOrderFilter(query, &pagingParams, &sortingParams)

		query.Order("type DESC").Order(getOrder(sortingParams)).
			Where("parent_id = ?", pathId)

	} else if fileQuery.Op == "find" {

		filterQuery := map[string]interface{}{}

		err := mapstructure.Decode(fileQuery, &filterQuery)

		if err != nil {
			return nil, &types.AppError{Error: err, Code: http.StatusBadRequest}
		}

		delete(filterQuery, "op")

		if filterQuery["updated_at"] == nil {
			delete(filterQuery, "updated_at")
		}

		if filterQuery["path"] != nil && filterQuery["name"] != nil {
			query.Where("parent_id = ?", pathId)
			delete(filterQuery, "path")
		}

		setOrderFilter(query, &pagingParams, &sortingParams)

		query.Order("type DESC").Order(getOrder(sortingParams)).Where(filterQuery)

	} else if fileQuery.Op == "search" {

		query.Where("drive.get_tsquery(?) @@ drive.get_tsvector(name)", fileQuery.Search)

		setOrderFilter(query, &pagingParams, &sortingParams)
		query.Order(getOrder(sortingParams))

	}

	var results []schemas.FileOut

	query.Find(&results)

	token := ""

	if len(results) == pagingParams.PerPage {
		lastItem := results[len(results)-1]
		token = utils.GetField(&lastItem, utils.CamelToPascalCase(sortingParams.Sort))
		token = base64.StdEncoding.EncodeToString([]byte(token))
	}

	res := &schemas.FileResponse{Results: results, NextPageToken: token}

	return res, nil
}

func (fs *FileService) getPathId(path string) (string, error) {

	var file models.File

	if err := fs.Db.Model(&models.File{}).Select("id").Where("path = ?", path).First(&file).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return "", errors.New("path not found")

	}
	return file.ID, nil
}

func (fs *FileService) MakeDirectory(c *gin.Context) (*schemas.FileOut, *types.AppError) {

	var payload schemas.MkDir

	var files []models.File

	if err := c.ShouldBindJSON(&payload); err != nil {
		return nil, &types.AppError{Error: errors.New("invalid request payload"), Code: http.StatusBadRequest}
	}

	userId := getUserId(c)
	if err := fs.Db.Raw("select * from drive.create_directories(?, ?)", userId, payload.Path).Scan(&files).Error; err != nil {
		return nil, &types.AppError{Error: errors.New("failed to create directories"), Code: http.StatusInternalServerError}
	}

	file := mapper.MapFileToFileOut(files[0])

	return &file, nil

}

func (fs *FileService) MoveFiles(c *gin.Context) (*schemas.Message, *types.AppError) {

	var payload schemas.FileOperation

	if err := c.ShouldBindJSON(&payload); err != nil {
		return nil, &types.AppError{Error: errors.New("invalid request payload"), Code: http.StatusBadRequest}
	}

	var destination models.File

	if err := fs.Db.Model(&models.File{}).Select("id").Where("path = ?", payload.Destination).First(&destination).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &types.AppError{Error: errors.New("destination not found"), Code: http.StatusNotFound}

	}

	if err := fs.Db.Model(&models.File{}).Where("id IN ?", payload.Files).UpdateColumn("parent_id", destination.ID).Error; err != nil {
		return nil, &types.AppError{Error: errors.New("move failed"), Code: http.StatusInternalServerError}
	}

	return &schemas.Message{Status: true, Message: "files moved"}, nil
}

func (fs *FileService) DeleteFiles(c *gin.Context) (*schemas.Message, *types.AppError) {

	var payload schemas.FileOperation

	if err := c.ShouldBindJSON(&payload); err != nil {
		return nil, &types.AppError{Error: errors.New("invalid request payload"), Code: http.StatusBadRequest}
	}

	if err := fs.Db.Exec("call drive.delete_files($1)", payload.Files).Error; err != nil {
		return nil, &types.AppError{Error: errors.New("failed to delete files"), Code: http.StatusInternalServerError}
	}

	return &schemas.Message{Status: true, Message: "files deleted"}, nil
}

func (fs *FileService) MoveDirectory(c *gin.Context) (*schemas.Message, *types.AppError) {

	var payload schemas.DirMove

	if err := c.ShouldBindJSON(&payload); err != nil {
		return nil, &types.AppError{Error: errors.New("invalid request payload"), Code: http.StatusBadRequest}
	}

	userId := getUserId(c)

	if err := fs.Db.Exec("select * from drive.move_directory(? , ? , ?)", payload.Source, payload.Destination, userId).Error; err != nil {
		return nil, &types.AppError{Error: errors.New("failed to move directory"), Code: http.StatusInternalServerError}
	}

	return &schemas.Message{Status: true, Message: "directory moved"}, nil
}

func (fs *FileService) GetFileStream(c *gin.Context) {

	w := c.Writer
	r := c.Request

	fileID := c.Param("fileID")

	file := &schemas.FileOutFull{}

	key := fmt.Sprintf("files:%s", fileID)

	err := cache.GetCache().Get(key, file)

	if err != nil {
		file, err = fs.GetFileByID(c)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cache.GetCache().Set(key, file, 0)
	}

	c.Header("Accept-Ranges", "bytes")

	var start, end int64

	rangeHeader := r.Header.Get("Range")

	if rangeHeader == "" {
		start = 0
		end = file.Size - 1
		w.WriteHeader(http.StatusOK)
	} else {
		ranges, err := range_parser.Parse(file.Size, r.Header.Get("Range"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		start = ranges[0].Start
		end = ranges[0].End
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, file.Size))
		w.WriteHeader(http.StatusPartialContent)
	}

	contentLength := end - start + 1

	mimeType := file.MimeType

	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	c.Header("Content-Type", mimeType)

	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("E-Tag", fmt.Sprintf("\"%s\"", md5.FromString(file.ID+strconv.FormatInt(file.Size, 10))))
	c.Header("Last-Modified", file.UpdatedAt.UTC().Format(http.TimeFormat))

	disposition := "inline"

	if c.Query("d") == "1" {
		disposition = "attachment"
	}

	c.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, file.Name))

	if r.Method != "HEAD" {
		parts := rangedParts(*file.Parts, start, end)
		lr, _ := reader.NewLinearReader(parts, contentLength)
		io.CopyN(w, lr, contentLength)
	}

}

func setOrderFilter(query *gorm.DB, pagingParams *schemas.PaginationQuery, sortingParams *schemas.SortingQuery) *gorm.DB {
	if pagingParams.NextPageToken != "" {
		sortColumn := sortingParams.Sort
		if sortColumn == "name" {
			sortColumn = "name collate numeric"
		} else {
			sortColumn = utils.CamelToSnake(sortingParams.Sort)
		}

		tokenValue, err := base64.StdEncoding.DecodeString(pagingParams.NextPageToken)
		if err == nil {
			if sortingParams.Order == "asc" {
				return query.Where(fmt.Sprintf("%s > ?", sortColumn), string(tokenValue))
			} else {
				return query.Where(fmt.Sprintf("%s < ?", sortColumn), string(tokenValue))
			}
		}
	}
	return query
}

func getOrder(sortingParams schemas.SortingQuery) string {
	sortColumn := utils.CamelToSnake(sortingParams.Sort)
	if sortingParams.Sort == "name" {
		sortColumn = "name collate numeric"
	}

	return fmt.Sprintf("%s %s", sortColumn, strings.ToUpper(sortingParams.Order))
}
