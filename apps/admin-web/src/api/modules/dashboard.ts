import { request } from "@/api/http";

export type DashboardRange = "24h" | "7d" | "30d" | "90d";
export type SyncJobStatus = "previewed" | "succeeded" | "failed";
export type ChannelIssueType = "hidden" | "incompatible";
export type ConfigIssueSource =
  | "account_auth"
  | "channel_login"
  | "channel_iap"
  | "package_iap_override"
  | "plugin_config"
  | "package_plugin_override";

export interface MetricLink {
  route: string;
  query: Record<string, string | number | boolean | undefined>;
}

export interface TimeRangeWindow {
  range: DashboardRange;
  since: string;
  until: string;
}

export interface FxReviewTopItem {
  runId: number;
  templateId: string;
  templateName: string;
  triggeredAt: string;
}

export interface ConfigIssueTopItem {
  source: ConfigIssueSource;
  gameId: string;
  gameName: string;
  target: string;
  lastCheckMessage: string;
}

export interface SyncJobTopItem {
  jobId: number;
  gameId: string;
  gameName: string;
  status: SyncJobStatus;
  executedAt: string;
}

export interface PendingSnapshotTopItem {
  snapshotId: number;
  gameId: string;
  gameName: string;
  configVersion: string;
  generatedAt: string;
}

export interface ChannelIssueTopItem {
  gameChannelId: number;
  gameId: string;
  gameName: string;
  channelId: string;
  marketCode: string;
  issue: ChannelIssueType;
}

export interface FxReviewMetric {
  permitted: boolean;
  envScoped: boolean;
  pendingReviewCount: number;
  link: MetricLink;
  topItems: FxReviewTopItem[];
}

export interface ConfigIssuesMetric {
  permitted: boolean;
  envScoped: boolean;
  invalidTotal: number;
  bySource: Array<{ source: ConfigIssueSource; invalidCount: number }>;
  link: MetricLink;
  topItems: ConfigIssueTopItem[];
}

export interface RecentSyncJobsMetric {
  permitted: boolean;
  envScoped: boolean;
  window: TimeRangeWindow;
  total: number;
  byStatus: Record<SyncJobStatus, number>;
  lastFailedAt: string | null;
  link: MetricLink;
  topItems: SyncJobTopItem[];
}

export interface PendingSnapshotsMetric {
  permitted: boolean;
  envScoped: boolean;
  draftCount: number;
  link: MetricLink;
  topItems: PendingSnapshotTopItem[];
}

export interface ChannelInstanceIssuesMetric {
  permitted: boolean;
  envScoped: boolean;
  hiddenCount: number;
  incompatibleCount: number;
  link: MetricLink;
  topItems: ChannelIssueTopItem[];
}

export interface DashboardSummary {
  environment: string;
  generatedAt: string;
  timeRange: TimeRangeWindow;
  fxReview: FxReviewMetric;
  configIssues: ConfigIssuesMetric;
  recentSyncJobs: RecentSyncJobsMetric;
  pendingSnapshots: PendingSnapshotsMetric;
  channelInstanceIssues: ChannelInstanceIssuesMetric;
}

export interface DashboardSummaryParams {
  range?: DashboardRange;
  withTopItems?: boolean;
  topN?: number;
}

function buildQuery(params: DashboardSummaryParams = {}): string {
  const search = new URLSearchParams();
  if (params.range) {
    search.set("range", params.range);
  }
  if (typeof params.withTopItems === "boolean") {
    search.set("withTopItems", String(params.withTopItems));
  }
  if (typeof params.topN === "number") {
    search.set("topN", String(params.topN));
  }
  const query = search.toString();
  return query ? `?${query}` : "";
}

export function getSummary(params: DashboardSummaryParams = {}): Promise<DashboardSummary> {
  return request<DashboardSummary>(`/api/admin/dashboard/summary${buildQuery(params)}`);
}
