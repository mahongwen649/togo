import type { Capability, ChannelModel } from "./api";

export type PresetModel = { id: string; label: string; vendor: string; capabilities: Capability[] };

export const PRESET_MODELS: PresetModel[] = [
  { id: "gpt-image-2", label: "GPT Image 2", vendor: "OpenAI 生图", capabilities: ["text-to-image", "image-to-image"] },
  { id: "gpt-5.5", label: "GPT-5.5", vendor: "OpenAI 图生文 / 视觉理解", capabilities: ["image-to-text"] },
  { id: "grok-imagine-image-2.0", label: "Grok Imagine 2.0", vendor: "Grok 生图", capabilities: ["text-to-image", "image-to-image"] },
  { id: "grok-4.6", label: "Grok 4.6", vendor: "Grok 图生文 / 视觉理解", capabilities: ["image-to-text"] },
];

export const PRESET_MODEL_GROUPS = PRESET_MODELS.reduce<Array<{ vendor: string; models: PresetModel[] }>>((groups, model) => {
  const group = groups.find((item) => item.vendor === model.vendor);
  if (group) {
    group.models.push(model);
  } else {
    groups.push({ vendor: model.vendor, models: [model] });
  }
  return groups;
}, []);

export const MODEL_CAPABILITIES: Capability[] = ["text-to-image", "image-to-image", "image-to-text"];
export const QUALITY_PRESETS = ["1K", "2K", "4K"] as const;
export const SIZE_PRESETS = ["1024x1024", "1536x1024", "1024x1536"];
// Keep this list aligned with Core's Grok Imagine geometry normalizer.
export const GROK_ASPECT_RATIOS = [
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
] as const;
export const GROK_RESOLUTIONS = ["1k", "2k"] as const;
export const GROK_QUALITY_PRESETS = ["low", "medium"] as const;
export const DEFAULT_GROK_ASPECT_RATIO = "1:1";
export const DEFAULT_GROK_EDIT_ASPECT_RATIO = "auto";
export const DEFAULT_GROK_RESOLUTION = "1k";
export const DEFAULT_GROK_QUALITY = "low";

export function modelFromId(modelId: string, capabilities: Capability[] = ["text-to-image"]): ChannelModel {
  return {
    id: modelId.trim(),
    capabilities
  };
}

export function parseModelIdList(input: string) {
  return input
    .split(/[\s,，;；]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

export function isGrokImagineModel(modelId: string) {
  const normalized = normalizeGrokModelID(modelId);
  return (
    normalized === "grok-imagine" ||
    normalized === "grok-imagine-edit" ||
    normalized.startsWith("grok-imagine-image")
  );
}

function normalizeGrokModelID(modelId: string) {
  const normalized = modelId.trim().toLowerCase();
  for (const prefix of ["xai/", "x-ai/", "grok/"]) {
    if (normalized.startsWith(prefix)) return normalized.slice(prefix.length);
  }
  return normalized;
}

export function isLikelyImageOnlyModel(modelId: string) {
  const normalized = modelId.trim().toLowerCase();
  return (
    normalized.includes("gpt-image") ||
    normalized.includes("seedream") ||
    normalized.includes("imagen") ||
    isGrokImagineModel(normalized)
  );
}

export function isValidGrokAspectRatio(value: string) {
  return (GROK_ASPECT_RATIOS as readonly string[]).includes(value.trim());
}

export function isValidGrokResolution(value: string) {
  return (GROK_RESOLUTIONS as readonly string[]).includes(value.trim());
}

export function isValidGrokQuality(value: string) {
  return (GROK_QUALITY_PRESETS as readonly string[]).includes(value.trim());
}

export function imageToTextModelHint(modelId: string) {
  return `图生文需要视觉理解/多模态聊天模型；当前 ${modelId} 更像生图模型。请在模型设置里添加视觉模型 ID，或切回文生图/图生图。`;
}

export function selectModelForCapability<T extends Pick<ChannelModel, "id" | "capabilities">>(
  models: readonly T[],
  currentModelId: string,
  capability: Capability
) {
  const candidates = models.filter((model) => model.capabilities.includes(capability));
  if (candidates.length === 0) return undefined;
  const current = candidates.find((model) => model.id === currentModelId);
  if (current && (capability !== "image-to-text" || isGoodFitForCapability(current.id, capability))) return current;
  return candidates.find((model) => isGoodFitForCapability(model.id, capability)) ?? candidates[0];
}

function isGoodFitForCapability(modelId: string, capability: Capability) {
  if (capability === "image-to-text") return !isLikelyImageOnlyModel(modelId);
  if (capability === "text-to-image" || capability === "image-to-image") return isLikelyImageOnlyModel(modelId);
  return true;
}

export function capabilityLabel(capability: string) {
  if (capability === "text-to-image") return "文生图";
  if (capability === "image-to-image") return "图生图";
  if (capability === "image-to-text") return "图生文";
  return capability;
}

export function filenameFromObjectKey(objectKey: string) {
  const filename = objectKey.split("/").filter(Boolean).pop()?.replaceAll('"', "").trim();
  return filename || "image.png";
}
