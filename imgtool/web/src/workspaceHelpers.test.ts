import { describe, expect, it, vi } from "vitest";
import { listHistory } from "./api";
import type { HistoryRecord } from "./api";
import { mergeFeedItems, recentHistoryFeedItems } from "./workspaceHelpers";
import type { ArtworkFeedItem } from "./workspaceTypes";

vi.mock("./api", () => ({
  generateImageEdit: vi.fn(),
  getSignedFileURL: vi.fn(),
  listHistory: vi.fn()
}));

function feedItem(id: string, createdAt: number): ArtworkFeedItem {
  return {
    id,
    status: "完成",
    prompt: id,
    channelName: "jq",
    modelId: "gpt-image-2",
    capability: "text-to-image",
    parameters: {},
    files: [],
    createdAt
  };
}

describe("mergeFeedItems", () => {
  it("keeps newest artwork closest to the composer at the bottom", () => {
    const items = mergeFeedItems([feedItem("old", 1000), feedItem("new", 2000)]);

    expect(items.map((item) => item.id)).toEqual(["old", "new"]);
  });

  it("keeps the newest 20 items when the workspace feed is full", () => {
    const existingItems = Array.from({ length: 20 }, (_, index) => feedItem(`old-${index + 1}`, index + 1));
    const items = mergeFeedItems([feedItem("new", 100), ...existingItems]);

    expect(items).toHaveLength(20);
    expect(items.map((item) => item.id)).not.toContain("old-1");
    expect(items.map((item) => item.id)).toContain("new");
    expect(items[items.length - 1].id).toBe("new");
  });
});

describe("recentHistoryFeedItems", () => {
  it("keeps failed records out of the workspace feed", async () => {
    const failedRecord: HistoryRecord = {
      id: "failed-edit",
      taskId: "task-1",
      kind: "image",
      status: "失败",
      channelName: "jq",
      modelId: "gpt-image-2",
      capability: "image-to-image",
      prompt: "change background",
      parameters: {},
      failureReason: "upstream rejected image edit",
      files: [],
      createdAt: 2000
    };
    const completedRecord: HistoryRecord = {
      id: "completed-image",
      taskId: "task-2",
      kind: "image",
      status: "完成",
      channelName: "jq",
      modelId: "gpt-image-2",
      capability: "text-to-image",
      prompt: "finished image",
      parameters: {},
      files: [],
      createdAt: 1000
    };
    vi.mocked(listHistory).mockResolvedValueOnce({
      ok: true,
      data: { records: [failedRecord, completedRecord] }
    });

    const items = await recentHistoryFeedItems();

    expect(items).toMatchObject([
      {
        id: "completed-image",
        status: "完成"
      }
    ]);
    expect(items.some((item) => item.status === "失败")).toBe(false);
  });
});
