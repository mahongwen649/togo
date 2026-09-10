import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const providersPayload = {
  ok: true,
  data: {
    defaultProviderId: "openai",
    providers: [
      {
        id: "openai",
        name: "OpenAI",
        available: true,
        defaultModelId: "gpt-image-2",
        models: [
          { id: "gpt-image-1", capabilities: ["text-to-image", "image-to-image"] },
          { id: "gpt-image-2", capabilities: ["text-to-image", "image-to-image"] },
        ],
      },
      {
        id: "grok",
        name: "Grok Heavy",
        available: true,
        defaultModelId: "grok-imagine-image-2.0",
        models: [
          { id: "grok-imagine-image-1.0", capabilities: ["text-to-image", "image-to-image"] },
          { id: "grok-imagine-image-2.0", capabilities: ["text-to-image", "image-to-image"] },
        ],
      },
    ],
  },
};

function installAuthenticatedFetch(payload = providersPayload) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = String(input);
    if (url === "/api/me") {
      return jsonResponse({
        ok: true,
        data: { user: { id: "usr_1", username: "alice", role: "user" } },
      });
    }
    if (url === "/api/providers") return jsonResponse(payload);
    if (url === "/api/history") return jsonResponse({ ok: true, data: { records: [] } });
    if (url === "/api/tasks") return jsonResponse({ ok: true, data: { tasks: [] } });
    if (url === "/api/generate/image" && init?.method === "POST") {
      return jsonResponse({
        ok: true,
        data: { taskId: "tsk_1", historyId: "his_1", status: "完成", files: [] },
      });
    }
    return jsonResponse({ ok: false }, 404);
  });
}

describe("App authentication and image workspace", () => {
  beforeEach(() => {
    window.history.pushState({}, "", "/");
    window.localStorage.clear();
    window.sessionStorage.clear();
    vi.restoreAllMocks();
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("shows host-entry guidance when current user is unauthenticated", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(jsonResponse({ ok: false }, 401));

    render(<App />);

    expect(await screen.findByRole("heading", { name: "请从主站进入" })).toBeInTheDocument();
    expect(screen.queryByLabelText("用户名")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "登录" })).not.toBeInTheDocument();
  });

  it("opens the workspace without a settings navigation item", async () => {
    installAuthenticatedFetch();

    render(<App />);

    expect(await screen.findByRole("heading", { name: "创作工作台" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "创作" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "历史" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "设置" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "用户" })).not.toBeInTheDocument();
  });

  it("redirects the removed settings route back to the workspace", async () => {
    window.history.pushState({}, "", "/settings");
    installAuthenticatedFetch();

    render(<App />);

    expect(await screen.findByRole("heading", { name: "创作工作台" })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/");
  });

  it("loads OpenAI by default and selects the provider default model", async () => {
    const fetchMock = installAuthenticatedFetch();

    render(<App />);

    expect(await screen.findByRole("heading", { name: "创作工作台" })).toBeInTheDocument();
    expect(await screen.findByDisplayValue("gpt-image-2")).toBeInTheDocument();
    expect(screen.getByLabelText("引擎")).toHaveValue("openai");
    expect(fetchMock).toHaveBeenCalledWith("/api/providers", expect.anything());
  });

  it("falls back to the first available provider when OpenAI is unavailable", async () => {
    installAuthenticatedFetch({
      ...providersPayload,
      data: {
        ...providersPayload.data,
        providers: providersPayload.data.providers.map((provider) =>
          provider.id === "openai" ? { ...provider, available: false } : provider,
        ),
      },
    });
    render(<App />);

    expect(await screen.findByDisplayValue("grok-imagine-image-2.0")).toBeInTheDocument();
    expect(screen.getByLabelText("引擎")).toHaveValue("grok");
  });

  it("refreshes the model list when switching to Grok", async () => {
    installAuthenticatedFetch();
    render(<App />);

    const engine = await screen.findByLabelText("引擎");
    await screen.findByRole("option", { name: "Grok Heavy" });
    await userEvent.selectOptions(engine, "grok");

    expect(await screen.findByDisplayValue("grok-imagine-image-2.0")).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "gpt-image-2" })).not.toBeInTheDocument();
    expect(screen.getByLabelText("引擎")).toHaveValue("grok");
  });

  it("submits a text-to-image request with the selected provider", async () => {
    const fetchMock = installAuthenticatedFetch();
    render(<App />);

    await screen.findByRole("heading", { name: "创作工作台" });
    await userEvent.type(screen.getByLabelText("提示词"), "a red fox in a library");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/generate/image",
        expect.objectContaining({ method: "POST" }),
      );
    });
    const request = fetchMock.mock.calls.find(([input]) => input === "/api/generate/image");
    expect(request?.[1]?.body).toBe(
      JSON.stringify({
        providerId: "openai",
        channelId: "openai",
        modelId: "gpt-image-2",
        prompt: "a red fox in a library",
        size: "1024x1024",
      }),
    );
  });

  it("submits a multipart image-to-image request with the reference image", async () => {
    const fetchMock = installAuthenticatedFetch();
    render(<App />);

    await screen.findByRole("heading", { name: "创作工作台" });
    await userEvent.selectOptions(screen.getByLabelText("模式"), "image-to-image");
    await userEvent.upload(
      screen.getByLabelText("参考图"),
      new File(["reference"], "reference.png", { type: "image/png" }),
    );
    await userEvent.type(screen.getByLabelText("提示词"), "make it cinematic");
    await userEvent.click(screen.getByRole("button", { name: "开始图生图" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/generate/image",
        expect.objectContaining({ method: "POST" }),
      );
    });
    const request = fetchMock.mock.calls.find(([input]) => input === "/api/generate/image");
    const body = request?.[1]?.body;
    expect(body).toBeInstanceOf(FormData);
    expect((body as FormData).get("providerId")).toBe("openai");
    expect((body as FormData).get("channelId")).toBe("openai");
    expect((body as FormData).get("modelId")).toBe("gpt-image-2");
    expect((body as FormData).get("prompt")).toBe("make it cinematic");
    expect((body as FormData).get("inputImage")).toBeInstanceOf(File);
  });
});
