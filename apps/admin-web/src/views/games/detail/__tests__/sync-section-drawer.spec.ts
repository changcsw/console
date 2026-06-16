import { fireEvent, render, screen } from "@testing-library/vue";
import CopyPublishedToDraftDialog from "@/views/cashier/templates/components/CopyPublishedToDraftDialog.vue";
import SyncSectionDrawer from "../components/SyncSectionDrawer.vue";

test("execute payload only includes selected sections", async () => {
  render(SyncSectionDrawer, {
    props: {
      open: true,
      preview: [{ section: "channels" }, { section: "payments" }]
    }
  });

  await fireEvent.click(screen.getByLabelText("channels"));
  await fireEvent.click(screen.getByText("Execute"));

  screen.getByText("selected_sections: channels");
});

test("published version can be copied into draft", () => {
  render(CopyPublishedToDraftDialog, {
    props: {
      open: true,
      sourceVersion: { version: 7, status: "published" }
    }
  });

  screen.getByText("Create draft from published v7");
});
