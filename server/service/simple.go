package service

import "github.com/gin-gonic/gin"

func ManagerIndex() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.HTML(200, "index.tmpl", nil)
	}
}
