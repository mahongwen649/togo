import { describe, expect, it } from "vitest";

import type { ChannelModel } from "./api";
import {
  GROK_ASPECT_RATIOS,
  isGrokImagineModel,
  isLikelyImageOnlyModel,
  PRESET_MODELS,
  selectModelForCapability,
} from "./modelCatalog";

describe("TogoAPI preset catalog", () => {
  it("exposes GPT and Grok image presets", () => {
    expect(PRESET_MODELS).toEqual([
      {
        id: "gpt-image-2",
        label: "GPT Image 2",
        vendor: "OpenAI 生图",
        capabilities: ["text-to-image", "image-to-image"]
      },
      {
        id: "gpt-5.5",
        label: "GPT-5.5",
        vendor: "OpenAI 图生文 / 视觉理解",
        capabilities: ["image-to-text"]
      },
      {
        id: "grok-imagine-image-2.0",
        label: "Grok Imagine 2.0",
        vendor: "Grok 生图",
        capabilities: ["text-to-image", "image-to-image"]
      },
      {
        id: "grok-4.6",
        label: "Grok 4.6",
        vendor: "Grok 图生文 / 视觉理解",
        capabilities: ["image-to-text"]
      }
    ]);
  });

  it("treats Grok Imagine as an image-only model", () => {
    expect(isGrokImagineModel("grok-imagine-image-2.0")).toBe(true);
    expect(isGrokImagineModel("xai/grok-imagine-image-2.0")).toBe(true);
    expect(isLikelyImageOnlyModel("grok-imagine-image-2.0")).toBe(true);
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
