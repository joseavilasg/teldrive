package services

import (
	"net/http"
	"strings"
	"time"

	"github.com/divyam234/drive/models"
	"github.com/divyam234/drive/schemas"
	"github.com/divyam234/drive/types"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/divyam234/drive/utils"
	"github.com/divyam234/drive/utils/auth"
	"github.com/gin-gonic/gin"
	"github.com/go-faster/errors"
	"github.com/go-jose/go-jose/v3/jwt"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	Db                *gorm.DB
	SessionMaxAge     int
	SessionCookieName string
}

func setCookie(c *gin.Context, key string, value string, age int) {

	config := utils.GetConfig()

	if config.CookieSameSite {
		c.SetSameSite(2)
	} else {
		c.SetSameSite(4)
	}
	c.SetCookie(key, value, age, "/", c.Request.Host, config.Https, true)

}

func (as *AuthService) SignUp(c *gin.Context) (*schemas.Message, *types.AppError) {

	var input schemas.SignInInput

	if err := c.ShouldBindJSON(&input); err != nil {
		return nil, &types.AppError{Error: err, Code: http.StatusBadRequest}
	}

	if !checkUserIsAllowed(input.Email) {
		return nil, &types.AppError{Error: errors.New("user not allowed"), Code: http.StatusBadRequest}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, &types.AppError{Error: err, Code: http.StatusBadRequest}
	}

	user := models.User{UserName: input.Username, Password: string(hashedPassword), Email: input.Email}

	if err := as.Db.Create(&user).Error; err != nil {
		pgErr := err.(*pgconn.PgError)
		if pgErr.Code == "23505" {
			return nil, &types.AppError{Error: errors.New("user exists"), Code: http.StatusBadRequest}
		}
		return nil, &types.AppError{Error: errors.New("failed to create user"), Code: http.StatusBadRequest}

	}

	jweToken, err := generateToken(&user, as.SessionMaxAge)

	if err != nil {
		return nil, &types.AppError{Error: err, Code: http.StatusBadRequest}
	}

	setCookie(c, as.SessionCookieName, jweToken, as.SessionMaxAge)

	file := &models.File{
		Name:     "root",
		Type:     "folder",
		MimeType: "drive/folder",
		Path:     "/",
		Depth:    utils.IntPointer(0),
		UserID:   user.Id,
		Status:   "active",
		ParentID: "root",
	}
	if err := as.Db.Create(file).Error; err != nil {
		return nil, &types.AppError{Error: errors.New("failed to create root folder"),
			Code: http.StatusInternalServerError}
	}

	return &schemas.Message{Status: true, Message: "signup success"}, nil
}

func (as *AuthService) LogIn(c *gin.Context) (*schemas.Message, *types.AppError) {

	var input schemas.LoginInput

	if err := c.ShouldBindJSON(&input); err != nil {
		return nil, &types.AppError{Error: err, Code: http.StatusBadRequest}
	}

	user := models.User{}

	if err := as.Db.Model(models.User{}).Where("user_name = ?", input.Username).First(&user).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &types.AppError{Error: errors.New("invalid username"), Code: http.StatusNotFound}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil && err == bcrypt.ErrMismatchedHashAndPassword {
		return nil, &types.AppError{Error: errors.New("invalid password"), Code: http.StatusBadRequest}
	}

	jweToken, err := generateToken(&user, as.SessionMaxAge)

	if err != nil {
		return nil, &types.AppError{Error: err, Code: http.StatusBadRequest}
	}

	setCookie(c, as.SessionCookieName, jweToken, as.SessionMaxAge)

	return &schemas.Message{Status: true, Message: "login success"}, nil
}

func (as *AuthService) GetSession(c *gin.Context) *schemas.Session {

	cookie, err := c.Request.Cookie(as.SessionCookieName)

	if err != nil {
		return nil
	}

	jwePayload, err := auth.Decode(cookie.Value)

	if err != nil {
		return nil
	}

	now := time.Now().UTC()

	newExpires := now.Add(time.Duration(as.SessionMaxAge) * time.Second)

	session := &schemas.Session{
		UserName: strings.Split(jwePayload.Subject, ":")[0],
		Expires:  newExpires.Format(time.RFC3339)}

	jwePayload.IssuedAt = jwt.NewNumericDate(now)

	jwePayload.Expiry = jwt.NewNumericDate(newExpires)

	jweToken, err := auth.Encode(jwePayload)

	if err != nil {
		return nil
	}
	setCookie(c, as.SessionCookieName, jweToken, as.SessionMaxAge)
	return session
}

func (as *AuthService) Logout(c *gin.Context) (*schemas.Message, *types.AppError) {
	setCookie(c, as.SessionCookieName, "", -1)
	return &schemas.Message{Status: true, Message: "logout success"}, nil
}
