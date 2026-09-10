import {
  type ClipboardEvent,
  type CSSProperties,
  type FormEvent,
  type KeyboardEvent,
  useEffect,
  useRef,
  useState,
} from "react";
import {
  ImagePlus,
  Layers,
  LoaderCircle,
  Maximize2,
  Send,
  SlidersHorizontal,
  X,
  Zap,
} from "lucide-react";
import type {
  AdminUser,
  Capability,
  Channel,
  ImageProvider,
  HistoryRecord,
  ResultFile,
} from "./api";
import {
  cancelTask,
  createAdminUser,
  deleteHistoryRecord,
  generateImage,
  getFileContent,
  getSignedFileDownloadURL,
  getSignedFileURL,
  getTask,
  listAdminUsers,
  listProviders,
  listHistory,
  listTasks,
  resetAdminUserPassword,
  setAdminUserDisabled,
} from "./api";
import { useAuth } from "./auth";
import {
  AppContent,
  AppProviders,
  ForbiddenPage,
  PageHeader,
  Shell,
} from "./layout";
import {
  capabilityLabel,
  DEFAULT_GROK_ASPECT_RATIO,
  DEFAULT_GROK_EDIT_ASPECT_RATIO,
  DEFAULT_GROK_QUALITY,
  DEFAULT_GROK_RESOLUTION,
  filenameFromObjectKey,
  GROK_ASPECT_RATIOS,
  GROK_QUALITY_PRESETS,
  GROK_RESOLUTIONS,
  isGrokImagineModel,
  isValidGrokAspectRatio,
  isValidGrokQuality,
  isValidGrokResolution,
  MODEL_CAPABILITIES,
  selectModelForCapability,
  SIZE_PRESETS,
} from "./modelCatalog";
import type {
  ArtworkFeedItem,
  ComposerPreferences,
  PreviewFile,
  ReuseHistoryDraft,
  RunningFeedTask,
} from "./workspaceTypes";
import { REUSE_HISTORY_KEY } from "./workspaceTypes";
import {
  filesWithPreviews,
  formatElapsed,
  historyRecordToFeedItem,
  isValidImageSize,
  mergeFeedItems,
  readComposerPreferences,
  readReuseHistoryDraft,
  recentHistoryFeedItems,
  saveComposerPreferences,
  submitImageToImage,
  taskToRunningFeedTask,
  uniqueValues,
} from "./workspaceHelpers";
import "./styles.css";

function isReferenceFile(file: ResultFile) {
  return file.role === "reference";
}

function imageFiles<T extends ResultFile>(files: T[]) {
  return files.filter((file) => file.mediaType === "image");
}

function referenceImageFiles<T extends ResultFile>(files: T[]) {
  return imageFiles(files).filter(isReferenceFile);
}

function resultImageFiles<T extends ResultFile>(files: T[]) {
  return imageFiles(files).filter((file) => !isReferenceFile(file));
}

function orderedDisplayFiles<T extends ResultFile>(files: T[]) {
  return files.slice().sort((left, right) => {
    const roleOrder =
      Number(isReferenceFile(left)) - Number(isReferenceFile(right));
    if (roleOrder !== 0) return -roleOrder;
    const createdOrder = (left.createdAt ?? 0) - (right.createdAt ?? 0);
    if (createdOrder !== 0) return createdOrder;
    return left.id.localeCompare(right.id);
  });
}

function firstImageFile<T extends ResultFile>(files: T[]) {
  return resultImageFiles(files)[0] ?? imageFiles(files)[0];
}

function firstReferenceImageFile<T extends ResultFile>(files: T[]) {
  return referenceImageFiles(files)[0];
}

function fileDisplayLabel(file: ResultFile, files: ResultFile[]) {
  if (isReferenceFile(file)) return "垫图";
  const resultIndex = resultImageFiles(files).findIndex(
    (item) => item.id === file.id,
  );
  return `作品 ${resultIndex >= 0 ? resultIndex + 1 : 1}`;
}

function referenceActionLabel(
  file: ResultFile,
  files: ResultFile[],
  prompt: string,
  scope = "",
) {
  if (isReferenceFile(file)) return `将${scope}垫图 ${prompt} 设为参考图`;
  const resultIndex = resultImageFiles(files).findIndex(
    (item) => item.id === file.id,
  );
  return `将${scope}作品 ${prompt} ${resultIndex >= 0 ? resultIndex + 1 : 1} 设为参考图`;
}

function previewImageAlt(
  file: ResultFile,
  files: ResultFile[],
  prompt: string,
) {
  if (files.length <= 1 && !isReferenceFile(file)) return prompt;
  return `${fileDisplayLabel(file, files)} ${prompt}`;
}

function missingReferenceImage(files: ResultFile[]) {
  return imageFiles(files).length > 0 && !firstReferenceImageFile(files);
}

async function writeImageToClipboard(bytes: ArrayBuffer, mimeType: string) {
  if (!navigator.clipboard?.write || typeof ClipboardItem === "undefined") {
    throw new Error("image clipboard is not supported");
  }
  const imageMimeType = mimeType.startsWith("image/") ? mimeType : "image/png";
  await navigator.clipboard.write([
    new ClipboardItem({
      [imageMimeType]: new Blob([bytes], { type: imageMimeType }),
    }),
  ]);
}

type RunningPhase = NonNullable<RunningFeedTask["phase"]>;

const RUNNING_PHASE_LABELS: Record<
  RunningPhase,
  { title: string; short: string; detail: string }
> = {
  uploading: {
    title: "正在上传参考图",
    short: "上传参考图",
    detail: "正在提交垫图与创作参数",
  },
  submitting: {
    title: "正在提交请求",
    short: "提交请求",
    detail: "正在连接模型供应源",
  },
  generating: {
    title: "正在生成画面",
    short: "生成中",
    detail: "模型正在构图、细化和渲染",
  },
  downloading: {
    title: "正在整理作品",
    short: "整理作品",
    detail: "正在下载预览图并写入作品流",
  },
};
const COMPLETION_FEED_REFRESH_MS = 15000;
const COMPLETION_FEED_REFRESH_INTERVAL_MS = 2000;

function runningPhaseSteps(task: RunningFeedTask) {
  const steps: RunningPhase[] = task.referencePreviewUrl
    ? ["uploading", "submitting", "generating", "downloading"]
    : ["submitting", "generating", "downloading"];
  return steps;
}

function activeRunningPhase(task: RunningFeedTask): RunningPhase {
  return (
    task.phase ??
    (task.id
      ? "generating"
      : task.referencePreviewUrl
        ? "uploading"
        : "submitting")
  );
}

function runningPhaseState(
  step: RunningPhase,
  activePhase: RunningPhase,
  steps: RunningPhase[],
) {
  const stepIndex = steps.indexOf(step);
  const activeIndex = steps.indexOf(activePhase);
  if (stepIndex < activeIndex) return "done";
  if (stepIndex === activeIndex) return "active";
  return "pending";
}

export default function App() {
  return (
    <AppProviders>
      <AppContent shell={<AppShell />} />
    </AppProviders>
  );
}

function AppShell() {
  const auth = useAuth();
  const username = auth.user?.username ?? "";
  return (
    <Shell
      workspace={<WorkspaceHome username={username} />}
      history={<HistoryPage username={username} />}
      adminUsers={<AdminUsersPage username={username} />}
      forbidden={<ForbiddenPage username={username} />}
    />
  );
}

function WorkspaceHome({ username }: { username: string }) {
  const initialPreferencesRef = useRef<
    { value?: ComposerPreferences } | undefined
  >(undefined);
  if (!initialPreferencesRef.current) {
    initialPreferencesRef.current = {
      value: readComposerPreferences(username),
    };
  }
  const initialPreferences = initialPreferencesRef.current.value;
  const [mode, setMode] = useState<Capability>(
    initialPreferences?.capability === "image-to-image"
      ? "image-to-image"
      : "text-to-image",
  );
  const [providers, setProviders] = useState<ImageProvider[]>([]);
  const [providerId, setProviderId] = useState("openai");
  const [channelId, setChannelId] = useState("");
  const [modelId, setModelId] = useState("");
  const [prompt, setPrompt] = useState("");
  const initialSize = SIZE_PRESETS.includes(initialPreferences?.size ?? "")
    ? (initialPreferences?.size ?? "1024x1024")
    : "1024x1024";
  const [size, setSize] = useState(initialSize);
  const [sizeMode, setSizeMode] = useState(initialSize);
  const [aspectRatio, setAspectRatio] = useState(
    initialPreferences?.aspectRatio ?? DEFAULT_GROK_ASPECT_RATIO,
  );
  const [resolution, setResolution] = useState(
    initialPreferences?.resolution ?? DEFAULT_GROK_RESOLUTION,
  );
  const [quality, setQuality] = useState(
    initialPreferences?.quality ?? DEFAULT_GROK_QUALITY,
  );
  const [image, setImage] = useState<File>();
  const [imagePreviewUrl, setImagePreviewUrl] = useState("");
  const [feedItems, setFeedItems] = useState<ArtworkFeedItem[]>([]);
  const [message, setMessage] = useState<string>();
  const [error, setError] = useState<string>();
  const [generationToast, setGenerationToast] = useState<string>();
  const [copiedImageFileId, setCopiedImageFileId] = useState("");
  const [generating, setGenerating] = useState(false);
  const [runningTask, setRunningTask] = useState<RunningFeedTask>();
  const [completionFeedRefreshUntil, setCompletionFeedRefreshUntil] =
    useState(0);
  const [composerReservedSpace, setComposerReservedSpace] = useState(340);
  const [nowTick, setNowTick] = useState(Date.now());
  const [reuseDraft, setReuseDraft] = useState<ReuseHistoryDraft>();
  const [lightbox, setLightbox] = useState<{
    url: string;
    prompt: string;
    objectKey: string;
    fileId: string;
  }>();
  const [pendingFeedDelete, setPendingFeedDelete] = useState<ArtworkFeedItem>();
  const referenceInputRef = useRef<HTMLInputElement>(null);
  const composerRef = useRef<HTMLFormElement>(null);
  const feedBottomRef = useRef<HTMLDivElement>(null);
  const runningTaskRef = useRef<HTMLElement>(null);
  const didInitialFeedScrollRef = useRef(false);
  const generationSequenceRef = useRef(0);

  function applyGeometryFromParameters(
    parameters: Record<string, unknown> | undefined,
    nextModelId: string,
  ) {
    if (isGrokImagineModel(nextModelId)) {
      const nextAspect =
        typeof parameters?.aspectRatio === "string" &&
        isValidGrokAspectRatio(parameters.aspectRatio)
          ? parameters.aspectRatio
          : undefined;
      const nextResolution =
        typeof parameters?.resolution === "string" &&
        isValidGrokResolution(parameters.resolution)
          ? parameters.resolution
          : undefined;
      const nextQuality =
        typeof parameters?.quality === "string" &&
        isValidGrokQuality(parameters.quality)
          ? parameters.quality
          : undefined;
      if (nextAspect) setAspectRatio(nextAspect);
      if (nextResolution) setResolution(nextResolution);
      if (nextQuality) setQuality(nextQuality);
      return;
    }
    const storedSize =
      typeof parameters?.size === "string" ? parameters.size : undefined;
    if (!storedSize) return;
    const nextSize = SIZE_PRESETS.includes(storedSize)
      ? storedSize
      : "1024x1024";
    setSize(nextSize);
    setSizeMode(nextSize);
  }

  function applyGeometryFromDraft(draft: ReuseHistoryDraft) {
    applyGeometryFromParameters(
      {
        size: draft.size,
        aspectRatio: draft.aspectRatio,
        resolution: draft.resolution,
        quality: draft.quality,
      },
      draft.modelId,
    );
  }

  async function refreshRecentWorkspaceFeed() {
    const items = await recentHistoryFeedItems();
    setFeedItems((current) => mergeFeedItems([...items, ...current]));
    return items;
  }

  useEffect(() => {
    const draft = readReuseHistoryDraft();
    if (!draft) return;
    setReuseDraft(draft);
    setMode(
      draft.capability === "image-to-image" ? "image-to-image" : "text-to-image",
    );
    setPrompt(draft.prompt);
    applyGeometryFromDraft(draft);
    if (draft.notice) setMessage(draft.notice);
    if (draft.referenceFileId) {
      void loadReferenceImage(
        {
          id: draft.referenceFileId,
          objectKey: draft.referenceObjectKey ?? "reference.png",
          mimeType: draft.referenceMimeType ?? "image/png",
        },
        draft.notice ?? "已将作品设为参考图",
      );
    }
  }, []);

  useEffect(() => {
    let active = true;
    async function loadProviders() {
      const payload = await listProviders();
      if (!active) return;
      if (!payload.ok) {
        setProviders([]);
        setError(payload.error?.message ?? "生图通道准备失败，请稍后重试");
        return;
      }
      const nextProviders = Array.isArray(payload.data.providers)
        ? payload.data.providers
        : [];
      setProviders(nextProviders);
      setError(undefined);
      const preferredProvider =
        nextProviders.find((item) => item.id === "openai" && item.available) ??
        nextProviders.find((item) => item.available) ??
        nextProviders.find((item) => item.id === "openai") ??
        nextProviders[0];
      const nextProvider =
        reuseDraft?.channelName?.toLowerCase().includes("grok")
          ? nextProviders.find((item) => item.id === "grok" && item.available) ??
            nextProviders.find((item) => item.id === "grok") ??
            preferredProvider
          : preferredProvider;
      if (!nextProvider) return;
      setProviderId(nextProvider.id);
      setChannelId(nextProvider.id);
      const requestedMode =
        reuseDraft?.capability === "image-to-image" ? "image-to-image" : mode;
      const preferredModel = nextProvider.models.find(
        (item) => item.id === reuseDraft?.modelId,
      );
      const defaultModel = nextProvider.models.find(
        (item) => item.id === nextProvider.defaultModelId,
      );
      const firstModel =
        preferredModel ??
        defaultModel ??
        nextProvider.models.find((item) =>
          item.capabilities.includes(requestedMode),
        ) ??
        nextProvider.models[0];
      if (firstModel) setModelId(firstModel.id);
    }
    void loadProviders().catch(() => {
      if (active) {
        setProviders([]);
        setError("生图通道准备失败，请稍后重试");
      }
    });
    return () => {
      active = false;
    };
  }, [mode, reuseDraft]);

  useEffect(() => {
    let active = true;
    async function loadRecentHistory() {
      try {
        const items = await recentHistoryFeedItems();
        if (active) {
          setFeedItems((current) => mergeFeedItems([...items, ...current]));
        }
      } catch {
        // Recent history is a convenience feed; composer should remain usable if it cannot load.
      }
    }
    void loadRecentHistory();
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    let active = true;
    async function loadRunningTasks() {
      try {
        const payload = await listTasks();
        if (!payload.ok || payload.data.tasks.length === 0) return;
        if (active) {
          setRunningTask(taskToRunningFeedTask(payload.data.tasks[0]));
          setNowTick(Date.now());
        }
      } catch {
        // Active tasks are a recovery aid; creation should remain usable if they cannot load.
      }
    }
    void loadRunningTasks();
    return () => {
      active = false;
    };
  }, []);

  const availableProviders = providers.filter((provider) => provider.available);
  const selectedProvider =
    providers.find((provider) => provider.id === providerId && provider.available) ??
    providers.find((provider) => provider.available) ??
    providers.find((provider) => provider.id === providerId) ??
    providers[0];
  const selectedChannel: Channel | undefined = selectedProvider
    ? {
        id: selectedProvider.id,
        name: selectedProvider.name,
        baseUrl: "",
        hasApiKey: true,
        models: selectedProvider.models,
      }
    : undefined;
  const modeChannels = selectedChannel ? [selectedChannel] : [];
  const modeModels =
    selectedChannel?.models.filter((model) =>
      model.capabilities.includes(mode),
    ) ?? [];
  const selectedModel = selectedChannel?.models.find(
    (model) => model.id === modelId,
  );
  const referencePromotesToImageEdit =
    mode === "text-to-image" &&
    Boolean(image) &&
    Boolean(selectedModel?.capabilities.includes("image-to-image"));
  const imageEditWithoutReferenceFallsBackToText =
    mode === "image-to-image" &&
    !image &&
    Boolean(selectedModel?.capabilities.includes("text-to-image"));
  const composerSubmitMode: Capability = referencePromotesToImageEdit
    ? "image-to-image"
    : imageEditWithoutReferenceFallsBackToText
      ? "text-to-image"
      : mode;
  const grokImagine = isGrokImagineModel(modelId);

  useEffect(() => {
    if (!grokImagine || composerSubmitMode !== "image-to-image") return;
    setAspectRatio((current) =>
      current === DEFAULT_GROK_EDIT_ASPECT_RATIO
        ? current
        : DEFAULT_GROK_EDIT_ASPECT_RATIO,
    );
  }, [grokImagine, composerSubmitMode]);

  useEffect(() => {
    if (selectedProvider && selectedProvider.id !== providerId) {
      setProviderId(selectedProvider.id);
      setChannelId(selectedProvider.id);
    }
    const providerDefaultModel = selectedProvider?.models.find(
      (model) =>
        model.id === selectedProvider.defaultModelId &&
        model.capabilities.includes(mode),
    );
    const preferredModel =
      providerDefaultModel ?? selectModelForCapability(modeModels, modelId, mode);
    if (preferredModel && preferredModel.id !== modelId) {
      setModelId(preferredModel.id);
    }
  }, [channelId, mode, modeModels, modelId, selectedProvider, providerId]);

  useEffect(() => {
    if (!selectedChannel?.id || !modelId) return;
    saveComposerPreferences(username, {
      channelId: selectedProvider.id,
      modelId,
      capability: mode,
      size,
      ...(isGrokImagineModel(modelId)
        ? { aspectRatio, resolution, quality }
        : {}),
    });
  }, [mode, modelId, selectedProvider?.id, size, aspectRatio, resolution, quality, username]);

  useEffect(() => {
    if (!image) {
      setImagePreviewUrl("");
      return;
    }
    const url = URL.createObjectURL(image);
    setImagePreviewUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [image]);

  useEffect(() => {
    if (!runningTask) return;
    const timer = window.setInterval(() => setNowTick(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [runningTask]);

  useEffect(() => {
    function updateComposerSpace() {
      const height = composerRef.current?.getBoundingClientRect().height ?? 0;
      if (height > 0) {
        setComposerReservedSpace(Math.ceil(height + 120));
      }
    }

    updateComposerSpace();
    window.addEventListener("resize", updateComposerSpace);
    let observer: ResizeObserver | undefined;
    if (typeof ResizeObserver !== "undefined" && composerRef.current) {
      observer = new ResizeObserver(updateComposerSpace);
      observer.observe(composerRef.current);
    }
    return () => {
      window.removeEventListener("resize", updateComposerSpace);
      observer?.disconnect();
    };
  }, []);

  useEffect(() => {
    if (!runningTask) return;
    const scrollIntoView = runningTaskRef.current?.scrollIntoView;
    if (typeof scrollIntoView === "function") {
      scrollIntoView.call(runningTaskRef.current, {
        behavior: "smooth",
        block: "end",
      });
    }
  }, [runningTask?.id, runningTask?.prompt]);

  useEffect(() => {
    if (
      didInitialFeedScrollRef.current ||
      (feedItems.length === 0 && !runningTask)
    )
      return;
    didInitialFeedScrollRef.current = true;
    function scrollFeedBottomIntoView() {
      const scrollIntoView = feedBottomRef.current?.scrollIntoView;
      if (typeof scrollIntoView === "function") {
        scrollIntoView.call(feedBottomRef.current, {
          behavior: "auto",
          block: "end",
        });
      }
    }
    const scheduleFrame =
      typeof window.requestAnimationFrame === "function"
        ? window.requestAnimationFrame.bind(window)
        : (callback: FrameRequestCallback) =>
            window.setTimeout(() => callback(Date.now()), 0);
    scheduleFrame(scrollFeedBottomIntoView);
    const settleTimers = [250, 900].map((delay) =>
      window.setTimeout(scrollFeedBottomIntoView, delay),
    );
    return () => {
      settleTimers.forEach((timer) => window.clearTimeout(timer));
    };
  }, [feedItems.length, runningTask]);

  useEffect(() => {
    if (!runningTask) return;
    const runningTaskID = runningTask.id;
    let active = true;
    async function refreshRunningTasks() {
      try {
        const payload = await listTasks();
        if (!active || !payload.ok) return;
        if (payload.data.tasks.length > 0) {
          const serverTask = taskToRunningFeedTask(payload.data.tasks[0]);
          setRunningTask((current) => ({
            ...serverTask,
            phase: "generating",
            referencePreviewUrl: current?.referencePreviewUrl,
            referenceName: current?.referenceName,
          }));
          return;
        }
        if (runningTaskID) {
          const finalPayload = await getTask(runningTaskID);
          if (
            finalPayload.ok &&
            (finalPayload.data.task.status === "failed" ||
              finalPayload.data.task.status === "timeout")
          ) {
            showGenerationFailure(
              finalPayload.data.task.failureReason ?? "生成任务失败",
            );
          }
        }
        setRunningTask(undefined);
        if (active) {
          await refreshRecentWorkspaceFeed();
          setCompletionFeedRefreshUntil(
            Date.now() + COMPLETION_FEED_REFRESH_MS,
          );
        }
      } catch {
        // Polling should not interrupt the composer if a transient request fails.
      }
    }
    const timer = window.setInterval(() => void refreshRunningTasks(), 3000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [runningTask]);

  useEffect(() => {
    if (!completionFeedRefreshUntil) return;
    let active = true;
    async function refreshUntilHistorySettles() {
      if (Date.now() > completionFeedRefreshUntil) {
        if (active) setCompletionFeedRefreshUntil(0);
        return;
      }
      try {
        await refreshRecentWorkspaceFeed();
      } catch {
        // A short-lived completion refresh should not interrupt the composer.
      }
    }
    void refreshUntilHistorySettles();
    const timer = window.setInterval(
      () => void refreshUntilHistorySettles(),
      COMPLETION_FEED_REFRESH_INTERVAL_MS,
    );
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [completionFeedRefreshUntil]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(undefined);
    setMessage(undefined);
    setGenerationToast(undefined);
    if (!selectedProvider?.id || !selectedProvider.available || !modelId) {
      setError("正在准备生图通道，请稍后重试");
      return;
    }
    if (!prompt.trim()) {
      setError("请输入提示词");
      return;
    }
    const effectiveMode: Capability =
      mode === "text-to-image" &&
      image &&
      selectedModel?.capabilities.includes("image-to-image")
        ? "image-to-image"
        : mode === "image-to-image" &&
            !image &&
            selectedModel?.capabilities.includes("text-to-image")
          ? "text-to-image"
          : mode;
    if (
      effectiveMode === "image-to-image" && !image
    ) {
      setError("请上传参考图");
      return;
    }
    const grokSubmit = isGrokImagineModel(modelId);
    const baseSize = size.trim();
    if (
      !grokSubmit &&
      (effectiveMode === "text-to-image" ||
        effectiveMode === "image-to-image") &&
      !isValidImageSize(baseSize)
    ) {
      setError("请输入有效尺寸，例如 1024x1024");
      return;
    }
    if (
      grokSubmit &&
      (effectiveMode === "text-to-image" ||
        effectiveMode === "image-to-image")
    ) {
      if (!isValidGrokAspectRatio(aspectRatio)) {
        setError("请选择有效的图片比例");
        return;
      }
      if (!isValidGrokResolution(resolution)) {
        setError("请选择 1k 或 2k 清晰度");
        return;
      }
      if (
        effectiveMode === "text-to-image" &&
        !isValidGrokQuality(quality)
      ) {
        setError("请选择 low 或 medium 质量");
        return;
      }
    }
    const requestedSize = baseSize;
    const imageParameters = grokSubmit
      ? {
          aspectRatio,
          resolution,
          ...(effectiveMode === "text-to-image" ? { quality } : {}),
        }
      : { size: requestedSize };
    const startedAt = Date.now();
    const taskDraft: RunningFeedTask = {
      prompt,
      channelName: selectedProvider.name,
      modelId,
      capability: effectiveMode,
      phase: effectiveMode === "text-to-image" ? "submitting" : "uploading",
      startedAt,
      referencePreviewUrl: imagePreviewUrl || undefined,
      referenceName: image?.name,
    };
    const generationSequence = generationSequenceRef.current + 1;
    generationSequenceRef.current = generationSequence;
    setNowTick(startedAt);
    setRunningTask(taskDraft);
    setGenerating(true);
    let keepRunningTask = false;
    try {
      const generationRequest =
        effectiveMode === "text-to-image"
          ? generateImage({
              providerId: selectedProvider.id,
              channelId: selectedProvider.id,
              modelId,
              prompt,
              ...imageParameters,
            })
          : effectiveMode === "image-to-image"
            ? submitImageToImage({
                providerId: selectedProvider.id,
                channelId: selectedProvider.id,
                modelId,
                prompt,
                ...imageParameters,
                image: image as File,
              })
            : Promise.reject(new Error("unsupported generation mode"));
      void hydrateSubmittedRunningTask(generationSequence);
      const payload = await generationRequest;
      if (generationSequenceRef.current !== generationSequence) return;
      if (!payload.ok) {
        showGenerationFailure(payload.error?.message ?? "生成失败");
        return;
      }
      if (payload.data.status === "running") {
        keepRunningTask = true;
        setRunningTask({
          ...taskDraft,
          id: payload.data.taskId,
          phase: "generating",
          startedAt,
        });
        setNowTick(Date.now());
        return;
      }
      const files = payload.data.files ?? [];
      setRunningTask((current) =>
        current ? { ...current, phase: "downloading" } : current,
      );
      const previews = await filesWithPreviews(files);
      const nextItem: ArtworkFeedItem = {
        id: payload.data.historyId || payload.data.taskId,
        status: payload.data.status,
        prompt,
        channelName: selectedProvider.name,
        modelId,
        capability: effectiveMode,
        parameters: imageParameters,
        resultText: payload.data.resultText,
        durationMs: payload.data.durationMs,
        files: previews,
        createdAt: Date.now(),
      };
      setFeedItems((current) => mergeFeedItems([nextItem, ...current]));
    } catch {
      if (generationSequenceRef.current === generationSequence) {
        showGenerationFailure("生成请求失败，请检查网络或后端服务后重试");
      }
    } finally {
      if (generationSequenceRef.current === generationSequence) {
        setGenerating(false);
        if (!keepRunningTask) {
          setRunningTask(undefined);
        }
      }
    }
  }

  function showGenerationFailure(message: string) {
    setError(message);
    setGenerationToast(message);
  }

  async function hydrateSubmittedRunningTask(generationSequence: number) {
    try {
      const payload = await listTasks();
      if (
        generationSequenceRef.current !== generationSequence ||
        !payload.ok ||
        payload.data.tasks.length === 0
      )
        return;
      const serverTask = taskToRunningFeedTask(payload.data.tasks[0]);
      setRunningTask((current) => ({
        ...serverTask,
        phase: "generating",
        referencePreviewUrl: current?.referencePreviewUrl,
        referenceName: current?.referenceName,
      }));
      setNowTick(Date.now());
    } catch {
      // The optimistic running card remains usable even if this fast hydration misses.
    }
  }

  function previewFeedFile(item: ArtworkFeedItem, file: PreviewFile) {
    if (!file.previewUrl) return;
    setLightbox({
      url: file.previewUrl,
      prompt: item.prompt,
      objectKey: file.objectKey,
      fileId: file.id,
    });
  }

  function requestFeedItemRemoval(item: ArtworkFeedItem) {
    setError(undefined);
    setMessage(undefined);
    setPendingFeedDelete(item);
  }

  async function removeFeedItem(item: ArtworkFeedItem) {
    setError(undefined);
    setMessage(undefined);
    setPendingFeedDelete(undefined);
    const payload = await deleteHistoryRecord(item.id);
    if (!payload.ok) {
      setError(payload.error?.message ?? "删除失败");
      return;
    }
    const deletedFileIds = new Set(item.files.map((file) => file.id));
    setFeedItems((current) =>
      current.filter((feedItem) => feedItem.id !== item.id),
    );
    setLightbox((current) =>
      current && deletedFileIds.has(current.fileId) ? undefined : current,
    );
    setMessage("作品已删除");
  }

  async function refreshFeedPreviewUrl(itemId: string, fileId: string) {
    const payload = await getSignedFileURL(fileId);
    if (!payload.ok) return;
    setFeedItems((current) =>
      current.map((item) =>
        item.id === itemId
          ? {
              ...item,
              files: item.files.map((file) =>
                file.id === fileId
                  ? { ...file, previewUrl: payload.data.url }
                  : file,
              ),
            }
          : item,
      ),
    );
  }

  async function copyFeedFileImage(file: PreviewFile) {
    setError(undefined);
    setMessage(undefined);
    setCopiedImageFileId("");
    const payload = await getFileContent(file.id);
    if (!payload.ok) {
      setError(payload.error?.message ?? "图片读取失败");
      return;
    }
    try {
      const mimeType = payload.data.mimeType.startsWith("image/")
        ? payload.data.mimeType
        : file.mimeType;
      await writeImageToClipboard(payload.data.bytes, mimeType);
      setCopiedImageFileId(file.id);
      setMessage("图片已复制");
    } catch {
      setError("复制图片失败，请使用下载图片或打开原图后复制");
    }
  }

  function handleFeedImageKeyDown(
    event: KeyboardEvent<HTMLButtonElement>,
    file: PreviewFile,
  ) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "c") {
      event.preventDefault();
      void copyFeedFileImage(file);
    }
  }

  async function loadReferenceImage(
    file: Pick<ResultFile, "id" | "objectKey" | "mimeType">,
    successMessage: string,
  ) {
    setError(undefined);
    const payload = await getFileContent(file.id);
    if (!payload.ok) {
      setError(payload.error?.message ?? "参考图读取失败");
      return false;
    }
    const mimeType = payload.data.mimeType || file.mimeType || "image/png";
    const referenceFile = new File(
      [payload.data.bytes],
      filenameFromObjectKey(file.objectKey),
      { type: mimeType },
    );
    setImage(referenceFile);
    if (referenceInputRef.current) {
      referenceInputRef.current.value = "";
    }
    setMessage(successMessage);
    return true;
  }

  function isReferenceImageFile(file: File) {
    return (
      file.type.startsWith("image/") || /\.(png|jpe?g|webp)$/i.test(file.name)
    );
  }

  function imageFileFromClipboard(event: ClipboardEvent<HTMLElement>) {
    const pastedFile = Array.from(event.clipboardData.files).find(
      isReferenceImageFile,
    );
    if (pastedFile) return pastedFile;
    return Array.from(event.clipboardData.items)
      .find((item) => item.kind === "file" && item.type.startsWith("image/"))
      ?.getAsFile();
  }

  function applyReferenceImageFile(file: File, successMessage: string) {
    setError(undefined);
    setImage(file);
    if (mode === "text-to-image") {
      setMode("image-to-image");
    }
    if (referenceInputRef.current) {
      referenceInputRef.current.value = "";
    }
    setMessage(successMessage);
  }

  function handleComposerPaste(event: ClipboardEvent<HTMLElement>) {
    const pastedImage = imageFileFromClipboard(event);
    if (!pastedImage) return;
    event.preventDefault();
    applyReferenceImageFile(pastedImage, "已从剪贴板添加参考图。");
  }

  function handlePromptKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (
      event.key !== "Enter" ||
      event.shiftKey ||
      event.ctrlKey ||
      event.metaKey ||
      event.altKey ||
      event.nativeEvent.isComposing
    )
      return;
    event.preventDefault();
    event.currentTarget.form?.requestSubmit();
  }

  async function useFeedFileAsReference(
    item: ArtworkFeedItem,
    file: PreviewFile,
  ) {
    setError(undefined);
    setMode("image-to-image");
    setPrompt(item.prompt);
    applyGeometryFromParameters(item.parameters, item.modelId);
    const matchingProvider =
      providers.find((provider) => provider.name === item.channelName) ??
      providers.find((provider) => provider.available);
    if (matchingProvider) {
      setProviderId(matchingProvider.id);
      setChannelId(matchingProvider.id);
      const matchingModel = selectModelForCapability(
        matchingProvider.models,
        item.modelId,
        "image-to-image",
      );
      if (matchingModel) setModelId(matchingModel.id);
    }
    await loadReferenceImage(file, "已将作品设为参考图，可继续图生图。");
  }

  async function cancelRunningTask(task: RunningFeedTask) {
    if (!task.id) return;
    setError(undefined);
    setMessage(undefined);
    const payload = await cancelTask(task.id);
    if (!payload.ok) {
      setError(payload.error?.message ?? "取消任务失败");
      return;
    }
    generationSequenceRef.current += 1;
    setGenerating(false);
    setRunningTask(undefined);
    if (payload.data.record && payload.data.record.status !== "失败") {
      const item = await historyRecordToFeedItem(payload.data.record);
      setFeedItems((current) => mergeFeedItems([item, ...current]));
    }
    setMessage("任务已取消");
  }

  function applyFeedItemToComposer(item: ArtworkFeedItem, notice?: string) {
    setError(undefined);
    setMessage(notice);
    setMode(
      item.capability === "image-to-image" ? "image-to-image" : "text-to-image",
    );
    setPrompt(item.prompt);
    if (item.capability === "text-to-image" || notice) clearReferenceImage();
    applyGeometryFromParameters(item.parameters, item.modelId);
    const matchingProvider = providers.find(
      (provider) =>
        provider.name === item.channelName &&
        provider.models.some((model) => model.id === item.modelId),
    );
    if (matchingProvider) {
      setProviderId(matchingProvider.id);
      setChannelId(matchingProvider.id);
      setModelId(item.modelId);
    }
  }

  async function reuseFeedItem(item: ArtworkFeedItem) {
    applyFeedItemToComposer(item);
    if (item.capability === "image-to-image") {
      const file = firstImageFile(item.files);
      if (file) {
        await loadReferenceImage(file, "已将作品设为参考图");
      } else {
        clearReferenceImage();
      }
    }
  }

  async function regenerateFeedItem(item: ArtworkFeedItem) {
    if (item.capability === "image-to-image") {
      const file = firstImageFile(item.files);
      const notice = file
        ? "已将上一张作品设为参考图，可继续调整后再次生成。"
        : "图生图记录没有可用结果图，需要重新上传参考图后再生成。";
      applyFeedItemToComposer(item, notice);
      if (file) {
        await loadReferenceImage(file, notice);
      }
      return;
    }
    const notice =
      String(item.capability) === "image-to-text"
        ? "图生文记录需要重新上传参考图后再生成。"
        : "已从作品带入创作参数，可继续调整后再次生成。";
    applyFeedItemToComposer(item, notice);
  }

  function clearReferenceImage() {
    setImage(undefined);
    if (referenceInputRef.current) {
      referenceInputRef.current.value = "";
    }
  }

  function selectMode(nextMode: Capability) {
    setMode(nextMode);
    if (nextMode === "text-to-image") {
      clearReferenceImage();
    }
  }

  const studioStyle = {
    "--composer-reserved-space": `${composerReservedSpace}px`,
  } as CSSProperties & Record<"--composer-reserved-space", string>;

  return (
    <div className="studio-page" style={studioStyle}>
      {generationToast ? (
        <div
          className="toast toast-error"
          role="status"
          aria-label="生成失败提示"
        >
          <div>
            <strong>生成失败</strong>
            <span>原因：{generationToast}</span>
          </div>
          <button
            type="button"
            onClick={() => setGenerationToast(undefined)}
            aria-label="关闭生成失败提示"
          >
            <X size={16} aria-hidden="true" />
          </button>
        </div>
      ) : null}
      <section className="workspace-feed">
        <PageHeader
          eyebrow="IMAGE FORGE"
          title="创作工作台"
          username={username}
        />
        <div className="status-strip">
          <span>{availableProviders.length} 个生图通道</span>
          <span>{generating || runningTask ? "1 个任务运行中" : "待命"}</span>
        </div>
        {error ? <div className="error page-message">{error}</div> : null}
        {message ? <div className="success page-message">{message}</div> : null}
        {generating ? (
          <div className="notice page-message">
            已提交到模型，正在等待图片返回。
          </div>
        ) : null}
        {pendingFeedDelete ? (
          <section className="workspace-confirm-bar" aria-label="确认删除作品">
            <div>
              <strong>确认删除作品？</strong>
              <p>{pendingFeedDelete.prompt}</p>
            </div>
            <div>
              <button
                type="button"
                onClick={() => setPendingFeedDelete(undefined)}
              >
                取消
              </button>
              <button
                className="danger-action"
                type="button"
                onClick={() => void removeFeedItem(pendingFeedDelete)}
              >
                确认删除作品
              </button>
            </div>
          </section>
        ) : null}

        <div className="artwork-feed">
          {feedItems.length === 0 && !runningTask ? (
            <article className="empty-stage-card">
              <strong>等待第一束灵感</strong>
              <p>选择生图引擎，输入提示词后开始生成。</p>
            </article>
          ) : null}
          {feedItems.map((item) => {
            const displayFiles = orderedDisplayFiles(item.files);
            const shouldShowMissingReference =
              item.capability === "image-to-image" &&
              missingReferenceImage(item.files);
            return (
              <article className="artwork-card" key={item.id}>
                <div className="artwork-card-header">
                  <div>
                    <strong>{item.prompt}</strong>
                    <p>
                      {item.channelName} / {item.modelId}
                    </p>
                  </div>
                  <div className="artwork-card-tools">
                    <span
                      className={
                        item.status === "失败" ? "danger-pill" : undefined
                      }
                    >
                      {item.status}
                    </span>
                    <span>{capabilityLabel(item.capability)}</span>
                    {typeof item.durationMs === "number" ? (
                      <span>耗时 {formatElapsed(item.durationMs)}</span>
                    ) : null}
                    <button
                      type="button"
                      aria-label={`复用作品 ${item.prompt}`}
                      onClick={() => void reuseFeedItem(item)}
                    >
                      复用
                    </button>
                    <button
                      type="button"
                      aria-label={`再次生成作品 ${item.prompt}`}
                      onClick={() => void regenerateFeedItem(item)}
                    >
                      再次生成
                    </button>
                    <button
                      className="danger-action"
                      type="button"
                      aria-label={`删除作品 ${item.prompt}`}
                      onClick={() => requestFeedItemRemoval(item)}
                    >
                      删除
                    </button>
                  </div>
                </div>
                {item.failureReason ? (
                  <div className="failure-result">{item.failureReason}</div>
                ) : null}
                {item.resultText ? (
                  <div className="text-result">{item.resultText}</div>
                ) : null}
                {shouldShowMissingReference ? (
                  <div className="missing-reference-note">
                    <strong>缺少原始垫图</strong>
                    <span>这条图生图记录没有保存原图，只能看到生成图。</span>
                  </div>
                ) : null}
                {displayFiles.length > 0 ? (
                  <div className="result-gallery artwork-preview-row">
                    {displayFiles.map((file) => (
                      <div
                        className="file-preview artwork-preview hero"
                        key={file.id}
                      >
                        {file.previewUrl ? (
                          <button
                            className="preview-thumb-button"
                            type="button"
                            aria-label={`打开作品预览 ${item.prompt}`}
                            onClick={() => previewFeedFile(item, file)}
                            onKeyDown={(event) =>
                              handleFeedImageKeyDown(event, file)
                            }
                          >
                            <img
                              alt={previewImageAlt(
                                file,
                                item.files,
                                item.prompt,
                              )}
                              src={file.previewUrl}
                              onError={() =>
                                void refreshFeedPreviewUrl(item.id, file.id)
                              }
                            />
                          </button>
                        ) : null}
                        <strong className="file-role-label">
                          {fileDisplayLabel(file, item.files)}
                        </strong>
                        <button
                          type="button"
                          aria-label={`${copiedImageFileId === file.id ? "已复制" : "复制图片"} ${item.prompt}`}
                          onClick={() => void copyFeedFileImage(file)}
                        >
                          {copiedImageFileId === file.id
                            ? "已复制"
                            : "复制图片"}
                        </button>
                        <button
                          type="button"
                          aria-label={referenceActionLabel(
                            file,
                            item.files,
                            item.prompt,
                          )}
                          onClick={() =>
                            void useFeedFileAsReference(item, file)
                          }
                        >
                          设为参考图
                        </button>
                      </div>
                    ))}
                  </div>
                ) : null}
              </article>
            );
          })}
          {runningTask ? (
            <article
              ref={runningTaskRef}
              className="running-card"
              aria-label={`运行中任务 ${runningTask.prompt}`}
            >
              {(() => {
                const phase = activeRunningPhase(runningTask);
                const phaseLabel = RUNNING_PHASE_LABELS[phase];
                const phaseSteps = runningPhaseSteps(runningTask);
                return (
                  <>
                    <div className="running-card-main">
                      {runningTask.referencePreviewUrl ? (
                        <div className="running-reference-frame">
                          <img
                            className="running-reference-preview"
                            alt={`运行中参考图 ${runningTask.referenceName ?? "参考图"}`}
                            src={runningTask.referencePreviewUrl}
                          />
                          <span>上传参考图</span>
                        </div>
                      ) : null}
                      <div className="running-copy">
                        <div className="running-title-row">
                          <span className="running-spinner" aria-hidden="true">
                            <LoaderCircle size={18} />
                          </span>
                          <div>
                            <span className="running-status-title">
                              {phaseLabel.title}
                            </span>
                            <strong>{runningTask.prompt}</strong>
                          </div>
                        </div>
                        <p>
                          {runningTask.channelName} / {runningTask.modelId}
                        </p>
                        <div className="running-progress" aria-hidden="true">
                          <span />
                        </div>
                        <span className="running-phase-detail">
                          {phaseLabel.detail}
                        </span>
                      </div>
                    </div>
                    <div className="running-card-side">
                      <div className="running-card-meta">
                        <span>{capabilityLabel(runningTask.capability)}</span>
                        <span>
                          已用时{" "}
                          {formatElapsed(nowTick - runningTask.startedAt)}
                        </span>
                        {runningTask.id ? (
                          <button
                            type="button"
                            aria-label={`取消任务 ${runningTask.prompt}`}
                            onClick={() => void cancelRunningTask(runningTask)}
                          >
                            取消
                          </button>
                        ) : null}
                      </div>
                      <ol className="running-phase-list" aria-label="生成状态">
                        {phaseSteps.map((step) => (
                          <li
                            className={`running-phase-step ${runningPhaseState(step, phase, phaseSteps)}`}
                            key={step}
                          >
                            <span aria-hidden="true" />
                            <b>{RUNNING_PHASE_LABELS[step].short}</b>
                            <em>{RUNNING_PHASE_LABELS[step].title}</em>
                          </li>
                        ))}
                      </ol>
                    </div>
                  </>
                );
              })()}
            </article>
          ) : null}
          <div
            ref={feedBottomRef}
            className="feed-bottom-anchor"
            aria-hidden="true"
          />
        </div>
        {lightbox ? (
          <div
            className="image-lightbox"
            role="dialog"
            aria-label="图片预览"
            aria-modal="true"
          >
            <button
              className="image-lightbox-backdrop"
              type="button"
              aria-label="关闭图片预览背景"
              onClick={() => setLightbox(undefined)}
            />
            <section className="image-lightbox-panel">
              <header>
                <div>
                  <span className="eyebrow">PREVIEW</span>
                  <h2>{lightbox.prompt}</h2>
                </div>
                <div className="image-lightbox-actions">
                  <a href={lightbox.url} target="_blank" rel="noreferrer">
                    打开原图
                  </a>
                  <button type="button" onClick={() => setLightbox(undefined)}>
                    关闭预览
                  </button>
                </div>
              </header>
              <div className="image-lightbox-stage">
                <img alt={lightbox.prompt} src={lightbox.url} />
              </div>
              <p>{lightbox.objectKey}</p>
            </section>
          </div>
        ) : null}
      </section>

      <form
        ref={composerRef}
        aria-label="创作输入栏"
        className="composer-shell"
        onPaste={handleComposerPaste}
        onSubmit={submit}
      >
        <div className="prompt-console">
          <label className="reference-card">
            {imagePreviewUrl ? (
              <img alt="参考图预览" src={imagePreviewUrl} />
            ) : (
              <ImagePlus size={28} aria-hidden="true" />
            )}
            <span>{image?.name ?? "参考图"}</span>
            {image ? (
              <button
                aria-label="清除参考图"
                className="reference-clear"
                type="button"
                onClick={(event) => {
                  event.preventDefault();
                  clearReferenceImage();
                }}
              >
                <X size={14} aria-hidden="true" />
              </button>
            ) : null}
            <input
              aria-label="参考图"
              accept="image/png,image/jpeg,image/webp"
              ref={referenceInputRef}
              type="file"
              onChange={(event) => setImage(event.target.files?.[0])}
            />
          </label>
          <textarea
            aria-label="提示词"
            className="prompt-input"
            placeholder={
              composerSubmitMode === "text-to-image"
                ? "描述画面中的物体、风格、构图和文字排版。"
                : "描述如何基于参考图改写画面。"
            }
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
            onKeyDown={handlePromptKeyDown}
          />
          <button
            aria-label={
              generating
                ? "生成中..."
                : composerSubmitMode === "text-to-image"
                  ? "开始文生图"
                  : "开始图生图"
            }
            className="send-orb"
            title={generating ? "生成中..." : "开始生成"}
            type="submit"
            disabled={generating}
          >
            <Send size={24} aria-hidden="true" />
          </button>
        </div>

        <div className="dock-toolbar">
          <label className="dock-control">
            <Zap size={16} aria-hidden="true" />
            <span>引擎</span>
            <select
              value={selectedProvider?.id ?? ""}
              onChange={(event) => {
                setProviderId(event.target.value);
                setChannelId(event.target.value);
              }}
            >
              {providers.map((provider) => (
                <option
                  key={provider.id}
                  value={provider.id}
                  disabled={!provider.available}
                >
                  {provider.name}{provider.available ? "" : "（暂不可用）"}
                </option>
              ))}
            </select>
          </label>
          <label className="dock-control">
            <Layers size={16} aria-hidden="true" />
            <span>模型</span>
            <select
              value={modelId}
              onChange={(event) => setModelId(event.target.value)}
            >
              {modeModels.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.id}
                </option>
              ))}
            </select>
          </label>
          <label className="dock-control">
            <SlidersHorizontal size={16} aria-hidden="true" />
            <span>模式</span>
            <select
              value={mode}
              aria-label="模式"
              onChange={(event) => selectMode(event.target.value as Capability)}
            >
              <option value="text-to-image">文生图</option>
              <option value="image-to-image">图生图</option>
            </select>
          </label>
          {composerSubmitMode === "text-to-image" ||
          composerSubmitMode === "image-to-image" ? (
            grokImagine ? (
              <>
                <label className="dock-control">
                  <Maximize2 size={16} aria-hidden="true" />
                  <span>比例</span>
                  <select
                    aria-label="比例"
                    value={aspectRatio}
                    onChange={(event) => setAspectRatio(event.target.value)}
                  >
                    {GROK_ASPECT_RATIOS.map((preset) => (
                      <option key={preset} value={preset}>
                        {preset}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="dock-control">
                  <Maximize2 size={16} aria-hidden="true" />
                  <span>清晰度</span>
                  <select
                    aria-label="清晰度"
                    value={resolution}
                    onChange={(event) => setResolution(event.target.value)}
                  >
                    {GROK_RESOLUTIONS.map((preset) => (
                      <option key={preset} value={preset}>
                        {preset}
                      </option>
                    ))}
                  </select>
                </label>
                {composerSubmitMode === "text-to-image" ? (
                  <label className="dock-control">
                    <Maximize2 size={16} aria-hidden="true" />
                    <span>质量</span>
                    <select
                      aria-label="质量"
                      value={quality}
                      onChange={(event) => setQuality(event.target.value)}
                    >
                      {GROK_QUALITY_PRESETS.map((preset) => (
                        <option key={preset} value={preset}>
                          {preset}
                        </option>
                      ))}
                    </select>
                  </label>
                ) : null}
              </>
            ) : (
              <label className="dock-control">
                <Maximize2 size={16} aria-hidden="true" />
                <span>尺寸</span>
                <select
                  aria-label="尺寸"
                  value={sizeMode}
                  onChange={(event) => {
                    const nextSizeMode = event.target.value;
                    setSizeMode(nextSizeMode);
                    if (nextSizeMode !== "custom") setSize(nextSizeMode);
                  }}
                >
                  {SIZE_PRESETS.map((preset) => (
                    <option key={preset} value={preset}>
                      {preset}
                    </option>
                  ))}
                </select>
              </label>
            )
          ) : null}
        </div>
      </form>
    </div>
  );
}

function HistoryPage({ username }: { username: string }) {
  const [records, setRecords] = useState<HistoryRecord[]>([]);
  const [previewUrls, setPreviewUrls] = useState<Record<string, string>>({});
  const [downloadUrls, setDownloadUrls] = useState<Record<string, string>>({});
  const [selectedRecordIds, setSelectedRecordIds] = useState<string[]>([]);
  const [channelFilter, setChannelFilter] = useState("");
  const [modelFilter, setModelFilter] = useState("");
  const [capabilityFilter, setCapabilityFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [message, setMessage] = useState<string>();
  const [error, setError] = useState<string>();
  const [copiedImageFileId, setCopiedImageFileId] = useState("");
  const [manualCopyText, setManualCopyText] = useState("");
  const [pendingBulkDelete, setPendingBulkDelete] = useState(false);
  const [pendingDeleteRecord, setPendingDeleteRecord] =
    useState<HistoryRecord>();
  const [showBulkDownloads, setShowBulkDownloads] = useState(false);
  const [deletingHistory, setDeletingHistory] = useState(false);
  const [lightbox, setLightbox] = useState<{
    url: string;
    prompt: string;
    objectKey: string;
    fileId: string;
  }>();
  const manualCopyRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    listHistory().then((payload) => {
      if (payload.ok) {
        setRecords(payload.data.records);
      } else {
        setError(payload.error?.message ?? "历史记录读取失败");
      }
    });
  }, []);

  useEffect(() => {
    const missingFiles = records
      .flatMap((record) => record.files)
      .filter((file) => file.mediaType === "image" && !previewUrls[file.id]);
    if (missingFiles.length === 0) return;

    let cancelled = false;
    Promise.all(
      missingFiles.map(async (file) => {
        const payload = await getSignedFileURL(file.id);
        return payload.ok
          ? { fileId: file.id, url: payload.data.url }
          : undefined;
      }),
    )
      .then((loadedFiles) => {
        if (cancelled) return;
        const successfulFiles = loadedFiles.filter(
          (file): file is { fileId: string; url: string } => Boolean(file),
        );
        if (successfulFiles.length === 0) return;
        setPreviewUrls((current) => {
          const next = { ...current };
          for (const file of successfulFiles) {
            next[file.fileId] = current[file.fileId] ?? file.url;
          }
          return next;
        });
      })
      .catch(() => {
        // Keep the history list usable when signed preview URL preloading is temporarily unavailable.
      });

    return () => {
      cancelled = true;
    };
  }, [records, previewUrls]);

  useEffect(() => {
    const missingFiles = records
      .flatMap((record) => record.files)
      .filter((file) => file.mediaType === "image" && !downloadUrls[file.id]);
    if (missingFiles.length === 0) return;

    let cancelled = false;
    Promise.all(
      missingFiles.map(async (file) => {
        const payload = await getSignedFileDownloadURL(file.id);
        return payload.ok
          ? { fileId: file.id, url: payload.data.url }
          : undefined;
      }),
    )
      .then((loadedFiles) => {
        if (cancelled) return;
        const successfulFiles = loadedFiles.filter(
          (file): file is { fileId: string; url: string } => Boolean(file),
        );
        if (successfulFiles.length === 0) return;
        setDownloadUrls((current) => {
          const next = { ...current };
          for (const file of successfulFiles) {
            next[file.fileId] = current[file.fileId] ?? file.url;
          }
          return next;
        });
      })
      .catch(() => {
        // Download links can be retried on the next history render or page refresh.
      });

    return () => {
      cancelled = true;
    };
  }, [records, downloadUrls]);

  useEffect(() => {
    setSelectedRecordIds([]);
    setPendingBulkDelete(false);
    setPendingDeleteRecord(undefined);
    setShowBulkDownloads(false);
  }, [channelFilter, modelFilter, capabilityFilter, statusFilter]);

  useEffect(() => {
    if (!manualCopyText) return;
    manualCopyRef.current?.focus();
    manualCopyRef.current?.select();
  }, [manualCopyText]);

  async function previewFile(record: HistoryRecord, file: ResultFile) {
    setError(undefined);
    const payload = await getSignedFileURL(file.id);
    if (!payload.ok) {
      setError(payload.error?.message ?? "预览地址生成失败");
      return;
    }
    const url = payload.data.url;
    setPreviewUrls((current) => ({ ...current, [file.id]: url }));
    setLightbox({
      url,
      prompt: record.prompt,
      objectKey: file.objectKey,
      fileId: file.id,
    });
  }

  function openCachedPreview(record: HistoryRecord, file: ResultFile) {
    const url = previewUrls[file.id];
    if (!url) return;
    setLightbox({
      url,
      prompt: record.prompt,
      objectKey: file.objectKey,
      fileId: file.id,
    });
  }

  async function refreshHistoryPreviewUrl(fileId: string) {
    const payload = await getSignedFileURL(fileId);
    if (!payload.ok) return;
    setPreviewUrls((current) => ({ ...current, [fileId]: payload.data.url }));
  }

  function copyWithTextArea(text: string) {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.setAttribute("readonly", "true");
    textarea.style.position = "fixed";
    textarea.style.left = "-9999px";
    textarea.style.top = "0";
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    try {
      return document.execCommand("copy");
    } finally {
      document.body.removeChild(textarea);
    }
  }

  async function copyToClipboard(text: string, successMessage: string) {
    setError(undefined);
    setMessage(undefined);
    setManualCopyText("");
    try {
      try {
        await navigator.clipboard.writeText(text);
      } catch {
        if (!copyWithTextArea(text)) {
          throw new Error("copy failed");
        }
      }
      setMessage(successMessage);
    } catch {
      setManualCopyText(text);
      setError("复制失败，请手动复制下方文本");
    }
  }

  async function copyHistoryFileImage(file: ResultFile) {
    setError(undefined);
    setMessage(undefined);
    setManualCopyText("");
    setCopiedImageFileId("");
    const payload = await getFileContent(file.id);
    if (!payload.ok) {
      setError(payload.error?.message ?? "图片读取失败");
      return;
    }
    try {
      const mimeType = payload.data.mimeType.startsWith("image/")
        ? payload.data.mimeType
        : file.mimeType;
      await writeImageToClipboard(payload.data.bytes, mimeType);
      setCopiedImageFileId(file.id);
      setMessage("图片已复制");
    } catch {
      setError("复制图片失败，请使用下载图片或打开原图后复制");
    }
  }

  function handleHistoryImageKeyDown(
    event: KeyboardEvent<HTMLButtonElement>,
    file: ResultFile,
  ) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "c") {
      event.preventDefault();
      void copyHistoryFileImage(file);
    }
  }

  function historyRecordToDraft(
    record: HistoryRecord,
    notice?: string,
  ): ReuseHistoryDraft {
    const referenceFile =
      record.capability === "image-to-image"
        ? (firstReferenceImageFile(record.files) ??
          firstImageFile(record.files))
        : undefined;
    return {
      channelName: record.channelName,
      modelId: record.modelId,
      capability: record.capability,
      prompt: record.prompt,
      size:
        typeof record.parameters.size === "string"
          ? record.parameters.size
          : undefined,
      aspectRatio:
        typeof record.parameters.aspectRatio === "string"
          ? record.parameters.aspectRatio
          : undefined,
      resolution:
        typeof record.parameters.resolution === "string"
          ? record.parameters.resolution
          : undefined,
      quality:
        typeof record.parameters.quality === "string"
          ? record.parameters.quality
          : undefined,
      referenceFileId: referenceFile?.id,
      referenceObjectKey: referenceFile?.objectKey,
      referenceMimeType: referenceFile?.mimeType,
      notice,
    };
  }

  function openDraftInWorkspace(draft: ReuseHistoryDraft) {
    sessionStorage.setItem(REUSE_HISTORY_KEY, JSON.stringify(draft));
    window.history.pushState({}, "", "/");
    window.dispatchEvent(new PopStateEvent("popstate"));
  }

  function reuseRecord(record: HistoryRecord) {
    openDraftInWorkspace(historyRecordToDraft(record));
  }

  function regenerateRecord(record: HistoryRecord) {
    const notice =
      String(record.capability) === "image-to-text"
        ? "图生文记录需要重新上传参考图后再生成。"
        : record.capability === "image-to-image" &&
            (firstReferenceImageFile(record.files) ??
              firstImageFile(record.files))
          ? "已将历史垫图带回创作栏，可继续调整后再次生成。"
          : record.capability === "image-to-image"
            ? "图生图记录没有可用结果图，需要重新上传参考图后再生成。"
            : "已从历史记录带入创作参数，可继续调整后再次生成。";
    openDraftInWorkspace(historyRecordToDraft(record, notice));
  }

  function useHistoryFileAsReference(record: HistoryRecord, file: ResultFile) {
    openDraftInWorkspace({
      channelName: record.channelName,
      modelId: record.modelId,
      capability: "image-to-image",
      prompt: record.prompt,
      size:
        typeof record.parameters.size === "string"
          ? record.parameters.size
          : undefined,
      aspectRatio:
        typeof record.parameters.aspectRatio === "string"
          ? record.parameters.aspectRatio
          : undefined,
      resolution:
        typeof record.parameters.resolution === "string"
          ? record.parameters.resolution
          : undefined,
      quality:
        typeof record.parameters.quality === "string"
          ? record.parameters.quality
          : undefined,
      referenceFileId: file.id,
      referenceObjectKey: file.objectKey,
      referenceMimeType: file.mimeType,
      notice: "已将历史作品设为参考图，可继续图生图。",
    });
  }

  function requestRecordRemoval(record: HistoryRecord) {
    setError(undefined);
    setMessage(undefined);
    setPendingBulkDelete(false);
    setShowBulkDownloads(false);
    setPendingDeleteRecord(record);
  }

  async function removeRecord(record: HistoryRecord) {
    setError(undefined);
    setMessage(undefined);
    setPendingDeleteRecord(undefined);
    const payload = await deleteHistoryRecord(record.id);
    if (!payload.ok) {
      setError(payload.error?.message ?? "删除失败");
      return;
    }
    removeRecordsFromPage([record]);
    setMessage("历史记录已删除");
  }

  function toggleSelectedRecord(recordId: string) {
    setSelectedRecordIds((current) =>
      current.includes(recordId)
        ? current.filter((item) => item !== recordId)
        : [...current, recordId],
    );
    setPendingBulkDelete(false);
    setPendingDeleteRecord(undefined);
    setShowBulkDownloads(false);
  }

  function selectFilteredRecords() {
    setSelectedRecordIds(filteredRecords.map((record) => record.id));
    setPendingBulkDelete(false);
    setPendingDeleteRecord(undefined);
    setShowBulkDownloads(false);
    setMessage(undefined);
    setError(undefined);
  }

  function clearSelectedRecords() {
    setSelectedRecordIds([]);
    setPendingBulkDelete(false);
    setPendingDeleteRecord(undefined);
    setShowBulkDownloads(false);
    setMessage(undefined);
    setError(undefined);
  }

  async function copySelectedPrompts() {
    if (selectedRecords.length === 0) return;
    const prompts = selectedRecords.map((record) => record.prompt).join("\n\n");
    await copyToClipboard(prompts, `已复制 ${selectedRecords.length} 条提示词`);
    setPendingBulkDelete(false);
    setPendingDeleteRecord(undefined);
    setShowBulkDownloads(false);
  }

  async function copySelectedResults() {
    if (selectedResultTexts.length === 0) return;
    await copyToClipboard(
      selectedResultTexts.join("\n\n"),
      `已复制 ${selectedResultTexts.length} 条结果`,
    );
    setPendingBulkDelete(false);
    setPendingDeleteRecord(undefined);
    setShowBulkDownloads(false);
  }

  function clearHistoryFilters() {
    setChannelFilter("");
    setModelFilter("");
    setCapabilityFilter("");
    setStatusFilter("");
    setPendingBulkDelete(false);
    setPendingDeleteRecord(undefined);
    setShowBulkDownloads(false);
    setMessage(undefined);
    setError(undefined);
  }

  function requestSelectedRecordsRemoval() {
    if (selectedRecordIds.length === 0) return;
    setError(undefined);
    setMessage(undefined);
    setShowBulkDownloads(false);
    setPendingDeleteRecord(undefined);
    setPendingBulkDelete(true);
  }

  function showSelectedImageDownloads() {
    if (
      selectedImageFiles.length === 0 ||
      selectedImageFilesWaitingForDownload > 0
    )
      return;
    setError(undefined);
    setMessage(undefined);
    setPendingBulkDelete(false);
    setPendingDeleteRecord(undefined);
    setShowBulkDownloads(true);
  }

  async function confirmSelectedRecordsRemoval() {
    const selectedRecords = filteredRecords.filter((record) =>
      selectedRecordIds.includes(record.id),
    );
    if (selectedRecords.length === 0) return;
    setError(undefined);
    setMessage(undefined);
    setDeletingHistory(true);
    const deletedRecords: HistoryRecord[] = [];
    let failedCount = 0;
    try {
      for (const record of selectedRecords) {
        const payload = await deleteHistoryRecord(record.id);
        if (!payload.ok) {
          if (payload.error?.code === "history_not_found") {
            deletedRecords.push(record);
          } else {
            failedCount += 1;
          }
          continue;
        }
        deletedRecords.push(record);
      }
      removeRecordsFromPage(deletedRecords);
      setPendingBulkDelete(false);
      setPendingDeleteRecord(undefined);
      if (failedCount > 0) {
        setError(
          `已删除 ${deletedRecords.length} 条历史记录，${failedCount} 条删除失败`,
        );
      } else {
        setMessage(`已删除 ${deletedRecords.length} 条历史记录`);
      }
    } finally {
      setDeletingHistory(false);
    }
  }

  function removeRecordsFromPage(deletedRecords: HistoryRecord[]) {
    const deletedIds = new Set(deletedRecords.map((record) => record.id));
    const deletedFileIds = new Set(
      deletedRecords.flatMap((record) => record.files.map((file) => file.id)),
    );
    setRecords((current) => current.filter((item) => !deletedIds.has(item.id)));
    setSelectedRecordIds((current) =>
      current.filter((id) => !deletedIds.has(id)),
    );
    setPreviewUrls((current) => {
      const next = { ...current };
      for (const fileId of deletedFileIds) {
        delete next[fileId];
      }
      return next;
    });
    setDownloadUrls((current) => {
      const next = { ...current };
      for (const fileId of deletedFileIds) {
        delete next[fileId];
      }
      return next;
    });
    setLightbox((current) =>
      current && deletedFileIds.has(current.fileId) ? undefined : current,
    );
    setShowBulkDownloads(false);
  }

  const visibleRecords = records.filter((record) => record.status === "完成");
  const channelOptions = uniqueValues(
    visibleRecords.map((record) => record.channelName),
  );
  const modelOptions = uniqueValues(
    visibleRecords.map((record) => record.modelId),
  );
  const statusOptions = uniqueValues(
    visibleRecords.map((record) => record.status),
  );
  const hasActiveFilters = Boolean(
    channelFilter || modelFilter || capabilityFilter || statusFilter,
  );
  const filteredRecords = visibleRecords
    .filter((record) => !channelFilter || record.channelName === channelFilter)
    .filter((record) => !modelFilter || record.modelId === modelFilter)
    .filter(
      (record) => !capabilityFilter || record.capability === capabilityFilter,
    )
    .filter((record) => !statusFilter || record.status === statusFilter)
    .slice()
    .sort((left, right) => right.createdAt - left.createdAt);
  const selectedRecords = filteredRecords.filter((record) =>
    selectedRecordIds.includes(record.id),
  );
  const selectedResultTexts = selectedRecords
    .map((record) => record.resultText?.trim())
    .filter((resultText): resultText is string => Boolean(resultText));
  const selectedImageFiles = selectedRecords.flatMap((record) =>
    record.files
      .filter((file) => file.mediaType === "image")
      .map((file) => ({
        file,
        record,
      })),
  );
  const selectedImageFilesWaitingForDownload = selectedImageFiles.filter(
    ({ file }) => !downloadUrls[file.id],
  ).length;
  const selectedDownloadFiles = selectedImageFiles.filter(({ file }) =>
    Boolean(downloadUrls[file.id]),
  );
  const canShowBulkDownloads =
    selectedImageFiles.length > 0 &&
    selectedImageFilesWaitingForDownload === 0 &&
    !deletingHistory;

  return (
    <>
      <PageHeader eyebrow="HISTORY" title="历史记录" username={username} />
      {message ? <div className="success page-message">{message}</div> : null}
      {error ? <div className="error page-message">{error}</div> : null}
      {manualCopyText ? (
        <section className="manual-copy-box" aria-label="手动复制">
          <div>
            <label htmlFor="manual-copy-text">手动复制文本</label>
            <button
              type="button"
              onClick={() => {
                setManualCopyText("");
                setError(undefined);
              }}
            >
              关闭手动复制
            </button>
          </div>
          <textarea
            id="manual-copy-text"
            readOnly
            ref={manualCopyRef}
            value={manualCopyText}
            onFocus={(event) => event.currentTarget.select()}
          />
        </section>
      ) : null}
      <section className="history-filter-bar" aria-label="历史筛选">
        <label>
          供应源筛选
          <select
            value={channelFilter}
            onChange={(event) => setChannelFilter(event.target.value)}
          >
            <option value="">全部供应源</option>
            {channelOptions.map((channelName) => (
              <option key={channelName} value={channelName}>
                {channelName}
              </option>
            ))}
          </select>
        </label>
        <label>
          模型筛选
          <select
            value={modelFilter}
            onChange={(event) => setModelFilter(event.target.value)}
          >
            <option value="">全部模型</option>
            {modelOptions.map((item) => (
              <option key={item} value={item}>
                {item}
              </option>
            ))}
          </select>
        </label>
        <label>
          类型筛选
          <select
            value={capabilityFilter}
            onChange={(event) => setCapabilityFilter(event.target.value)}
          >
            <option value="">全部类型</option>
            {MODEL_CAPABILITIES.map((capability) => (
              <option key={capability} value={capability}>
                {capabilityLabel(capability)}
              </option>
            ))}
          </select>
        </label>
        <label>
          状态筛选
          <select
            value={statusFilter}
            onChange={(event) => setStatusFilter(event.target.value)}
          >
            <option value="">全部状态</option>
            {statusOptions.map((status) => (
              <option key={status} value={status}>
                {status}
              </option>
            ))}
          </select>
        </label>
        <button
          type="button"
          onClick={clearHistoryFilters}
          disabled={!hasActiveFilters}
        >
          清空筛选
        </button>
      </section>
      <section className="history-bulk-bar" aria-label="批量操作">
        <strong>已选 {selectedRecordIds.length} 项</strong>
        <div>
          <button
            type="button"
            onClick={selectFilteredRecords}
            disabled={filteredRecords.length === 0}
          >
            选择当前筛选
          </button>
          <button
            type="button"
            onClick={() => void copySelectedPrompts()}
            disabled={selectedRecordIds.length === 0 || deletingHistory}
          >
            复制所选提示词
          </button>
          <button
            type="button"
            onClick={() => void copySelectedResults()}
            disabled={selectedResultTexts.length === 0 || deletingHistory}
          >
            复制所选结果
          </button>
          <button
            type="button"
            onClick={showSelectedImageDownloads}
            disabled={!canShowBulkDownloads}
          >
            下载所选图片
          </button>
          <button
            type="button"
            onClick={clearSelectedRecords}
            disabled={selectedRecordIds.length === 0 || deletingHistory}
          >
            清空选择
          </button>
          <button
            className="danger-action"
            type="button"
            onClick={requestSelectedRecordsRemoval}
            disabled={selectedRecordIds.length === 0 || deletingHistory}
          >
            删除所选
          </button>
        </div>
      </section>
      {pendingBulkDelete ? (
        <section className="history-confirm-bar" aria-label="确认批量删除">
          <strong>
            确认删除选中的 {selectedRecordIds.length} 条历史记录？
          </strong>
          <div>
            <button
              type="button"
              onClick={() => setPendingBulkDelete(false)}
              disabled={deletingHistory}
            >
              取消
            </button>
            <button
              className="danger-action"
              type="button"
              onClick={() => void confirmSelectedRecordsRemoval()}
              disabled={deletingHistory}
            >
              {deletingHistory ? "删除中..." : "确认删除"}
            </button>
          </div>
        </section>
      ) : null}
      {pendingDeleteRecord ? (
        <section className="history-confirm-bar" aria-label="确认删除历史记录">
          <div>
            <strong>确认删除历史记录？</strong>
            <p>{pendingDeleteRecord.prompt}</p>
          </div>
          <div>
            <button
              type="button"
              onClick={() => setPendingDeleteRecord(undefined)}
              disabled={deletingHistory}
            >
              取消
            </button>
            <button
              className="danger-action"
              type="button"
              onClick={() => void removeRecord(pendingDeleteRecord)}
              disabled={deletingHistory}
            >
              {deletingHistory ? "删除中..." : "确认删除记录"}
            </button>
          </div>
        </section>
      ) : null}
      {showBulkDownloads && selectedDownloadFiles.length > 0 ? (
        <section className="history-download-panel" aria-label="批量下载图片">
          <div className="history-download-panel-header">
            <strong>已准备 {selectedDownloadFiles.length} 张图片下载</strong>
            <button type="button" onClick={() => setShowBulkDownloads(false)}>
              关闭批量下载
            </button>
          </div>
          <div className="history-download-list">
            {selectedDownloadFiles.map(({ file, record }) => (
              <a
                key={file.id}
                href={downloadUrls[file.id]}
                download={filenameFromObjectKey(file.objectKey)}
                aria-label={`下载 ${record.prompt}`}
              >
                <span>{record.prompt}</span>
                <small>{filenameFromObjectKey(file.objectKey)}</small>
              </a>
            ))}
          </div>
        </section>
      ) : null}
      <div className="history-list">
        {visibleRecords.length === 0 ? (
          <div className="placeholder-panel">还没有历史记录。</div>
        ) : filteredRecords.length === 0 ? (
          <div className="placeholder-panel">没有匹配的历史记录。</div>
        ) : (
          filteredRecords.map((record) => {
            const displayFiles = orderedDisplayFiles(record.files);
            const shouldShowMissingReference =
              record.capability === "image-to-image" &&
              missingReferenceImage(record.files);
            return (
              <article className="history-item" key={record.id}>
                <div className="history-item-header">
                  <label className="history-select">
                    <input
                      aria-label={`选择 ${record.prompt}`}
                      checked={selectedRecordIds.includes(record.id)}
                      type="checkbox"
                      onChange={() => toggleSelectedRecord(record.id)}
                    />
                  </label>
                  <div>
                    <strong>{record.prompt}</strong>
                    <p>
                      {record.channelName} / {record.modelId}
                    </p>
                  </div>
                  <div className="history-item-meta">
                    <span
                      className={
                        record.status === "失败" ? "danger-pill" : undefined
                      }
                    >
                      {record.status} · {capabilityLabel(record.capability)}
                    </span>
                    {typeof record.durationMs === "number" ? (
                      <span>耗时 {formatElapsed(record.durationMs)}</span>
                    ) : null}
                  </div>
                </div>
                <div className="history-actions">
                  <button type="button" onClick={() => reuseRecord(record)}>
                    复用到创作
                  </button>
                  <button
                    type="button"
                    onClick={() => regenerateRecord(record)}
                  >
                    再次生成
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      void copyToClipboard(record.prompt, "提示词已复制")
                    }
                  >
                    复制提示词
                  </button>
                  {record.resultText ? (
                    <button
                      type="button"
                      onClick={() =>
                        void copyToClipboard(
                          record.resultText ?? "",
                          "结果已复制",
                        )
                      }
                    >
                      复制结果
                    </button>
                  ) : null}
                  <button
                    className="danger-action"
                    type="button"
                    onClick={() => requestRecordRemoval(record)}
                  >
                    删除
                  </button>
                </div>
                {record.failureReason ? (
                  <div className="failure-result">{record.failureReason}</div>
                ) : null}
                {record.resultText ? (
                  <div className="text-result">{record.resultText}</div>
                ) : null}
                {shouldShowMissingReference ? (
                  <div className="missing-reference-note">
                    <strong>缺少原始垫图</strong>
                    <span>这条图生图记录没有保存原图，只能看到生成图。</span>
                  </div>
                ) : null}
                {displayFiles.length > 0 ? (
                  <div className="file-grid">
                    {displayFiles.map((file) => {
                      const label = fileDisplayLabel(file, record.files);
                      return (
                        <div className="file-preview" key={file.id}>
                          {previewUrls[file.id] ? (
                            <button
                              className="preview-thumb-button"
                              type="button"
                              aria-label={`打开预览 ${record.prompt}`}
                              onClick={() => openCachedPreview(record, file)}
                              onKeyDown={(event) =>
                                handleHistoryImageKeyDown(event, file)
                              }
                            >
                              <img
                                alt={
                                  isReferenceFile(file)
                                    ? `${label} ${record.prompt}`
                                    : record.prompt
                                }
                                src={previewUrls[file.id]}
                                onError={() =>
                                  void refreshHistoryPreviewUrl(file.id)
                                }
                              />
                            </button>
                          ) : (
                            <button
                              type="button"
                              onClick={() => void previewFile(record, file)}
                            >
                              预览
                            </button>
                          )}
                          {previewUrls[file.id] ? (
                            <a
                              className="file-original-link"
                              href={previewUrls[file.id]}
                              target="_blank"
                              rel="noreferrer"
                              aria-label={`打开原图 ${record.prompt}`}
                            >
                              打开原图
                            </a>
                          ) : null}
                          {downloadUrls[file.id] ? (
                            <a
                              className="file-original-link"
                              href={downloadUrls[file.id]}
                              download={filenameFromObjectKey(file.objectKey)}
                              aria-label={`下载图片 ${record.prompt}`}
                            >
                              下载图片
                            </a>
                          ) : null}
                          {file.mediaType === "image" ? (
                            <button
                              type="button"
                              aria-label={`${copiedImageFileId === file.id ? "已复制" : "复制图片"} ${record.prompt}`}
                              onClick={() => void copyHistoryFileImage(file)}
                            >
                              {copiedImageFileId === file.id
                                ? "已复制"
                                : "复制图片"}
                            </button>
                          ) : null}
                          {file.mediaType === "image" ? (
                            <button
                              type="button"
                              aria-label={referenceActionLabel(
                                file,
                                record.files,
                                record.prompt,
                                "历史",
                              )}
                              onClick={() =>
                                useHistoryFileAsReference(record, file)
                              }
                            >
                              设为参考图
                            </button>
                          ) : null}
                          <strong className="file-role-label">{label}</strong>
                          <span>{file.objectKey}</span>
                        </div>
                      );
                    })}
                  </div>
                ) : null}
              </article>
            );
          })
        )}
      </div>
      {lightbox ? (
        <div
          className="image-lightbox"
          role="dialog"
          aria-label="图片预览"
          aria-modal="true"
        >
          <button
            className="image-lightbox-backdrop"
            type="button"
            aria-label="关闭图片预览背景"
            onClick={() => setLightbox(undefined)}
          />
          <section className="image-lightbox-panel">
            <header>
              <div>
                <span className="eyebrow">PREVIEW</span>
                <h2>{lightbox.prompt}</h2>
              </div>
              <div className="image-lightbox-actions">
                <a href={lightbox.url} target="_blank" rel="noreferrer">
                  打开原图
                </a>
                <button type="button" onClick={() => setLightbox(undefined)}>
                  关闭预览
                </button>
              </div>
            </header>
            <div className="image-lightbox-stage">
              <img alt={lightbox.prompt} src={lightbox.url} />
            </div>
            <p>{lightbox.objectKey}</p>
          </section>
        </div>
      ) : null}
    </>
  );
}

function AdminUsersPage({ username }: { username: string }) {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [newUsername, setNewUsername] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [message, setMessage] = useState<string>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    void refreshUsers();
  }, []);

  async function refreshUsers() {
    const payload = await listAdminUsers();
    if (payload.ok) {
      setUsers(payload.data.users);
    } else {
      setError(payload.error?.message ?? "用户列表读取失败");
    }
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(undefined);
    setMessage(undefined);
    const payload = await createAdminUser({
      username: newUsername,
      password: newPassword,
      role: "user",
    });
    if (!payload.ok) {
      setError(payload.error?.message ?? "创建用户失败");
      return;
    }
    setUsers((current) => [...current, payload.data.user]);
    setNewUsername("");
    setNewPassword("");
    setMessage("用户已创建");
  }

  async function toggleDisabled(user: AdminUser) {
    const disabled = !user.disabledAt;
    const payload = await setAdminUserDisabled(user.id, disabled);
    if (!payload.ok) {
      setError(payload.error?.message ?? "用户状态更新失败");
      return;
    }
    setUsers((current) =>
      current.map((item) =>
        item.id === user.id
          ? { ...item, disabledAt: disabled ? Date.now() : 0 }
          : item,
      ),
    );
  }

  async function resetPassword(user: AdminUser) {
    const password = window.prompt(`输入 ${user.username} 的新密码`);
    if (!password) return;
    const payload = await resetAdminUserPassword(user.id, password);
    if (!payload.ok) {
      setError(payload.error?.message ?? "密码重置失败");
      return;
    }
    setMessage("密码已重置");
  }

  return (
    <>
      <PageHeader eyebrow="ADMIN" title="用户管理" username={username} />
      {message ? <div className="success">{message}</div> : null}
      {error ? <div className="error">{error}</div> : null}
      <div className="admin-grid">
        <form className="panel" onSubmit={submit}>
          <h2>创建用户</h2>
          <label>
            新用户名
            <input
              value={newUsername}
              onChange={(event) => setNewUsername(event.target.value)}
            />
          </label>
          <label>
            初始密码
            <input
              value={newPassword}
              onChange={(event) => setNewPassword(event.target.value)}
              type="password"
            />
          </label>
          <button type="submit">创建用户</button>
        </form>

        <section className="panel">
          <h2>账号列表</h2>
          <div className="channel-list">
            {users.map((user) => (
              <article className="channel-item user-item" key={user.id}>
                <div>
                  <strong>{user.username}</strong>
                  <p>{user.role === "admin" ? "管理员" : "普通用户"}</p>
                </div>
                <span>{user.disabledAt ? "已禁用" : "启用中"}</span>
                <div className="button-row">
                  <button
                    type="button"
                    onClick={() => void toggleDisabled(user)}
                  >
                    {user.disabledAt ? "启用" : "禁用"}
                  </button>
                  <button
                    type="button"
                    onClick={() => void resetPassword(user)}
                  >
                    重置密码
                  </button>
                </div>
              </article>
            ))}
          </div>
        </section>
      </div>
    </>
  );
}
