import { fireEvent, render, screen, waitFor } from "@testing-library/vue";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { ElMessage, ElMessageBox } from "element-plus";
import { ApiError } from "@/api/http";
import SyncSectionDrawer from "../components/SyncSectionDrawer.vue";
import type { DiffSection, SyncPreviewResponse } from "@/api/syncSections";

// sync #21 · SyncSectionDrawer 组件测试（L4 vitest，mock API）。
// 覆盖 compact 前端要点：预览分组+计数徽标、差异行配色、update sandbox→production 对照、
// section 级复选框、include_deletes 默认关 + delete「仅提示不执行」、密文 masked 不可展开、
// 确认 payload(baselineToken+selectedSections+includeDeletes+operatorNote)、结果反馈与错误码分支。

const previewApi = vi.fn();
const executeApi = vi.fn();

vi.mock("@/api/syncSections", () => ({
  previewSyncSections: (...args: unknown[]) => previewApi(...args),
  executeSyncSections: (...args: unknown[]) => executeApi(...args)
}));

function channelsSection(over: Partial<DiffSection> = {}): DiffSection {
  return {
    section: "channels",
    summary: { add: 1, update: 1, delete: 1 },
    dependencies: ["game", "markets"],
    changes: [
      {
        op: "add",
        entityType: "game_channel",
        entityKey: "JP/google",
        fieldName: "*",
        sandboxValue: { market: "JP", channelId: "google", enabled: true },
        productionValue: null,
        masked: false
      },
      {
        op: "update",
        entityType: "game_channel_login_config",
        entityKey: "JP/google",
        fieldName: "clientSecret",
        sandboxValue: "SANDBOX_PLAINTEXT_SECRET",
        productionValue: "PROD_PLAINTEXT_SECRET",
        masked: true
      },
      {
        op: "delete",
        entityType: "game_channel",
        entityKey: "KR/apple",
        fieldName: "*",
        sandboxValue: null,
        productionValue: { market: "KR", channelId: "apple" },
        masked: false
      }
    ],
    ...over
  };
}

function paymentsSection(over: Partial<DiffSection> = {}): DiffSection {
  return {
    section: "payments",
    summary: { add: 0, update: 1, delete: 0 },
    dependencies: ["channels", "packages"],
    changes: [
      {
        op: "update",
        entityType: "payment_route",
        entityKey: "card/*/google/JP/*/USD",
        fieldName: "merchantId",
        sandboxValue: "M-SANDBOX",
        productionValue: "M-PROD",
        masked: false
      }
    ],
    ...over
  };
}

function makePreview(over: Partial<SyncPreviewResponse> = {}): SyncPreviewResponse {
  return {
    gameId: "100001",
    sourceEnv: "sandbox",
    targetEnv: "production",
    sourceHash: "sha256-source-hash-value",
    targetHashBefore: "sha256-target-before-hash",
    hasDiff: true,
    baselineToken: "baseline-token-abc.def",
    previewedAt: "2026-07-01T00:00:00Z",
    expiresAt: "2026-07-01T00:30:00Z",
    sections: [channelsSection(), paymentsSection()],
    ...over
  };
}

function renderDrawer() {
  return render(SyncSectionDrawer, {
    props: { open: true, gameId: "100001" }
  });
}

async function waitForPreview() {
  await waitFor(() => expect(previewApi).toHaveBeenCalledWith("100001", {}));
  await waitFor(() =>
    expect(document.body.querySelector(".sync-section-card")).not.toBeNull()
  );
}

function executeButton(): HTMLButtonElement {
  const buttons = Array.from(document.body.querySelectorAll("button"));
  const btn = buttons.find((b) => b.textContent?.includes("执行同步"));
  if (!btn) {
    throw new Error("执行同步按钮未找到");
  }
  return btn as HTMLButtonElement;
}

beforeEach(() => {
  vi.clearAllMocks();
  document.body.innerHTML = "";
  vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
  vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
});

describe("SyncSectionDrawer · 预览与展示", () => {
  test("打开抽屉自动预览全 section（默认空 body）", async () => {
    previewApi.mockResolvedValue(makePreview());
    renderDrawer();
    await waitForPreview();
    expect(previewApi).toHaveBeenCalledWith("100001", {});
  });

  test("按 section 分组并显示 add/update/delete 计数徽标", async () => {
    previewApi.mockResolvedValue(makePreview());
    renderDrawer();
    await waitForPreview();

    const html = document.body.innerHTML;
    // channels 组徽标 add1/update1/delete1
    expect(html).toContain("add 1");
    expect(html).toContain("update 1");
    expect(html).toContain("delete 1");
    // 两个分组均渲染
    expect(screen.getByText("channels")).toBeInTheDocument();
    expect(screen.getByText("payments")).toBeInTheDocument();
    // 依赖提示
    expect(document.body.textContent).toContain("game / markets");
  });

  test("差异行按 op 配色：add 绿 / update 黄 / delete 红", async () => {
    previewApi.mockResolvedValue(makePreview({ sections: [channelsSection()] }));
    renderDrawer();
    await waitForPreview();

    expect(document.body.querySelector(".sync-change--add")).not.toBeNull();
    expect(document.body.querySelector(".sync-change--update")).not.toBeNull();
    expect(document.body.querySelector(".sync-change--delete")).not.toBeNull();
  });

  test("update 行展示 sandbox→production 对照", async () => {
    previewApi.mockResolvedValue(makePreview({ sections: [paymentsSection()] }));
    renderDrawer();
    await waitForPreview();

    const text = document.body.textContent ?? "";
    expect(text).toContain("sandbox:");
    expect(text).toContain("production:");
    expect(text).toContain("M-SANDBOX");
    expect(text).toContain("M-PROD");
  });

  test("密文 masked 恒显示 ••••••，绝不出现明文", async () => {
    previewApi.mockResolvedValue(makePreview({ sections: [channelsSection()] }));
    renderDrawer();
    await waitForPreview();

    const text = document.body.textContent ?? "";
    expect(text).toContain("••••••");
    // masked=true 的 clientSecret 明文两侧值均被脱敏
    expect(text).not.toContain("SANDBOX_PLAINTEXT_SECRET");
    expect(text).not.toContain("PROD_PLAINTEXT_SECRET");
  });

  test("hasDiff=false：提示无差异且执行按钮禁用", async () => {
    previewApi.mockResolvedValue(
      makePreview({ hasDiff: false, sections: [] })
    );
    renderDrawer();
    await waitFor(() => expect(previewApi).toHaveBeenCalled());
    await waitFor(() =>
      expect(document.body.textContent).toContain("无差异")
    );
    expect(executeButton()).toBeDisabled();
  });
});

describe("SyncSectionDrawer · include_deletes opt-in", () => {
  test("默认关闭时 delete 行标注「仅提示，不执行」且置灰", async () => {
    previewApi.mockResolvedValue(makePreview({ sections: [channelsSection()] }));
    renderDrawer();
    await waitForPreview();

    expect(document.body.textContent).toContain("仅提示，不执行");
    expect(document.body.querySelector(".sync-change--delete-muted")).not.toBeNull();
  });

  test("开启 include_deletes 后 delete 行不再置灰", async () => {
    previewApi.mockResolvedValue(makePreview({ sections: [channelsSection()] }));
    renderDrawer();
    await waitForPreview();

    const toggle = screen.getByRole("switch");
    await fireEvent.click(toggle);
    await waitFor(() =>
      expect(document.body.querySelector(".sync-change--delete-muted")).toBeNull()
    );
  });
});

describe("SyncSectionDrawer · 执行 payload", () => {
  test("默认全选 → payload 含 baselineToken+全 selectedSections+includeDeletes(false)+operatorNote", async () => {
    previewApi.mockResolvedValue(makePreview());
    executeApi.mockResolvedValue({});
    renderDrawer();
    await waitForPreview();

    await fireEvent.update(screen.getByPlaceholderText("备注（可选）"), "上线备注");
    await fireEvent.click(executeButton());

    await waitFor(() =>
      expect(executeApi).toHaveBeenCalledWith("100001", {
        selectedSections: ["channels", "payments"],
        baselineToken: "baseline-token-abc.def",
        includeDeletes: false,
        operatorNote: "上线备注"
      })
    );
  });

  test("取消勾选某 section 后 payload 仅含所选 section", async () => {
    previewApi.mockResolvedValue(makePreview());
    executeApi.mockResolvedValue({});
    renderDrawer();
    await waitForPreview();

    // 默认全选，取消 payments → 仅剩 channels
    await fireEvent.click(screen.getByLabelText("payments"));
    await fireEvent.click(executeButton());

    await waitFor(() =>
      expect(executeApi).toHaveBeenCalledWith("100001", {
        selectedSections: ["channels"],
        baselineToken: "baseline-token-abc.def",
        includeDeletes: false,
        operatorNote: ""
      })
    );
  });

  test("include_deletes 开启后 payload includeDeletes=true", async () => {
    previewApi.mockResolvedValue(makePreview());
    executeApi.mockResolvedValue({});
    renderDrawer();
    await waitForPreview();

    await fireEvent.click(screen.getByRole("switch"));
    await fireEvent.click(executeButton());

    await waitFor(() =>
      expect(executeApi).toHaveBeenCalledWith(
        "100001",
        expect.objectContaining({ includeDeletes: true })
      )
    );
  });

  test("全部取消勾选时执行按钮禁用，不触发 execute", async () => {
    previewApi.mockResolvedValue(makePreview());
    executeApi.mockResolvedValue({});
    renderDrawer();
    await waitForPreview();

    await fireEvent.click(screen.getByLabelText("channels"));
    await fireEvent.click(screen.getByLabelText("payments"));

    await waitFor(() => expect(executeButton()).toBeDisabled());
    await fireEvent.click(executeButton());
    expect(executeApi).not.toHaveBeenCalled();
  });
});

describe("SyncSectionDrawer · 结果反馈与错误码", () => {
  test("成功：toast 提示并 emit executed", async () => {
    previewApi.mockResolvedValue(makePreview());
    const result = { syncJobId: "9012", status: "succeeded" };
    executeApi.mockResolvedValue(result);
    const { emitted } = renderDrawer();
    await waitForPreview();

    await fireEvent.click(executeButton());

    await waitFor(() => expect(ElMessage.success).toHaveBeenCalledWith("同步执行成功"));
    await waitFor(() => expect(emitted().executed).toBeTruthy());
    expect(emitted().executed[0]).toEqual([result]);
  });

  test("SYNC_BASELINE_MISMATCH：弹「目标已变更，请重新预览」，确认后重新预览", async () => {
    previewApi.mockResolvedValue(makePreview());
    executeApi.mockRejectedValue(new ApiError(409, "SYNC_BASELINE_MISMATCH", "baseline changed"));
    const confirmSpy = vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    renderDrawer();
    await waitForPreview();
    expect(previewApi).toHaveBeenCalledTimes(1);

    await fireEvent.click(executeButton());

    await waitFor(() =>
      expect(confirmSpy).toHaveBeenCalledWith(
        expect.anything(),
        "目标已变更，请重新预览",
        expect.objectContaining({ confirmButtonText: "重新预览" })
      )
    );
    // 确认 → 重新 loadPreview
    await waitFor(() => expect(previewApi).toHaveBeenCalledTimes(2));
  });

  test("SYNC_TOKEN_CONSUMED：弹「预览凭证已使用，请重新预览」", async () => {
    previewApi.mockResolvedValue(makePreview());
    executeApi.mockRejectedValue(new ApiError(409, "SYNC_TOKEN_CONSUMED", "token consumed"));
    const confirmSpy = vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    renderDrawer();
    await waitForPreview();

    await fireEvent.click(executeButton());

    await waitFor(() =>
      expect(confirmSpy).toHaveBeenCalledWith(
        expect.anything(),
        "预览凭证已使用，请重新预览",
        expect.objectContaining({ confirmButtonText: "重新预览" })
      )
    );
  });

  test("VALIDATION_FAILED：行内展示缺失依赖 details", async () => {
    previewApi.mockResolvedValue(makePreview());
    executeApi.mockRejectedValue(
      new ApiError(400, "VALIDATION_FAILED", "dependency missing", [
        { section: "packages", missingDependency: "channels", entityKey: "JP/google" }
      ])
    );
    renderDrawer();
    await waitForPreview();

    await fireEvent.click(executeButton());

    await waitFor(() => {
      const text = document.body.textContent ?? "";
      expect(text).toContain("依赖校验失败");
      expect(text).toContain("section=packages");
      expect(text).toContain("missing=channels");
      expect(text).toContain("key=JP/google");
    });
  });

  test("UNKNOWN_SECTION：行内展示 section 非法提示", async () => {
    previewApi.mockResolvedValue(makePreview());
    executeApi.mockRejectedValue(
      new ApiError(400, "UNKNOWN_SECTION", "unknown section", [{ section: "bogus" }])
    );
    renderDrawer();
    await waitForPreview();

    await fireEvent.click(executeButton());

    await waitFor(() =>
      expect(document.body.textContent).toContain("section 非法")
    );
  });

  test("预览失败展示错误态", async () => {
    previewApi.mockRejectedValue(new ApiError(500, "INTERNAL", "boom"));
    renderDrawer();
    await waitFor(() => expect(previewApi).toHaveBeenCalled());
    await waitFor(() => expect(document.body.textContent).toContain("boom"));
  });
});
