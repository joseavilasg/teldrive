package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/divyam234/drive/models"
	"github.com/divyam234/drive/types"
	"github.com/divyam234/drive/utils"
	"github.com/divyam234/drive/utils/auth"
	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v3/jwt"
)

func getUserId(c *gin.Context) int {
	val, _ := c.Get("jwtUser")
	jwtUser := val.(*jwt.Claims)
	userId, _ := strconv.Atoi(strings.Split(jwtUser.Subject, ":")[1])
	return userId
}

func rangedParts(parts models.Parts, startByte, endByte int64) []types.Part {

	chunkSize := parts[0].Size

	numParts := int64(len(parts))

	validParts := []types.Part{}

	firstChunk := max(startByte/chunkSize, 0)

	lastChunk := min(endByte/chunkSize, numParts)

	startInFirstChunk := startByte % chunkSize

	endInLastChunk := endByte % chunkSize

	if firstChunk == lastChunk {
		validParts = append(validParts, types.Part{
			Url:   parts[firstChunk].Url,
			Start: startInFirstChunk,
			End:   endInLastChunk,
		})
	} else {
		validParts = append(validParts, types.Part{
			Url:   parts[firstChunk].Url,
			Start: startInFirstChunk,
			End:   parts[firstChunk].Size - 1,
		})

		// Add valid parts from any chunks in between.
		for i := firstChunk + 1; i < lastChunk; i++ {
			validParts = append(validParts, types.Part{
				Url:   parts[i].Url,
				Start: 0,
				End:   parts[i].Size - 1,
			})
		}

		// Add valid parts from the last chunk.
		validParts = append(validParts, types.Part{
			Url:   parts[lastChunk].Url,
			Start: 0,
			End:   endInLastChunk,
		})
	}

	return validParts
}

func generateToken(user *models.User, maxAge int) (string, error) {

	now := time.Now().UTC()

	jwtClaims := &jwt.Claims{
		Issuer:   "drive",
		Subject:  fmt.Sprintf("%s:%d", user.UserName, user.Id),
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Duration(maxAge) * time.Second)),
	}

	jweToken, err := auth.Encode(jwtClaims)
	if err != nil {
		return "", err
	}
	return jweToken, nil
}

func checkUserIsAllowed(email string) bool {
	config := utils.GetConfig()
	found := false
	if len(config.AllowedUsers) > 0 {
		for _, user := range config.AllowedUsers {
			if user == email {
				found = true
				break
			}
		}
	} else {
		found = true
	}
	return found
}
