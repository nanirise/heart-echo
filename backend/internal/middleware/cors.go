package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS 放行浏览器的跨域请求。
//
// 生产环境走 Nginx 同源代理，用不到它；本地 Vue dev server 与后端不同端口，
// 没有它浏览器会直接把请求拦掉。
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return cors.New(cors.Config{
		// AllowCredentials 为 true 时 AllowOrigins 不能写 "*"：浏览器会直接拒绝，
		// 且等于对任意站点开放携带 Cookie 的请求。origins 必须是具体域名。
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
