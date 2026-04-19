package jobs

import "log"

// RunInboundPollerJob is a stub for the inbound tracking poller (PRD 2.2).
//
// INC-003 (backlog) status: this poller has never been implemented. The
// inbound-tracking flow currently operates entirely on customer pre-alerts
// and warehouse staff manual receive operations; there is no upstream
// carrier API integration. The function is kept as a placeholder so that
// when that integration lands, the wiring point in cmd/server is obvious.
//
// Until then, calling this function is a documented no-op. It is not
// scheduled by runDailyJobs, so customers see no behavior either way.
func RunInboundPollerJob() {
	log.Print("inbound poller not implemented (see INC-003 backlog note)")
}
