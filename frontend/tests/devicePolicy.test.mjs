import test from 'node:test'
import assert from 'node:assert/strict'
import { sections,categoryView,observation,latency,metric } from '../src/devicePolicy.js'
const now = Date.now()
const base = { name:'Fixture',monitoring_mode:'icmp',monitoring_profile:'active',check_interval_seconds:15,status:'online',last_checked_at:new Date(now-1000).toISOString(),last_probe_latency_ms:1.25 }
test('three categories are disjoint and all visible menus are Turkish',()=>{
 assert.deepEqual(sections.map(s=>s.label),['Genel Bakış','Ajan Özeti','Web İzleme','IP Cihazları','Olay Günlüğü','Yedekleme Merkezi'])
 assert.deepEqual(['server','website','icmp'].map(mode=>categoryView({monitoring_mode:mode})),['devices','websites','ip-devices'])
})
test('unknown, stale and suspended observations do not become healthy',()=>{
 assert.equal(observation({...base,last_checked_at:null},now),'unknown')
 assert.equal(observation({...base,last_checked_at:new Date(now-120000).toISOString()},now),'stale')
 assert.equal(observation({...base,monitoring_profile:'maintenance'},now),'maintenance')
 assert.equal(observation({...base,monitoring_profile:'paused'},now),'paused')
 assert.equal(observation(base,now),'online')
 assert.equal(latency({...base,last_probe_latency_ms:null}),'—')
 assert.equal(latency({...base,status:'offline'}),'—')
})
test('missing or stale telemetry has no synthetic zeros',()=>{
 assert.equal(metric({...base,telemetry_fresh:false,latest_cpu:0},'latest_cpu'),'—')
 assert.equal(metric({...base,telemetry_fresh:true,latest_cpu:null},'latest_cpu'),'—')
 assert.equal(metric({...base,telemetry_fresh:true,latest_metric_at:new Date(now).toISOString(),latest_cpu:0},'latest_cpu'),'0 %')
})
