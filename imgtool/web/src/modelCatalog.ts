import type { Capability, ChannelModel } from "./api";

export const MODEL_CAPABILITIES: Capability[] = ["text-to-image", "image-to-image"];
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

export function isValidGrokAspectRatio(value: string) {
  return (GROK_ASPECT_RATIOS as readonly string[]).includes(value.trim());
}

export function isValidGrokResolution(value: string) {
  return (GROK_RESOLUTIONS as readonly string[]).includes(value.trim());
}

export function isValidGrokQuality(value: string) {
  return (GROK_QUALITY_PRESETS as readonly string[]).includes(value.trim());
}

export function selectModelForCapability<T extends Pick<ChannelModel, "id" | "capabilities">>(
  models: readonly T[],
  currentModelId: string,
  capability: Capability
) {
  const candidates = models.filter((model) => model.capabilities.includes(capability));
  if (candidates.length === 0) return undefined;
  const current = candidates.find((model) => model.id === currentModelId);
  return current ?? candidates[0];
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
