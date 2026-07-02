package dto

import "time"

type DashboardSummaryParams struct {
	Range        string
	WithTopItems bool
	TopN         int
}

type DashboardSummary struct {
	Environment           string                      `json:"environment"`
	GeneratedAt           time.Time                   `json:"generatedAt"`
	TimeRange             DashboardTimeRange          `json:"timeRange"`
	FXReview              DashboardFXReviewMetric     `json:"fxReview"`
	ConfigIssues          DashboardConfigIssuesMetric `json:"configIssues"`
	RecentSyncJobs        DashboardRecentSyncMetric   `json:"recentSyncJobs"`
	PendingSnapshots      DashboardPendingSnapMetric  `json:"pendingSnapshots"`
	ChannelInstanceIssues DashboardChannelIssueMetric `json:"channelInstanceIssues"`
}

type DashboardTimeRange struct {
	Range string    `json:"range"`
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
}

type DashboardMetricLink struct {
	Route string         `json:"route"`
	Query map[string]any `json:"query"`
}

type DashboardFXReviewMetric struct {
	Permitted          bool                    `json:"permitted"`
	EnvScoped          bool                    `json:"envScoped"`
	PendingReviewCount int64                   `json:"pendingReviewCount"`
	Link               DashboardMetricLink     `json:"link"`
	TopItems           []DashboardFXReviewItem `json:"topItems"`
}

type DashboardFXReviewItem struct {
	RunID        int64     `json:"runId"`
	TemplateID   string    `json:"templateId"`
	TemplateName string    `json:"templateName"`
	TriggeredAt  time.Time `json:"triggeredAt"`
}

type DashboardConfigIssuesMetric struct {
	Permitted    bool                        `json:"permitted"`
	EnvScoped    bool                        `json:"envScoped"`
	InvalidTotal int64                       `json:"invalidTotal"`
	BySource     []DashboardConfigIssueCount `json:"bySource"`
	Link         DashboardMetricLink         `json:"link"`
	TopItems     []DashboardConfigIssueItem  `json:"topItems"`
}

type DashboardConfigIssueCount struct {
	Source       string `json:"source"`
	InvalidCount int64  `json:"invalidCount"`
}

type DashboardConfigIssueItem struct {
	Source           string `json:"source"`
	GameID           string `json:"gameId"`
	GameName         string `json:"gameName"`
	Target           string `json:"target"`
	LastCheckMessage string `json:"lastCheckMessage"`
}

type DashboardRecentSyncMetric struct {
	Permitted    bool                   `json:"permitted"`
	EnvScoped    bool                   `json:"envScoped"`
	Window       DashboardTimeRange     `json:"window"`
	Total        int64                  `json:"total"`
	ByStatus     DashboardSyncJobStatus `json:"byStatus"`
	LastFailedAt *time.Time             `json:"lastFailedAt"`
	Link         DashboardMetricLink    `json:"link"`
	TopItems     []DashboardSyncJobItem `json:"topItems"`
}

type DashboardSyncJobStatus struct {
	Previewed int64 `json:"previewed"`
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
}

type DashboardSyncJobItem struct {
	JobID      int64     `json:"jobId"`
	GameID     string    `json:"gameId"`
	GameName   string    `json:"gameName"`
	Status     string    `json:"status"`
	ExecutedAt time.Time `json:"executedAt"`
}

type DashboardPendingSnapMetric struct {
	Permitted  bool                       `json:"permitted"`
	EnvScoped  bool                       `json:"envScoped"`
	DraftCount int64                      `json:"draftCount"`
	Link       DashboardMetricLink        `json:"link"`
	TopItems   []DashboardSnapshotTopItem `json:"topItems"`
}

type DashboardSnapshotTopItem struct {
	SnapshotID    int64     `json:"snapshotId"`
	GameID        string    `json:"gameId"`
	GameName      string    `json:"gameName"`
	ConfigVersion string    `json:"configVersion"`
	GeneratedAt   time.Time `json:"generatedAt"`
}

type DashboardChannelIssueMetric struct {
	Permitted         bool                           `json:"permitted"`
	EnvScoped         bool                           `json:"envScoped"`
	HiddenCount       int64                          `json:"hiddenCount"`
	IncompatibleCount int64                          `json:"incompatibleCount"`
	Link              DashboardMetricLink            `json:"link"`
	TopItems          []DashboardChannelIssueTopItem `json:"topItems"`
}

type DashboardChannelIssueTopItem struct {
	GameChannelID int64  `json:"gameChannelId"`
	GameID        string `json:"gameId"`
	GameName      string `json:"gameName"`
	ChannelID     string `json:"channelId"`
	MarketCode    string `json:"marketCode"`
	Issue         string `json:"issue"`
}
