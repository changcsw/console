import { request } from "@/api/http";

export interface GameListItem {
  gameId: string;
  name: string;
  alias: string;
  defaultMarketCode: string;
  status: string;
}

export async function fetchGames(): Promise<{ items: GameListItem[] }> {
  return request<{ items: GameListItem[] }>("/api/admin/games");
}

