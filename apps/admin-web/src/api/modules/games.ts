import { request } from "@/api/http";

// 枚举（与 00-common §3 全局事实源一致；本模块本地引用，待 dictionary store 落地后可切换）
export type GameStatus = "draft" | "active" | "disabled";
export type Market = "GLOBAL" | "JP" | "KR" | "SEA" | "HMT" | "CN";
export type LegalScopeType = "default" | "market" | "locale";

export interface Paginated<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

export interface GameListItem {
  gameId: string;
  name: string;
  alias: string;
  iconUrl: string;
  status: GameStatus;
  defaultMarketCode: string;
  marketCodes: string[];
  marketCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface GameMarket {
  marketCode: Market;
  isDefault: boolean;
  enabled: boolean;
  defaultLocale: string;
}

export interface GameLegalLink {
  scopeType: LegalScopeType;
  scopeValue: string;
  termsUrl: string;
  privacyUrl: string;
  deleteAccountUrl: string;
}

export interface GameDetail {
  gameId: string;
  name: string;
  alias: string;
  iconUrl: string;
  status: GameStatus;
  defaultMarketCode: string;
  /** 详情接口恒为脱敏值 "masked"；仅创建接口一次性返明文 */
  gameSecret: string;
  secretMasked: boolean;
  environment?: string;
  markets: GameMarket[];
  legalLinks: GameLegalLink[];
  createdAt: string;
  updatedAt: string;
}

export interface ListGamesQuery {
  page?: number;
  pageSize?: number;
  sort?: string;
  keyword?: string;
  status?: GameStatus;
  marketCode?: Market;
}

export interface CreateGameRequest {
  name: string;
  alias: string;
  iconUrl?: string;
  defaultMarketCode?: Market;
  status?: GameStatus;
  markets?: Market[];
}

export interface UpdateGameRequest {
  name?: string;
  alias?: string;
  iconUrl?: string;
  status?: GameStatus;
  defaultMarketCode?: Market;
}

export interface ReplaceMarketsItem {
  marketCode: Market;
  isDefault?: boolean;
  enabled?: boolean;
  defaultLocale?: string;
}

export interface ReplaceMarketsRequest {
  markets: ReplaceMarketsItem[];
}

export interface ReplaceLegalLinksItem {
  scopeType: LegalScopeType;
  scopeValue?: string;
  termsUrl?: string;
  privacyUrl?: string;
  deleteAccountUrl?: string;
}

export interface ReplaceLegalLinksRequest {
  legalLinks: ReplaceLegalLinksItem[];
}

function buildQuery(params: Record<string, unknown>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      search.append(key, String(value));
    }
  }
  const query = search.toString();
  return query ? `?${query}` : "";
}

// GET /games — 列表（game.read）
export function listGames(query: ListGamesQuery = {}): Promise<Paginated<GameListItem>> {
  return request<Paginated<GameListItem>>(`/api/admin/games${buildQuery({ ...query })}`);
}

// POST /games — 创建（game.write）；201 一次性返明文 gameSecret（secretMasked:false）
export function createGame(payload: CreateGameRequest): Promise<GameDetail> {
  return request<GameDetail>("/api/admin/games", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

// GET /games/{gameId} — 详情（game.read）；gameSecret 恒 "masked"
export function getGame(gameId: string): Promise<GameDetail> {
  return request<GameDetail>(`/api/admin/games/${encodeURIComponent(gameId)}`);
}

// PATCH /games/{gameId} — 编辑基础信息（game.write）
export function updateGame(gameId: string, payload: UpdateGameRequest): Promise<GameDetail> {
  return request<GameDetail>(`/api/admin/games/${encodeURIComponent(gameId)}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

// PUT /games/{gameId}/markets — 全量覆盖市场（game.write）
export function replaceMarkets(gameId: string, payload: ReplaceMarketsRequest): Promise<GameDetail> {
  return request<GameDetail>(`/api/admin/games/${encodeURIComponent(gameId)}/markets`, {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

// PUT /games/{gameId}/legal-links — 全量覆盖法务链接（game.write）
// 后端返回完整 GameDetail（与 PUT /markets 一致）；调用方按需读取 res.legalLinks。
export function replaceLegalLinks(gameId: string, payload: ReplaceLegalLinksRequest): Promise<GameDetail> {
  return request<GameDetail>(`/api/admin/games/${encodeURIComponent(gameId)}/legal-links`, {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}
