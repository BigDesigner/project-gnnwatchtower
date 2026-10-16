package api

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/models"
	"math"
	"time"
)

func requireServerMode(c *gin.Context, id int) bool {
	var mode, kind string
	if database.DB == nil || database.DB.QueryRow(`SELECT monitoring_mode,device_type FROM devices WHERE id=?`, id).Scan(&mode, &kind) != nil {
		c.JSON(404, gin.H{"error": "Cihaz bulunamadı"})
		return false
	}
	if mode != models.ModeServer || !models.ServerType(kind) {
		c.JSON(403, gin.H{"error": "Bu izleme kategorisinde ajan, servis veya donanım telemetrisi kullanılamaz"})
		return false
	}
	return true
}

func loadDevices() ([]models.Device, error) {
	if database.DB == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := database.DB.Query(`SELECT d.id,d.name,d.ip_address,COALESCE(d.mac_address,''),d.device_type,COALESCE(d.os_version,''),COALESCE(d.purpose,''),COALESCE(d.location,''),COALESCE(d.operator_notes,''),d.is_agent_installed,COALESCE(d.agent_token,''),d.monitoring_mode,d.monitoring_profile,d.check_interval_seconds,d.status,d.last_seen_at,d.last_status_change_at,d.response_time_ms,d.created_at,d.updated_at,COALESCE(d.agent_version,''),COALESCE(d.agent_upgrade_status,'none'),COALESCE(d.agent_upgrade_target,''),COALESCE(d.agent_upgrade_error,''),d.agent_last_upgrade_at,COALESCE(d.machine_uuid,''),d.last_checked_at,d.last_probe_latency_ms,d.probe_error,d.http_result,m.cpu_usage_pct,m.ram_usage_pct,m.disk_usage_pct,m.temperature_celsius,m.net_rx_kbps,m.net_tx_kbps,m.recorded_at
 FROM devices d LEFT JOIN system_metrics m ON m.id=(SELECT MAX(id) FROM system_metrics WHERE device_id=d.id) ORDER BY d.id`)
	if err != nil {
		return nil, err
	}
	devices := []models.Device{}
	now := time.Now()
	for rows.Next() {
		var d models.Device
		var seen, upgrade, checked, metric sql.NullTime
		var rtt, cpu, ram, disk, temp, rx, tx sql.NullFloat64
		if err = rows.Scan(&d.ID, &d.Name, &d.IPAddress, &d.MACAddress, &d.DeviceType, &d.OSVersion, &d.Purpose, &d.Location, &d.OperatorNotes, &d.IsAgentInstalled, &d.AgentToken, &d.MonitoringMode, &d.MonitoringProfile, &d.CheckIntervalSeconds, &d.Status, &seen, &d.LastStatusChangeAt, &d.ResponseTimeMS, &d.CreatedAt, &d.UpdatedAt, &d.AgentVersion, &d.AgentUpgradeStatus, &d.AgentUpgradeTarget, &d.AgentUpgradeError, &upgrade, &d.MachineUUID, &checked, &rtt, &d.ProbeError, &d.HTTPResult, &cpu, &ram, &disk, &temp, &rx, &tx, &metric); err != nil {
			rows.Close()
			return nil, err
		}
		if seen.Valid {
			d.LastSeenAt = &seen.Time
		}
		if upgrade.Valid {
			d.AgentLastUpgradeAt = &upgrade.Time
		}
		if checked.Valid {
			d.LastCheckedAt = &checked.Time
		}
		if metric.Valid {
			d.LatestMetricAt = &metric.Time
		}
		d.ObservationState = models.ObservationState(d, now)
		d.TelemetryFresh = models.TelemetryFresh(d, now)
		if rtt.Valid && rtt.Float64 >= 0 && d.ObservationState == "online" {
			v := rtt.Float64
			d.LastProbeLatencyMS = &v
		}
		if d.TelemetryFresh {
			if cpu.Valid {
				v := cpu.Float64
				d.LatestCPU = &v
			}
			if ram.Valid {
				v := ram.Float64
				d.LatestRAM = &v
			}
			if disk.Valid {
				v := disk.Float64
				d.LatestDisk = &v
			}
			if temp.Valid && temp.Float64 > 0 {
				v := temp.Float64
				d.LatestTemp = &v
			}
			if rx.Valid {
				v := rx.Float64
				d.LatestNetRx = &v
			}
			if tx.Valid {
				v := tx.Float64
				d.LatestNetTx = &v
			}
		}
		if d.MonitoringMode != models.ModeServer {
			d.AgentToken = ""
			d.LatestMetricAt = nil
			d.IsAgentInstalled = false
			d.AgentVersion = ""
			d.AgentUpgradeStatus = ""
			d.AgentUpgradeTarget = ""
			d.AgentUpgradeError = ""
			d.MachineUUID = ""
		}
		devices = append(devices, d)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range devices {
		if devices[i].MonitoringMode == models.ModeServer {
			devices[i].Services = getServices(devices[i].ID)
		}
	}
	return devices, nil
}

func telemetryNumber(v sql.NullFloat64) any {
	if !v.Valid || math.IsNaN(v.Float64) || math.IsInf(v.Float64, 0) {
		return nil
	}
	return math.Round(v.Float64*10) / 10
}
