package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupRouter configures all REST routes, middleware, and web UI handlers
func SetupRouter(webFS fs.FS) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Relaxed CORS for dev and embedded production
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	r.Use(cors.New(corsConfig))

	api := r.Group("/api")
	{
		// Public Health, Auth & Agent Endpoints
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"service": "gnnwatchtower",
				"port":    3800,
			})
		})
		api.POST("/auth/login", Login)
		api.POST("/agent/report", AgentHeartbeat)
		api.GET("/agent/install/linux", GenerateLinuxAgentScript)
		api.GET("/agent/install/windows", GenerateWindowsAgentScript)
		api.GET("/agent/download/:platform", DownloadAgentBinary)

		// Protected Routes
		protected := api.Group("")
		protected.Use(AuthMiddleware())
		{
			protected.POST("/auth/change-password", ChangePassword)
			protected.GET("/dashboard/stats", GetDashboardStats)

			// Devices
			protected.GET("/devices", GetDevices)
			protected.POST("/devices", CreateDevice)
			protected.PUT("/devices/:id", UpdateDevice)
			protected.DELETE("/devices/:id", DeleteDevice)
			protected.GET("/devices/:id/telemetry", GetDeviceTelemetry)

			// Gateway & Telemetry
			protected.GET("/system/gateway-health", GetGatewayHealth)

			// Device Services
			protected.POST("/devices/:id/services", AddServiceToDevice)
			protected.DELETE("/devices/:id/services/:serviceId", DeleteService)

			// Subnet Discovery
			protected.POST("/discovery/scan", TriggerDiscovery)

			// Downtime Incidents
			protected.GET("/incidents", GetIncidents)
			protected.PUT("/incidents/:id/classify", ClassifyIncident)

			// Backup & Restore (JSON Bundle)
			protected.GET("/backup/export", ExportBackup)
			protected.POST("/backup/import", ImportBackup)

			// Settings
			protected.GET("/settings", GetSettings)
			protected.PUT("/settings", UpdateSettings)
		}
	}

	// Serve Frontend (Embedded Web UI or SPA fallback)
	if webFS != nil {
		fileServer := http.FileServer(http.FS(webFS))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"error": "API route not found"})
				return
			}

			// Check if file exists in webFS
			cleanPath := strings.TrimPrefix(path, "/")
			if cleanPath == "" {
				cleanPath = "index.html"
			}

			f, err := webFS.Open(cleanPath)
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}

			// SPA fallback: serve index.html
			indexFile, err := webFS.Open("index.html")
			if err == nil {
				_ = indexFile.Close()
				c.Request.URL.Path = "/"
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}

			c.String(http.StatusOK, "GNNwatchtower Core is running on port 3800. Web UI assets not embedded yet.")
		})
	}

	return r
}
