package handlers

import (
	"github.com/gin-gonic/gin"
)

func ExecuteUpdate(c *gin.Context) {
	c.JSON(200, gin.H{"message": "Handling the UPDATE"})
}