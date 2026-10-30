package main

import (
	"database/sql"
	"fmt"
	"gnnwatchtower/internal/version"
	"net/url"
)

// Queue only recently reporting, existing agents; do not provision devices or change credentials.
func prepareAgentMigration(path string) error {
	db, e := sql.Open("sqlite", "file:"+path+"?mode=rw&_pragma="+url.QueryEscape("busy_timeout(10000)"))
	if e != nil {
		return e
	}
	defer db.Close()
	result, e := db.Exec(`UPDATE devices SET agent_upgrade_status='pending',agent_upgrade_target=? WHERE is_agent_installed=1 AND monitoring_mode='server' AND monitoring_profile='active' AND COALESCE(agent_version,'')!=? AND last_seen_at>=datetime('now','-90 seconds')`, version.Get(), version.Get())
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	fmt.Printf("%d\n", n)
	return nil
}
