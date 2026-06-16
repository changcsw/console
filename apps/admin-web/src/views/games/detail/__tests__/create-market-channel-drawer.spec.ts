import { fireEvent, render, screen } from "@testing-library/vue";
import CreateMarketChannelDrawer from "../components/CreateMarketChannelDrawer.vue";

test("copy mode clears secret and file fields", async () => {
  render(CreateMarketChannelDrawer, {
    props: {
      open: true,
      gameId: "game-1",
      selectedMarket: "JP",
      availableMarkets: ["GLOBAL", "JP", "CN"],
      sourceInstance: {
        market: "GLOBAL",
        channelId: "google",
        normalConfig: { clientId: "global-id" }
      }
    }
  });

  await fireEvent.update(screen.getByLabelText("clientSecret"), "filled-secret");
  await fireEvent.update(screen.getByLabelText("keystoreFile"), "filled-file");
  await fireEvent.click(screen.getByRole("button", { name: "Copy from existing market" }));

  expect((screen.getByLabelText("clientId") as HTMLInputElement).value).toBe("global-id");
  expect((screen.getByLabelText("clientSecret") as HTMLInputElement).value).toBe("");
  expect((screen.getByLabelText("keystoreFile") as HTMLInputElement).value).toBe("");
});
