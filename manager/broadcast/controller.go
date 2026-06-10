package broadcast

import (
	"chat/auth"
	"chat/utils"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func ViewBroadcastAPI(c *gin.Context) {
	c.JSON(http.StatusOK, getLatestBroadcast(c))
}

func CreateBroadcastAPI(c *gin.Context) {
	user := auth.RequireAdmin(c)
	if user == nil {
		return
	}

	var form createRequest
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusOK, createResponse{
			Status: false,
			Error:  err.Error(),
		})
		return
	}

	if strings.TrimSpace(form.Content) == "" {
		c.JSON(http.StatusOK, createResponse{
			Status: false,
			Error:  "content is empty",
		})
		return
	}

	err := createBroadcast(c, user, form.Content)
	if err != nil {
		c.JSON(http.StatusOK, createResponse{
			Status: false,
			Error:  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, createResponse{
		Status: true,
	})
}

func GetBroadcastListAPI(c *gin.Context) {
	user := auth.RequireAdmin(c)
	if user == nil {
		return
	}

	data, err := getBroadcastList(c)
	if err != nil {
		c.JSON(http.StatusOK, listResponse{
			Data: []Info{},
		})
		return
	}

	c.JSON(http.StatusOK, listResponse{
		Data: data,
	})
}

func RemoveBroadcastAPI(c *gin.Context) {
	user := auth.RequireAdmin(c)
	if user == nil {
		return
	}

	index := utils.ParseInt(c.Param("index"))
	if index <= 0 {
		c.JSON(http.StatusOK, createResponse{
			Status: false,
			Error:  "invalid broadcast id",
		})
		return
	}

	err := removeBroadcast(c, index)
	c.JSON(http.StatusOK, createResponse{
		Status: err == nil,
		Error:  utils.GetError(err),
	})
}

func UpdateBroadcastAPI(c *gin.Context) {
	user := auth.RequireAdmin(c)
	if user == nil {
		return
	}

	var form updateRequest
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusOK, createResponse{
			Status: false,
			Error:  err.Error(),
		})
		return
	}

	if form.Index <= 0 {
		c.JSON(http.StatusOK, createResponse{
			Status: false,
			Error:  "invalid broadcast id",
		})
		return
	}

	if strings.TrimSpace(form.Content) == "" {
		c.JSON(http.StatusOK, createResponse{
			Status: false,
			Error:  "content is empty",
		})
		return
	}

	err := updateBroadcast(c, form.Index, form.Content)
	c.JSON(http.StatusOK, createResponse{
		Status: err == nil,
		Error:  utils.GetError(err),
	})
}
