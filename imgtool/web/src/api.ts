export type ApiSuccess<T> = {
  ok: true;
  data: T;
};

export type ApiFailure = {
  ok: false;
  error?: {
    source?: string;
    code?: string;
    message: string;
  };
};

export type ApiResponse<T> = ApiSuccess<T> | ApiFailure;

export type CurrentUser = {
  id: string;
  username: string;
  role: "admin" | "user";
};

export type Capability = "text-to-image" | "image-to-image";

export type ChannelModel = {
  id: string;
  capabilities: Capability[];
};

export type Channel = {
  id: string;
  name: string;
  baseUrl: string;
  hasApiKey: boolean;
  models: ChannelModel[];
  createdAt?: number;
  updatedAt?: number;
};

export type ImageProvider = {
  id: "openai" | "grok" | string;
  name: string;
  models: ChannelModel[];
  defaultModelId?: string;
  available: boolean;
  message?: string;
};

export type AdminUser = {
  id: string;
  username: string;
  role: "admin" | "user";
  disabledAt?: number;
  createdAt?: number;
  updatedAt?: number;
};

export type GenerationResult = {
  taskId: string;
  historyId: string;
  status: string;
  durationMs?: number;
  resultText?: string;
  files?: ResultFile[];
};

export type ResultFile = {
  id: string;
  storageProvider: string;
  bucket: string;
  objectKey: string;
  role?: "reference" | "result" | string;
  mediaType: string;
  mimeType: string;
  sizeBytes: number;
  createdAt: number;
};

export type HistoryRecord = {
  id: string;
  taskId: string;
  kind: string;
  status: string;
  channelName: string;
  modelId: string;
  capability: Capability;
  prompt: string;
  parameters: Record<string, unknown>;
  resultText?: string;
  failureReason?: string;
  durationMs?: number;
  files: ResultFile[];
  createdAt: number;
};

export type Task = {
  id: string;
  kind: string;
  status: string;
  channelName: string;
  modelId: string;
  capability: Capability;
  prompt: string;
  parameters: Record<string, unknown>;
  startedAt: number;
  endedAt?: number;
  durationMs?: number;
  errorSource?: string;
  failureReason?: string;
  createdAt: number;
  updatedAt: number;
};

export async function getMe() {
  return request<{ user: CurrentUser }>("/api/me");
}

export async function login(username: string, password: string) {
  return request<{ user: CurrentUser }>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}

export async function logout() {
  return request<{ loggedOut: boolean }>("/api/auth/logout", {
    method: "POST",
  });
}

export async function listProviders() {
  return request<{
    defaultProviderId: string;
    providers: ImageProvider[];
  }>("/api/providers");
}

export async function listAdminUsers() {
  return request<{ users: AdminUser[] }>("/api/admin/users");
}

export async function createAdminUser(input: {
  username: string;
  password: string;
  role?: "admin" | "user";
}) {
  return request<{ user: AdminUser }>("/api/admin/users", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function setAdminUserDisabled(id: string, disabled: boolean) {
  return request<{ updated: boolean }>(`/api/admin/users/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ disabled }),
  });
}

export async function resetAdminUserPassword(id: string, password: string) {
  return request<{ updated: boolean }>(
    `/api/admin/users/${id}/reset-password`,
    {
      method: "POST",
      body: JSON.stringify({ password }),
    },
  );
}

export async function generateImage(input: {
  providerId: string;
  channelId: string;
  modelId: string;
  prompt: string;
  size?: string;
  quality?: string;
  aspectRatio?: string;
  resolution?: string;
}) {
  return request<GenerationResult>("/api/generate/image", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function generateImageEdit(formData: FormData) {
  return requestForm<GenerationResult>("/api/generate/image", formData);
}

export async function listHistory() {
  return request<{ records: HistoryRecord[] }>("/api/history");
}

export async function listTasks() {
  return request<{ tasks: Task[] }>("/api/tasks");
}

export async function getTask(id: string) {
  return request<{ task: Task }>(`/api/tasks/${id}`);
}

export async function cancelTask(id: string) {
  return request<{ cancelled: boolean; record?: HistoryRecord }>(
    `/api/tasks/${id}/cancel`,
    {
      method: "POST",
    },
  );
}

export async function deleteHistoryRecord(id: string) {
  return request<{ deleted: boolean }>(`/api/history/${id}`, {
    method: "DELETE",
  });
}

export async function getSignedFileURL(fileId: string) {
  return request<{ url: string; file?: ResultFile }>(
    `/api/files/${fileId}/signed-url`,
  );
}

export async function getSignedFileDownloadURL(fileId: string) {
  return request<{ url: string; file?: ResultFile }>(
    `/api/files/${fileId}/download-url`,
  );
}

export async function getFileContent(
  fileId: string,
): Promise<ApiResponse<{ bytes: ArrayBuffer; mimeType: string }>> {
  const response = await fetch(`/api/files/${fileId}/content`, {
    credentials: "include",
  });
  if (!response.ok) {
    const payload = (await response.json().catch(() => undefined)) as
      ApiFailure | undefined;
    return {
      ok: false,
      error: payload?.error ?? { message: `请求失败：${response.status}` },
    };
  }
  const bytes = await response.arrayBuffer();
  return {
    ok: true,
    data: {
      bytes,
      mimeType: response.headers.get("Content-Type") ?? "",
    },
  };
}

async function request<T>(
  url: string,
  init: RequestInit = {},
): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...init.headers,
    },
  });
  const payload = (await response
    .json()
    .catch(() => ({ ok: false }))) as ApiResponse<T>;
  if (!response.ok && payload.ok !== false) {
    return {
      ok: false,
      error: { message: `请求失败：${response.status}` },
    };
  }
  return payload;
}

async function requestForm<T>(
  url: string,
  body: FormData,
): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    method: "POST",
    body,
    credentials: "include",
  });
  const payload = (await response
    .json()
    .catch(() => ({ ok: false }))) as ApiResponse<T>;
  if (!response.ok && payload.ok !== false) {
    return {
      ok: false,
      error: { message: `请求失败：${response.status}` },
    };
  }
  return payload;
}

