package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"gnnwatchtower/internal/version"
)

// MaxBodySizeMiddleware limits request body size to mitigate DoS/OOM attacks
func MaxBodySizeMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// SetupRouter configures all REST routes, middleware, and web UI handlers
func SetupRouter(webFS fs.FS) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	// Fix R04: Do not trust arbitrary X-Forwarded-For headers from untrusted direct connections
	_ = r.SetTrustedProxies(nil)

	// Relaxed CORS for dev and embedded production
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Agent-Token", "X-Enrollment-Key"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	r.Use(cors.New(corsConfig))

	api := r.Group("/api")
	// Global 2MB payload ceiling for general API routes
	api.Use(MaxBodySizeMiddleware(2 * 1024 * 1024))
	{
		// Public Health, Auth & Agent Endpoints
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"service": "gnnwatchtower",
				"version": version.Get(),
				"port":    3800,
			})
		})
		api.POST("/auth/login", MaxBodySizeMiddleware(64*1024), Login)
		api.POST("/agent/enroll", MaxBodySizeMiddleware(256*1024), AgentEnroll)
		api.POST("/agent/report", MaxBodySizeMiddleware(1024*1024), AgentHeartbeat)
		api.GET("/agent/download/:platform", DownloadAgentBinary)
		api.GET("/agent/signer-key", GetAgentSignerKey)

		// Protected Routes (Authentication Required)
		protected := api.Group("")
		protected.Use(AuthMiddleware())
		{
			protected.GET("/auth/me", GetCurrentUser)
			protected.POST("/auth/logout", Logout)
			protected.POST("/auth/change-password", ChangePassword)
			protected.GET("/dashboard/stats", GetDashboardStats)

			// Fix R01: Protect installation scripts containing fleet enrollment keys
			protected.GET("/agent/install/linux", GenerateLinuxAgentScript)
			protected.GET("/agent/install/windows", GenerateWindowsAgentScript)

			// Agent Remote Upgrade Orchestration
			protected.POST("/agent/:id/upgrade", TriggerAgentUpgrade)
			protected.POST("/agent/upgrade-all", TriggerAllAgentsUpgrade)
			protected.GET("/agent/versions", GetAgentVersions)

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
			protected.POST("/backup/import", MaxBodySizeMiddleware(50*1024*1024), ImportBackup)

			// Settings
			protected.GET("/settings", GetSettings)
			protected.PUT("/settings", UpdateSettings)
			protected.POST("/settings/test-alert", TestAlert)
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
