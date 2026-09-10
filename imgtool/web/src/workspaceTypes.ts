import type { Capability, ResultFile } from "./api";

export const REUSE_HISTORY_KEY = "imgtool:reuse-history-record";
export const COMPOSER_PREFS_KEY_PREFIX = "imgtool:composer-preferences";

export type ComposerPreferences = {
  channelId?: string;
  modelId?: string;
  capability?: Capability;
  size?: string;
  aspectRatio?: string;
  resolution?: string;
  quality?: string;
};

export type PreviewFile = ResultFile & { previewUrl?: string };

export type ArtworkFeedItem = {
  id: string;
  status: string;
  prompt: string;
  channelName: string;
  modelId: string;
  capability: Capability;
  parameters: Record<string, unknown>;
  resultText?: string;
  failureReason?: string;
  durationMs?: number;
  files: PreviewFile[];
  createdAt: number;
};

export type RunningFeedTask = {
  id?: string;
  prompt: string;
  channelName: string;
  modelId: string;
  capability: Capability;
  phase?: "uploading" | "submitting" | "generating" | "downloading";
  startedAt: number;
  referencePreviewUrl?: string;
  referenceName?: string;
};

export type ReuseHistoryDraft = {
  channelName: string;
  modelId: string;
  capability: Capability;
  prompt: string;
  size?: string;
  aspectRatio?: string;
  resolution?: string;
  quality?: string;
  referenceFileId?: string;
  referenceObjectKey?: string;
  referenceMimeType?: string;
  notice?: string;
};
