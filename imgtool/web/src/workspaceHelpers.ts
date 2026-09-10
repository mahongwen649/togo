import type { Capability, HistoryRecord, ResultFile, Task } from "./api";
import {
  generateImageEdit,
  generateText,
  getSignedFileURL,
  listHistory,
} from "./api";
import {
  isValidGrokAspectRatio,
  isValidGrokQuality,
  isValidGrokResolution,
  MODEL_CAPABILITIES,
  QUALITY_PRESETS,
  SIZE_PRESETS,
} from "./modelCatalog";
import type {
  ArtworkFeedItem,
  ComposerPreferences,
  PreviewFile,
  ReuseHistoryDraft,
  RunningFeedTask,
} from "./workspaceTypes";
import { COMPOSER_PREFS_KEY_PREFIX, REUSE_HISTORY_KEY } from "./workspaceTypes";

export async function submitImageToText(input: {
  channelId: string;
  modelId: string;
  prompt: string;
  image: File;
}) {
  const formData = new FormData();
  formData.set("channelId", input.channelId);
  formData.set("modelId", input.modelId);
  formData.set("prompt", input.prompt);
  formData.set("inputImage", input.image);
  return generateText(formData);
}

export async function submitImageToImage(input: {
  channelId: string;
  modelId: string;
  prompt: string;
  size?: string;
  aspectRatio?: string;
  resolution?: string;
  image: File;
}) {
  const formData = new FormData();
  formData.set("channelId", input.channelId);
  formData.set("modelId", input.modelId);
  formData.set("prompt", input.prompt);
  if (input.size) formData.set("size", input.size);
  if (input.aspectRatio) formData.set("aspectRatio", input.aspectRatio);
  if (input.resolution) formData.set("resolution", input.resolution);
  formData.set("inputImage", input.image);
  return generateImageEdit(formData);
}

export async function historyRecordToFeedItem(
  record: HistoryRecord,
): Promise<ArtworkFeedItem> {
  return {
    id: record.id,
    status: record.status,
    prompt: record.prompt,
    channelName: record.channelName,
    modelId: record.modelId,
    capability: record.capability,
    parameters: record.parameters,
    resultText: record.resultText,
    failureReason: record.failureReason,
    durationMs: record.durationMs,
    files: await filesWithPreviews(record.files),
    createdAt: record.createdAt,
  };
}

export async function recentHistoryFeedItems(): Promise<ArtworkFeedItem[]> {
  const payload = await listHistory();
  if (!payload.ok) return [];
  return Promise.all(
    payload.data.records
      .slice()
      .sort((left, right) => right.createdAt - left.createdAt)
      .filter((record) => record.status !== "失败")
      .slice(0, 20)
      .map(historyRecordToFeedItem),
  );
}

export function taskToRunningFeedTask(task: Task): RunningFeedTask {
  return {
    id: task.id,
    prompt: task.prompt,
    channelName: task.channelName,
    modelId: task.modelId,
    capability: task.capability,
    phase: "generating",
    startedAt: task.startedAt,
  };
}

export async function filesWithPreviews(
  files: ResultFile[],
): Promise<PreviewFile[]> {
  return Promise.all(
    files.map(async (file) => {
      const signed = await getSignedFileURL(file.id);
      return { ...file, previewUrl: signed.ok ? signed.data.url : undefined };
    }),
  );
}

export function mergeFeedItems(items: ArtworkFeedItem[]) {
  const byId = new Map<string, ArtworkFeedItem>();
  for (const item of items) {
    const current = byId.get(item.id);
    if (!current || item.createdAt >= current.createdAt) {
      byId.set(item.id, item);
    }
  }
  return Array.from(byId.values())
    .sort((left, right) => left.createdAt - right.createdAt)
    .slice(-20);
}

export function formatElapsed(milliseconds: number) {
  const seconds = Math.max(0, Math.floor(milliseconds / 1000));
  if (seconds < 60) return `${seconds}秒`;
  const minutes = Math.floor(seconds / 60);
  const restSeconds = seconds % 60;
  return `${minutes}分${restSeconds.toString().padStart(2, "0")}秒`;
}

export function uniqueValues(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort((left, right) =>
    left.localeCompare(right),
  );
}

function optionalString(value: unknown) {
  return typeof value === "string" ? value : undefined;
}

function optionalQuality(value: unknown) {
  if (typeof value !== "string") return undefined;
  if (
    QUALITY_PRESETS.includes(value as (typeof QUALITY_PRESETS)[number]) ||
    isValidGrokQuality(value)
  ) {
    return value;
  }
  return undefined;
}

export function readReuseHistoryDraft(): ReuseHistoryDraft | undefined {
  const raw = sessionStorage.getItem(REUSE_HISTORY_KEY);
  if (!raw) return undefined;
  sessionStorage.removeItem(REUSE_HISTORY_KEY);
  try {
    const parsed = JSON.parse(raw) as Partial<ReuseHistoryDraft>;
    if (
      typeof parsed.channelName !== "string" ||
      typeof parsed.modelId !== "string" ||
      typeof parsed.prompt !== "string" ||
      !MODEL_CAPABILITIES.includes(parsed.capability as Capability)
    ) {
      return undefined;
    }
    return {
      channelName: parsed.channelName,
      modelId: parsed.modelId,
      capability: parsed.capability as Capability,
      prompt: parsed.prompt,
      size: optionalString(parsed.size),
      aspectRatio: optionalString(parsed.aspectRatio),
      resolution: optionalString(parsed.resolution),
      quality: optionalQuality(parsed.quality),
      referenceFileId: optionalString(parsed.referenceFileId),
      referenceObjectKey: optionalString(parsed.referenceObjectKey),
      referenceMimeType: optionalString(parsed.referenceMimeType),
      notice: optionalString(parsed.notice),
    };
  } catch {
    return undefined;
  }
}

function composerPreferencesKey(username: string) {
  return `${COMPOSER_PREFS_KEY_PREFIX}:${username || "user"}`;
}

export function readComposerPreferences(
  username: string,
): ComposerPreferences | undefined {
  const raw = localStorage.getItem(composerPreferencesKey(username));
  if (!raw) return undefined;
  try {
    const parsed = JSON.parse(raw) as Partial<ComposerPreferences>;
    return {
      channelId:
        typeof parsed.channelId === "string" ? parsed.channelId : undefined,
      modelId: typeof parsed.modelId === "string" ? parsed.modelId : undefined,
      capability: MODEL_CAPABILITIES.includes(parsed.capability as Capability)
        ? (parsed.capability as Capability)
        : undefined,
      size:
        typeof parsed.size === "string" && isValidImageSize(parsed.size)
          ? parsed.size
          : undefined,
      aspectRatio:
        typeof parsed.aspectRatio === "string" &&
        isValidGrokAspectRatio(parsed.aspectRatio)
          ? parsed.aspectRatio
          : undefined,
      resolution:
        typeof parsed.resolution === "string" &&
        isValidGrokResolution(parsed.resolution)
          ? parsed.resolution
          : undefined,
      quality:
        typeof parsed.quality === "string" && isValidGrokQuality(parsed.quality)
          ? parsed.quality
          : undefined,
    };
  } catch {
    return undefined;
  }
}

export function saveComposerPreferences(
  username: string,
  preferences: ComposerPreferences,
) {
  localStorage.setItem(
    composerPreferencesKey(username),
    JSON.stringify(preferences),
  );
}

export function isValidImageSize(value: string) {
  return SIZE_PRESETS.includes(value.trim());
}
