export const sections = [
 { id: 'overview', label: 'Genel Bakış', icon: 'grid_view' },
 { id: 'devices', mode: 'server', label: 'Ajan Özeti', icon: 'dns' },
 { id: 'websites', mode: 'website', label: 'Web İzleme', icon: 'language' },
 { id: 'ip-devices', mode: 'icmp', label: 'IP Cihazları', icon: 'router' },
 { id: 'incidents', label: 'Olay Günlüğü', icon: 'history' },
 { id: 'backups', label: 'Yedekleme Merkezi', icon: 'backup' }
]
export const stateLabels = { online: 'Erişilebilir', offline: 'Ulaşılamıyor', unknown: 'Sonuç Doğrulanmadı', stale: 'Güncel Kontrol Yok', maintenance: 'Bakımda', paused: 'İzleme Duraklatıldı' }
export const typeLabels = { windows: 'Windows', linux: 'Linux', ubuntu: 'Ubuntu', debian: 'Debian', centos: 'CentOS', rhel: 'Red Hat', rocky: 'Rocky Linux', almalinux: 'AlmaLinux', nas: 'NAS', synology: 'Synology NAS', qnap: 'QNAP NAS', esxi: 'VMware ESXi', esx: 'VMware ESX', switch: 'Ağ Anahtarı', router: 'Yönlendirici', other: 'Diğer', website: 'Web Sitesi' }
export const categoryView = d => sections.find(s => s.mode === d.monitoring_mode)?.id || 'ip-devices'
export const categoryLabel = d => sections.find(s => s.mode === d?.monitoring_mode)?.label || 'Kategorisi Bilinmiyor'
export function observation(d, now = Date.now()) {
 if (['paused', 'maintenance'].includes(d.monitoring_profile)) return d.monitoring_profile
 if (!d.last_checked_at || !Number.isFinite(Date.parse(d.last_checked_at)) || Date.parse(d.last_checked_at) > now + 30000) return 'unknown'
 const cadence = d.monitoring_profile === 'dormant_24h' ? 86400 : d.monitoring_profile === 'dormant_30d' ? 2592000 : Math.max(d.monitoring_mode === 'icmp' ? 60 : 15, d.check_interval_seconds || 15)
 if (now - Date.parse(d.last_checked_at) > (2 * cadence + 30) * 1000) return 'stale'
 return ['online', 'offline'].includes(d.status) ? d.status : 'unknown'
}
export const dateTime = value => value ? new Date(value).toLocaleString('tr-TR') : '—'
export function latency(d) { return observation(d) === 'online' && typeof d.last_probe_latency_ms === 'number' ? d.last_probe_latency_ms.toLocaleString('tr-TR', { maximumFractionDigits: 2 }) + ' ms' : '—' }
export const telemetryCurrent = (d, now = Date.now()) => !!d.telemetry_fresh && !!d.latest_metric_at && now - Date.parse(d.latest_metric_at) >= -30000 && now - Date.parse(d.latest_metric_at) <= 90000 && observation(d, now) === 'online'
export const nasType = type => ['nas','synology','qnap','storage','truenas','freenas','netapp'].includes(type)
export const metric = (d, key) => telemetryCurrent(d) && typeof d[key] === 'number' ? d[key].toLocaleString('tr-TR', { maximumFractionDigits: 1 }) + ' %' : '—'
export const apiError = (err, fallback = 'İşlem tamamlanamadı. Bağlantıyı kontrol edip yeniden deneyin.') => err.response?.data?.error || fallback
