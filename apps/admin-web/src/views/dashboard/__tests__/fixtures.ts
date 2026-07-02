import type { DashboardSummary } from "@/api/modules/dashboard";

export const DASHBOARD_SAMPLE_SUMMARY: DashboardSummary = {
  environment: "production",
  generatedAt: "2026-06-17T13:00:00Z",
  timeRange: {
    range: "7d",
    since: "2026-06-10T13:00:00Z",
    until: "2026-06-17T13:00:00Z"
  },
  fxReview: {
    permitted: true,
    envScoped: false,
    pendingReviewCount: 2,
    link: { route: "/cashier", query: { tab: "fx-review", status: "pending_review" } },
    topItems: [
      {
        runId: 1201,
        templateId: "global_cashier_v3",
        templateName: "Global Cashier Price v3",
        triggeredAt: "2026-06-17T02:00:00Z"
      }
    ]
  },
  configIssues: {
    permitted: true,
    envScoped: true,
    invalidTotal: 6,
    bySource: [
      { source: "account_auth", invalidCount: 1 },
      { source: "channel_login", invalidCount: 2 },
      { source: "channel_iap", invalidCount: 1 },
      { source: "package_iap_override", invalidCount: 1 },
      { source: "plugin_config", invalidCount: 1 },
      { source: "package_plugin_override", invalidCount: 0 }
    ],
    link: { route: "/games", query: { configStatus: "invalid" } },
    topItems: [
      {
        source: "channel_login",
        gameId: "g_swordman",
        gameName: "剑客世界",
        target: "google",
        lastCheckMessage: "缺少必填敏感字段或文件字段"
      }
    ]
  },
  recentSyncJobs: {
    permitted: true,
    envScoped: true,
    window: {
      range: "7d",
      since: "2026-06-10T13:00:00Z",
      until: "2026-06-17T13:00:00Z"
    },
    total: 4,
    byStatus: { previewed: 1, succeeded: 2, failed: 1 },
    lastFailedAt: "2026-06-15T09:30:00Z",
    link: { route: "/games", query: { tab: "sync-history", targetEnv: "production" } },
    topItems: [
      {
        jobId: 88,
        gameId: "g_swordman",
        gameName: "剑客世界",
        status: "failed",
        executedAt: "2026-06-15T09:30:00Z"
      }
    ]
  },
  pendingSnapshots: {
    permitted: true,
    envScoped: true,
    draftCount: 3,
    link: { route: "/games", query: { tab: "snapshots", status: "draft" } },
    topItems: [
      {
        snapshotId: 510,
        gameId: "g_swordman",
        gameName: "剑客世界",
        configVersion: "20260617080000-a1b2c3d4",
        generatedAt: "2026-06-17T08:00:00Z"
      }
    ]
  },
  channelInstanceIssues: {
    permitted: true,
    envScoped: true,
    hiddenCount: 2,
    incompatibleCount: 1,
    link: { route: "/games", query: { tab: "channels", issue: "hidden,incompatible" } },
    topItems: [
      {
        gameChannelId: 3301,
        gameId: "g_swordman",
        gameName: "剑客世界",
        channelId: "google",
        marketCode: "CN",
        issue: "incompatible"
      }
    ]
  }
};

export function cloneSummary(summary: DashboardSummary = DASHBOARD_SAMPLE_SUMMARY): DashboardSummary {
  return JSON.parse(JSON.stringify(summary)) as DashboardSummary;
}

export const DASHBOARD_EMPTY_SUMMARY: DashboardSummary = {
  ...cloneSummary(),
  fxReview: { ...cloneSummary().fxReview, pendingReviewCount: 0, topItems: [] },
  configIssues: {
    ...cloneSummary().configIssues,
    invalidTotal: 0,
    bySource: cloneSummary().configIssues.bySource.map((item) => ({ ...item, invalidCount: 0 })),
    topItems: []
  },
  recentSyncJobs: {
    ...cloneSummary().recentSyncJobs,
    total: 0,
    byStatus: { previewed: 0, succeeded: 0, failed: 0 },
    lastFailedAt: null,
    topItems: []
  },
  pendingSnapshots: { ...cloneSummary().pendingSnapshots, draftCount: 0, topItems: [] },
  channelInstanceIssues: {
    ...cloneSummary().channelInstanceIssues,
    hiddenCount: 0,
    incompatibleCount: 0,
    topItems: []
  }
};
