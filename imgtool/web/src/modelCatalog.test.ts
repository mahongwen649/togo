import { describe, expect, it } from "vitest";

import type { ChannelModel } from "./api";
import {
  GROK_ASPECT_RATIOS,
  isGrokImagineModel,
  selectModelForCapability,
} from "./modelCatalog";

describe("image model helpers", () => {
  it("recognizes Grok Imagine model IDs with provider prefixes", () => {
    expect(isGrokImagineModel("grok-imagine-image-2.0")).toBe(true);
    expect(isGrokImagineModel("xai/grok-imagine-image-2.0")).toBe(true);
    expect(isGrokImagineModel("gpt-image-2")).toBe(false);
  });

  it("exposes all Core-supported Grok Imagine aspect ratios", () => {
    expect(GROK_ASPECT_RATIOS).toEqual([
      "1:1",
      "16:9",
      "9:16",
      "4:3",
      "3:4",
      "3:2",
      "2:3",
      "2:1",
      "1:2",
      "19.5:9",
      "9:19.5",
      "20:9",
      "9:20",
      "auto",
    ]);
  });
});

describe("selectModelForCapability", () => {
  it("keeps a configured custom image model selected", () => {
    const models: ChannelModel[] = [
      { id: "gemini-3-pro-image-preview", capabilities: ["text-to-image"] },
      { id: "gemini-3.1-flash-image-preview", capabilities: ["text-to-image"] }
    ];

    expect(selectModelForCapability(models, "gemini-3.1-flash-image-preview", "text-to-image")?.id).toBe(
      "gemini-3.1-flash-image-preview"
    );
  });
});
