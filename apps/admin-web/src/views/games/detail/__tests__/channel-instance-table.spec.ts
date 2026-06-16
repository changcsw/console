import { render, screen } from "@testing-library/vue";
import type { GameMarketChannelListItem } from "@/api/gameMarketChannels";
import ChannelInstanceTable from "../components/ChannelInstanceTable.vue";

const items: GameMarketChannelListItem[] = [
  {
    id: "game-1:GLOBAL:google",
    gameId: "game-1",
    market: "GLOBAL",
    channelId: "google",
    configStatus: "valid",
    hidden: false,
    includedInSnapshot: true,
    includedInSync: true,
    includedInRuntimeConfig: true,
    incompatibleWithMarket: false
  },
  {
    id: "game-1:JP:google",
    gameId: "game-1",
    market: "JP",
    channelId: "google",
    configStatus: "invalid",
    hidden: false,
    includedInSnapshot: true,
    includedInSync: true,
    includedInRuntimeConfig: false,
    incompatibleWithMarket: false
  }
];

test("shows all market rows by default", () => {
  render(ChannelInstanceTable, {
    props: {
      selectedMarket: "",
      items
    }
  });

  expect(screen.getByText("GLOBAL")).toBeInTheDocument();
  expect(screen.getByText("JP")).toBeInTheDocument();
});

test("filters rows by selected market", () => {
  render(ChannelInstanceTable, {
    props: {
      selectedMarket: "JP",
      items
    }
  });

  expect(screen.queryByText("GLOBAL")).not.toBeInTheDocument();
  expect(screen.getByText("JP")).toBeInTheDocument();
});
