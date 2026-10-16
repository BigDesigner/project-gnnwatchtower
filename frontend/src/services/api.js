import axios from 'axios'

const client = axios.create({
  baseURL: '/api',
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// Request interceptor to attach JWT token
client.interceptors.request.use(config => {
  const token = localStorage.getItem('gnn_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Response interceptor for handling 401 Unauthorized and 403 Forced Password Change
client.interceptors.response.use(
  response => response,
  error => {
    if (error.response && error.response.status === 401) {
      // Clear token if invalid or expired
      localStorage.removeItem('gnn_token')
      window.dispatchEvent(new CustomEvent('gnn-auth-failed'))
    } else if (error.response && error.response.status === 403 && error.response.data?.must_change_password) {
      window.dispatchEvent(new CustomEvent('gnn-must-change-password'))
    }
    return Promise.reject(error)
  }
)

export default {
  // Auth
  async login(username, password) {
    const res = await client.post('/auth/login', { username, password })
    if (res.data && res.data.token) {
      localStorage.setItem('gnn_token', res.data.token)
    }
    return res.data
  },
  async getMe() {
    const res = await client.get('/auth/me')
    return res.data
  },
  async changePassword(currentPassword, newPassword) {
    const res = await client.post('/auth/change-password', {
      current_password: currentPassword,
      new_password: newPassword
    })
    return res.data
  },
  async logout() {
    try {
      await client.post('/auth/logout')
    } catch {
      // Ignore network errors or already invalidated token
    } finally {
      localStorage.removeItem('gnn_token')
      window.dispatchEvent(new CustomEvent('gnn-auth-failed'))
    }
  },

  // Dashboard & System Telemetry
  async getDashboardStats() {
    const res = await client.get('/dashboard/stats')
    return res.data
  },
  async getGatewayHealth() {
    const res = await client.get('/system/gateway-health')
    return res.data
  },

  // Devices & Services
  async getDevices() {
    const res = await client.get('/devices')
    return res.data
  },
  async createDevice(data) {
    const res = await client.post('/devices', data)
    return res.data
  },
  async updateDevice(id, data) {
    const res = await client.put(`/devices/${id}`, data)
    return res.data
  },
  async deleteDevice(id) {
    const res = await client.delete(`/devices/${id}`)
    return res.data
  },
  async getDeviceTelemetry(id) {
    const res = await client.get(`/devices/${id}/telemetry`)
    return res.data
  },
  async addService(deviceId, data) {
    const res = await client.post(`/devices/${deviceId}/services`, data)
    return res.data
  },
  async deleteService(deviceId, serviceId) {
    const res = await client.delete(`/devices/${deviceId}/services/${serviceId}`)
    return res.data
  },

  // Downtime Incidents
  async getIncidents() {
    const res = await client.get('/incidents')
    return res.data
  },
  async classifyIncident(id, data) {
    const res = await client.put(`/incidents/${id}/classify`, data)
    return res.data
  },

  // Settings
  async getSettings() {
    const res = await client.get('/settings')
    return res.data
  },
  async updateSettings(data) {
    const res = await client.put('/settings', data)
    return res.data
  },
  async testAlert(data) {
    const res = await client.post('/settings/test-alert', data)
    return res.data
  },

  // Backup & Restore
  async exportBackup() {
    const res = await client.get('/backup/export')
    return res.data
  },
  async importBackup(bundle) {
    const res = await client.post('/backup/import', bundle)
    return res.data
  },

  // Agent Install Scripts
  async getLinuxAgentScript() {
    const res = await client.get('/agent/install/linux')
    return res.data
  },
  async getWindowsAgentScript() {
    const res = await client.get('/agent/install/windows')
    return res.data
  },

  // Agent Remote Upgrade Orchestration
  async upgradeAgent(deviceId) {
    const res = await client.post(`/agent/${deviceId}/upgrade`)
    return res.data
  },
  async upgradeAllAgents() {
    const res = await client.post('/agent/upgrade-all')
    return res.data
  },
  async getAgentVersions() {
    const res = await client.get('/agent/versions')
    return res.data
  }
}
