import { request } from "@/api/http";

export type ChannelConfigStatus = "empty" | "invalid" | "valid";

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
}

export async function fetchGameMarketChannels(gameId: string, market = ""): Promise<GameMarketChannelListItem[]> {
  const query = market ? `?market=${encodeURIComponent(market)}` : "";
  return request<GameMarketChannelListItem[]>(`/api/admin/games/${gameId}/market-channels${query}`);
}
