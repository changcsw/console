import { request } from "@/api/http";

export type ChannelConfigStatus = "empty" | "invalid" | "valid";
export type ChannelRegion = "domestic" | "overseas";

export interface GameMarketChannelListItem {
  id: string;
  gameId: string;
  market: string;
  channelId: string;
  configStatus: ChannelConfigStatus;
  hidden: boolean;
  includedInSnapshot: boolean;
  includedInSync: boolean;
  includedInRuntimeConfig: boolean;
  incompatibleWithMarket: boolean;
  normalConfig?: Record<string, unknown>;
  secretConfig?: Record<string, string>;
  fileConfig?: Record<string, string>;
}

export interface SourceMarketChannelInstance {
  market: string;
  channelId: string;
  normalConfig: Record<string, unknown>;
  secretConfig?: Record<string, string>;
  fileConfig?: Record<string, string>;
}

export interface CreateGameMarketChannelPayload {
  channelId: string;
  region: ChannelRegion;
  copyFromMarket?: string;
  normalConfig?: Record<string, unknown>;
  secretConfig?: Record<string, string>;
  fileConfig?: Record<string, string>;
}

export async function fetchGameMarketChannels(gameId: string, market = ""): Promise<GameMarketChannelListItem[]> {
  const query = market ? `?market=${encodeURIComponent(market)}` : "";
  return request<GameMarketChannelListItem[]>(`/api/admin/games/${gameId}/market-channels${query}`);
}

export async function createGameMarketChannel(
  gameId: string,
  market: string,
  payload: CreateGameMarketChannelPayload
): Promise<GameMarketChannelListItem> {
  return request<GameMarketChannelListItem>(`/api/admin/games/${gameId}/markets/${market}/channels`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function hideGameMarketChannel(id: string): Promise<GameMarketChannelListItem> {
  return request<GameMarketChannelListItem>(`/api/admin/game-market-channels/${id}/hide`, {
    method: "POST"
  });
}

export async function unhideGameMarketChannel(id: string): Promise<GameMarketChannelListItem> {
  return request<GameMarketChannelListItem>(`/api/admin/game-market-channels/${id}/unhide`, {
    method: "POST"
  });
}
