import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import type { Channel } from "./api";

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function imageResponse(body: BodyInit, contentType = "image/png") {
  return new Response(body, {
    status: 200,
    headers: { "Content-Type": contentType },
  });
}

describe("App auth shell", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("shows host-entry guidance when current user is unauthenticated", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ ok: false }, 401),
    );

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "请从主站进入" }),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("用户名")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("密码")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "登录" }),
    ).not.toBeInTheDocument();
  });

  it("does not expose password login when unauthenticated", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ ok: false }, 401));

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "请从主站进入" }),
    ).toBeInTheDocument();

    expect(fetchMock).not.toHaveBeenCalledWith(
      "/api/auth/login",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
      }),
    );
  });

  it("renders app shell when current user exists", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({
        ok: true,
        data: { user: { id: "usr_1", username: "alice", role: "user" } },
      }),
    );

    render(<App />);

    expect(await screen.findByText("alice")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "创作" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "设置" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "历史" })).toBeInTheDocument();
  });

  it("marks the current sidebar page as active", async () => {
    window.history.pushState({}, "", "/settings");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "admin" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({ ok: true, data: { channels: [] } });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "设置" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("link", { name: "创作" })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("logs out and returns to login page", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/auth/logout" && init?.method === "POST") {
          return jsonResponse({ ok: true, data: { loggedOut: true } });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(await screen.findByText("alice")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "退出" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/auth/logout",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
        }),
      );
    });
    expect(
      await screen.findByRole("heading", { name: "请从主站进入" }),
    ).toBeInTheDocument();
  });

  it("loads channels on settings page and creates a channel without displaying the api key", async () => {
    window.history.pushState({}, "", "/settings");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels" && (!init || init.method === undefined)) {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/channels" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              channel: {
                id: "chn_2",
                name: "Router",
                baseUrl: "https://router.example.com",
                hasApiKey: true,
                models: [{ id: "model-a", capabilities: ["text-to-image"] }],
              },
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("OpenAI")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("供应源名称"), "Router");
    await userEvent.type(
      screen.getByLabelText("Base URL"),
      "https://router.example.com",
    );
    await userEvent.type(screen.getByLabelText("API Key"), "sk-secret");
    await userEvent.type(screen.getByLabelText("模型 ID"), "model-a");
    await userEvent.click(screen.getByRole("button", { name: "保存供应源" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/channels",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
        }),
      );
    });
    expect(screen.queryByText("sk-secret")).not.toBeInTheDocument();
  });

  it("presents settings as a compact router console", async () => {
    window.history.pushState({}, "", "/settings");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "供应源档案" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "模型配置" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("1 个供应源")).toBeInTheDocument();
  });

  it("edits and deletes channels from settings page", async () => {
    window.history.pushState({}, "", "/settings");
    let channelState = [
      {
        id: "chn_1",
        name: "OpenAI",
        baseUrl: "https://api.openai.com",
        hasApiKey: true,
        models: [
          {
            id: "gpt-image-1",
            capabilities: ["text-to-image", "image-to-text"],
          },
        ],
      },
    ];
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels" && (!init || init.method === undefined)) {
          return jsonResponse({
            ok: true,
            data: {
              channels: channelState,
            },
          });
        }
        if (input === "/api/channels/chn_1" && init?.method === "PATCH") {
          channelState = [
            {
              id: "chn_1",
              name: "OpenAI Updated",
              baseUrl: "https://api.openai.com",
              hasApiKey: true,
              models: [
                {
                  id: "gpt-image-1",
                  capabilities: ["text-to-image", "image-to-text"],
                },
              ],
            },
          ];
          return jsonResponse({
            ok: true,
            data: {
              channel: {
                id: "chn_1",
                name: "OpenAI Updated",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-1",
                    capabilities: ["text-to-image", "image-to-text"],
                  },
                ],
              },
            },
          });
        }
        if (input === "/api/channels/chn_1" && init?.method === "DELETE") {
          channelState = [];
          return jsonResponse({ ok: true, data: { deleted: true } });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    await userEvent.click(await screen.findByRole("button", { name: "编辑" }));
    await userEvent.clear(screen.getByLabelText("供应源名称"));
    await userEvent.type(screen.getByLabelText("供应源名称"), "OpenAI Updated");
    await userEvent.type(screen.getByLabelText("API Key"), "sk-new");
    await userEvent.click(screen.getByRole("button", { name: "保存修改" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/channels/chn_1",
        expect.objectContaining({
          method: "PATCH",
          credentials: "include",
        }),
      );
    });
    expect(await screen.findByText("OpenAI Updated")).toBeInTheDocument();
    expect(screen.queryByText("sk-new")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "删除" }));
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/channels/chn_1",
        expect.objectContaining({
          method: "DELETE",
          credentials: "include",
        }),
      );
    });
    expect(screen.queryByText("OpenAI Updated")).not.toBeInTheDocument();
  });

  it("refreshes channels from the server after saving settings", async () => {
    window.history.pushState({}, "", "/settings");
    let listCount = 0;
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels" && (!init || init.method === undefined)) {
          listCount += 1;
          return jsonResponse({
            ok: true,
            data: {
              channels:
                listCount === 1
                  ? []
                  : [
                      {
                        id: "chn_1",
                        name: "Saved Channel",
                        baseUrl: "https://api.example.com",
                        hasApiKey: true,
                        models: [
                          {
                            id: "gpt-image-1",
                            capabilities: ["text-to-image"],
                          },
                        ],
                      },
                    ],
            },
          });
        }
        if (input === "/api/channels" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              channel: {
                id: "temporary",
                name: "Temporary Local",
                baseUrl: "https://api.example.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("供应源名称"), "Saved Channel");
    await userEvent.type(
      screen.getByLabelText("Base URL"),
      "https://api.example.com",
    );
    await userEvent.type(screen.getByLabelText("API Key"), "sk-test");
    await userEvent.type(screen.getByLabelText("模型 ID"), "gpt-image-1");
    await userEvent.click(screen.getByRole("button", { name: "保存供应源" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/channels",
        expect.objectContaining({
          credentials: "include",
        }),
      );
    });
    expect(await screen.findByText("Saved Channel")).toBeInTheDocument();
    expect(screen.queryByText("Temporary Local")).not.toBeInTheDocument();
  });

  it("saves multiple models for one channel from settings", async () => {
    window.history.pushState({}, "", "/settings");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels" && (!init || init.method === undefined)) {
          return jsonResponse({ ok: true, data: { channels: [] } });
        }
        if (input === "/api/channels" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              channel: {
                id: "chn_1",
                name: "Router",
                baseUrl: "https://router.example.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  { id: "gpt-4o", capabilities: ["text-to-image"] },
                ],
              },
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("供应源名称"), {
      target: { value: "Router" },
    });
    fireEvent.change(screen.getByLabelText("Base URL"), {
      target: { value: "https://router.example.com" },
    });
    fireEvent.change(screen.getByLabelText("API Key"), {
      target: { value: "sk-secret" },
    });
    fireEvent.change(screen.getByLabelText("模型 ID"), {
      target: { value: "gpt-image-1" },
    });
    await userEvent.click(screen.getByRole("button", { name: "加入模型" }));
    await userEvent.clear(screen.getByLabelText("模型 ID"));
    await userEvent.type(screen.getByLabelText("模型 ID"), "gpt-4o");
    await userEvent.click(screen.getByRole("button", { name: "加入模型" }));
    await userEvent.click(screen.getByRole("button", { name: "保存供应源" }));

    await waitFor(() => {
      const postCall = fetchMock.mock.calls.find(
        ([input, init]) => input === "/api/channels" && init?.method === "POST",
      );
      expect(postCall).toBeTruthy();
      expect(JSON.parse(String(postCall?.[1]?.body)).models).toEqual(
        expect.arrayContaining([
          { id: "gpt-image-1", capabilities: ["text-to-image"] },
          { id: "gpt-4o", capabilities: ["text-to-image"] },
        ]),
      );
    });
    expect(screen.queryByText("sk-secret")).not.toBeInTheDocument();
  });

  it("prefills the TogoAPI models with recommended capabilities", async () => {
    window.history.pushState({}, "", "/settings");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels" && (!init || init.method === undefined)) {
          return jsonResponse({ ok: true, data: { channels: [] } });
        }
        if (input === "/api/channels" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              channel: {
                id: "chn_1",
                name: "Router",
                baseUrl: "https://router.example.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-2",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("API Key"), "sk-secret");
    await userEvent.click(screen.getByRole("button", { name: "保存供应源" }));

    await waitFor(() => {
      const postCall = fetchMock.mock.calls.find(
        ([input, init]) => input === "/api/channels" && init?.method === "POST",
      );
      expect(JSON.parse(String(postCall?.[1]?.body))).toMatchObject({
        name: "TogoAPI",
        baseUrl: "https://api.togoapi.com",
        models: [
          {
            id: "gpt-image-2",
            capabilities: ["text-to-image", "image-to-image"],
          },
          { id: "gpt-5.5", capabilities: ["image-to-text"] },
          {
            id: "grok-imagine-image-2.0",
            capabilities: ["text-to-image", "image-to-image"],
          },
          { id: "grok-4.6", capabilities: ["image-to-text"] },
        ],
      });
    });
  });

  it("does not expose unsupported GPT-4o as a preset", async () => {
    window.history.pushState({}, "", "/settings");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels" && (!init || init.method === undefined)) {
          return jsonResponse({ ok: true, data: { channels: [] } });
        }
        if (input === "/api/channels" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              channel: {
                id: "chn_1",
                name: "Router",
                baseUrl: "https://router.example.com",
                hasApiKey: true,
                models: [{ id: "gpt-4o", capabilities: ["image-to-text"] }],
              },
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "GPT-4o" }),
    ).not.toBeInTheDocument();
    expect(
      fetchMock.mock.calls.some(
        ([input, init]) => input === "/api/channels" && init?.method === "POST",
      ),
    ).toBe(false);
  });

  it("groups preset models and shows each preset card capabilities", async () => {
    window.history.pushState({}, "", "/settings");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels" && (!init || init.method === undefined)) {
        return jsonResponse({ ok: true, data: { channels: [] } });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "OpenAI 生图" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "OpenAI 图生文 / 视觉理解" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Google 香蕉系列" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "豆包 / 即梦" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "GPT-5.5" })).toHaveTextContent(
      "图生文",
    );
  });

  it("only includes the supported TogoAPI preset catalog", async () => {
    window.history.pushState({}, "", "/settings");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels" && (!init || init.method === undefined)) {
        return jsonResponse({ ok: true, data: { channels: [] } });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "OpenAI 图生文 / 视觉理解" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "OpenAI 生图" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Grok 生图" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "GPT-5.5" })).toHaveTextContent(
      "图生文",
    );
    expect(
      screen.getByRole("button", { name: "GPT Image 2" }),
    ).toHaveTextContent("图生图");
    expect(
      screen.getByRole("button", { name: "Grok Imagine 2.0" }),
    ).toHaveTextContent("图生图");
    expect(
      screen.queryByRole("button", {
        name: "Nano Banana Pro / Gemini 3 Pro Image",
      }),
    ).not.toBeInTheDocument();
  });

  it("adds multiple custom model IDs from one pasted field", async () => {
    window.history.pushState({}, "", "/settings");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels" && (!init || init.method === undefined)) {
          return jsonResponse({ ok: true, data: { channels: [] } });
        }
        if (input === "/api/channels" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              channel: {
                id: "chn_1",
                name: "Router",
                baseUrl: "https://router.example.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  { id: "gpt-4o", capabilities: ["text-to-image"] },
                ],
              },
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("供应源名称"), "Router");
    await userEvent.type(
      screen.getByLabelText("Base URL"),
      "https://router.example.com",
    );
    await userEvent.type(screen.getByLabelText("API Key"), "sk-secret");
    await userEvent.type(
      screen.getByLabelText("模型 ID"),
      "gpt-image-1, gpt-4o gpt-image-1",
    );
    await userEvent.click(screen.getByRole("button", { name: "加入模型" }));
    await userEvent.click(screen.getByRole("button", { name: "保存供应源" }));

    await waitFor(() => {
      const postCall = fetchMock.mock.calls.find(
        ([input, init]) => input === "/api/channels" && init?.method === "POST",
      );
      expect(JSON.parse(String(postCall?.[1]?.body)).models).toEqual(
        expect.arrayContaining([
          { id: "gpt-image-1", capabilities: ["text-to-image"] },
          { id: "gpt-4o", capabilities: ["text-to-image"] },
        ]),
      );
    });
  });

  it("allows a prefilled TogoAPI model to be toggled", async () => {
    window.history.pushState({}, "", "/settings");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels" && (!init || init.method === undefined)) {
        return jsonResponse({ ok: true, data: { channels: [] } });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    const gpt55Card = screen.getByRole("button", { name: "GPT-5.5" });
    expect(gpt55Card).toHaveAttribute("aria-pressed", "true");

    await userEvent.click(gpt55Card);
    expect(gpt55Card).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByLabelText("gpt-5.5 图生文")).not.toBeInTheDocument();

    await userEvent.click(gpt55Card);
    expect(gpt55Card).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByLabelText("gpt-5.5 图生文")).toBeChecked();
  });

  it("lets users choose capabilities for each settings model before saving", async () => {
    window.history.pushState({}, "", "/settings");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels" && (!init || init.method === undefined)) {
          return jsonResponse({ ok: true, data: { channels: [] } });
        }
        if (input === "/api/channels" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              channel: {
                id: "chn_1",
                name: "Router",
                baseUrl: "https://router.example.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("供应源名称"), {
      target: { value: "Router" },
    });
    fireEvent.change(screen.getByLabelText("Base URL"), {
      target: { value: "https://router.example.com" },
    });
    fireEvent.change(screen.getByLabelText("API Key"), {
      target: { value: "sk-secret" },
    });
    fireEvent.change(screen.getByLabelText("模型 ID"), {
      target: { value: "gpt-image-1" },
    });
    await userEvent.click(screen.getByRole("button", { name: "加入模型" }));

    await userEvent.click(screen.getByLabelText("gpt-image-1 图生图"));
    await userEvent.click(screen.getByLabelText("gpt-image-1 图生文"));
    await userEvent.click(screen.getByRole("button", { name: "保存供应源" }));

    await waitFor(() => {
      const postCall = fetchMock.mock.calls.find(
        ([input, init]) => input === "/api/channels" && init?.method === "POST",
      );
      expect(JSON.parse(String(postCall?.[1]?.body)).models).toEqual(
        expect.arrayContaining([
          {
            id: "gpt-image-1",
            capabilities: ["text-to-image", "image-to-image", "image-to-text"],
          },
        ]),
      );
    });

    fireEvent.change(screen.getByLabelText("供应源名称"), {
      target: { value: "Router" },
    });
    fireEvent.change(screen.getByLabelText("Base URL"), {
      target: { value: "https://router.example.com" },
    });
    fireEvent.change(screen.getByLabelText("API Key"), {
      target: { value: "sk-secret" },
    });
    fireEvent.change(screen.getByLabelText("模型 ID"), {
      target: { value: "gpt-image-1" },
    });
    await userEvent.click(screen.getByRole("button", { name: "加入模型" }));
    await userEvent.click(screen.getByLabelText("gpt-image-1 文生图"));
    await userEvent.click(screen.getByRole("button", { name: "保存供应源" }));

    expect(
      await screen.findByText("每个模型至少选择一种能力"),
    ).toBeInTheDocument();
  }, 10000);

  it("exports settings channels without stored api keys", async () => {
    window.history.pushState({}, "", "/settings");
    const createObjectURL = vi
      .spyOn(URL, "createObjectURL")
      .mockReturnValue("blob:settings-export");
    const revokeObjectURL = vi
      .spyOn(URL, "revokeObjectURL")
      .mockImplementation(() => undefined);
    const anchorClick = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels" && (!init || init.method === undefined)) {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "Router",
                baseUrl: "https://router.example.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "导出供应源" }));

    const exportedBlob = createObjectURL.mock.calls[0]?.[0] as Blob;
    const exported = JSON.parse(await exportedBlob.text());
    expect(exported).toMatchObject({
      schemaVersion: 1,
      channels: [
        {
          name: "Router",
          baseUrl: "https://router.example.com",
          models: [{ id: "gpt-image-1", capabilities: ["text-to-image"] }],
        },
      ],
    });
    expect(JSON.stringify(exported)).not.toContain("apiKey");
    expect(JSON.stringify(exported)).not.toContain("sk-");
    expect(anchorClick).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:settings-export");
  });

  it("imports settings channels as sources that need api keys filled later", async () => {
    window.history.pushState({}, "", "/settings");
    let channels: Channel[] = [];
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels" && (!init || init.method === undefined)) {
          return jsonResponse({ ok: true, data: { channels } });
        }
        if (input === "/api/channels" && init?.method === "POST") {
          const body = JSON.parse(String(init.body));
          channels = [
            ...channels,
            {
              id: `chn_${channels.length + 1}`,
              name: body.name,
              baseUrl: body.baseUrl,
              hasApiKey: Boolean(body.apiKey),
              models: body.models,
            },
          ];
          return jsonResponse({
            ok: true,
            data: { channel: channels[channels.length - 1] },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "模型设置" }),
    ).toBeInTheDocument();
    const file = new File(
      [
        JSON.stringify({
          schemaVersion: 1,
          channels: [
            {
              name: "Imported Router",
              baseUrl: "https://imported.example.com",
              apiKey: "sk-should-be-ignored",
              models: [{ id: "gpt-4o", capabilities: ["image-to-text"] }],
            },
          ],
        }),
      ],
      "sources.json",
      { type: "application/json" },
    );

    await userEvent.upload(screen.getByLabelText("导入供应源配置"), file);

    await waitFor(() => {
      const postCall = fetchMock.mock.calls.find(
        ([input, init]) => input === "/api/channels" && init?.method === "POST",
      );
      expect(JSON.parse(String(postCall?.[1]?.body))).toMatchObject({
        name: "Imported Router",
        baseUrl: "https://imported.example.com",
        apiKey: "",
        models: [{ id: "gpt-4o", capabilities: ["image-to-text"] }],
      });
    });
    expect(await screen.findByText("Imported Router")).toBeInTheDocument();
    expect(screen.getByText("未配置密钥")).toBeInTheDocument();
    expect(
      screen.getByText("已导入 1 个供应源，请编辑补充 API Key 后再生成。"),
    );
  });

  it("lets admin list and create users", async () => {
    window.history.pushState({}, "", "/admin/users");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: {
              user: { id: "usr_admin", username: "admin", role: "admin" },
            },
          });
        }
        if (
          input === "/api/admin/users" &&
          (!init || init.method === undefined)
        ) {
          return jsonResponse({
            ok: true,
            data: {
              users: [
                {
                  id: "usr_admin",
                  username: "admin",
                  role: "admin",
                  disabledAt: 0,
                },
                {
                  id: "usr_alice",
                  username: "alice",
                  role: "user",
                  disabledAt: 0,
                },
              ],
            },
          });
        }
        if (input === "/api/admin/users" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              user: {
                id: "usr_bob",
                username: "bob",
                role: "user",
                disabledAt: 0,
              },
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "用户管理" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("alice")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("新用户名"), "bob");
    await userEvent.type(screen.getByLabelText("初始密码"), "bob-password");
    await userEvent.click(screen.getByRole("button", { name: "创建用户" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/admin/users",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
        }),
      );
    });
    expect(await screen.findByText("bob")).toBeInTheDocument();
  });

  it("hides admin users page from normal users", async () => {
    window.history.pushState({}, "", "/admin/users");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({
        ok: true,
        data: { user: { id: "usr_1", username: "alice", role: "user" } },
      }),
    );

    render(<App />);

    expect(await screen.findByText("需要管理员权限")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "用户" }),
    ).not.toBeInTheDocument();
  });

  it("submits image-to-text generation from workspace", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [{ id: "gpt-4o", capabilities: ["image-to-text"] }],
                },
              ],
            },
          });
        }
        if (input === "/api/generate/text" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              taskId: "tsk_1",
              historyId: "his_1",
              status: "完成",
              resultText: "caption result",
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "image-to-text",
    );
    await userEvent.type(screen.getByLabelText("提示词"), "describe");
    await userEvent.upload(
      screen.getByLabelText("参考图"),
      new File(["fake-image"], "test.png", { type: "image/png" }),
    );
    await userEvent.click(screen.getByRole("button", { name: "开始图生文" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/generate/text",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
        }),
      );
    });
    expect(await screen.findByText("caption result")).toBeInTheDocument();
  });

  it("submits workspace generation when pressing Enter in the prompt box", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/history" || input === "/api/tasks") {
          return jsonResponse({
            ok: true,
            data: input === "/api/history" ? { records: [] } : { tasks: [] },
          });
        }
        if (input === "/api/generate/image" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              taskId: "tsk_1",
              historyId: "his_1",
              status: "完成",
              files: [],
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "draw by enter");
    await userEvent.keyboard("{Enter}");

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/generate/image",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
        }),
      );
    });
  });

  it("keeps Shift Enter as a newline in the prompt box", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/history" || input === "/api/tasks") {
          return jsonResponse({
            ok: true,
            data: input === "/api/history" ? { records: [] } : { tasks: [] },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "line one");
    await userEvent.keyboard("{Shift>}{Enter}{/Shift}line two");

    expect(screen.getByLabelText("提示词")).toHaveValue("line one\nline two");
    expect(fetchMock).not.toHaveBeenCalledWith(
      "/api/generate/image",
      expect.objectContaining({
        method: "POST",
      }),
    );
  });

  it("switches image-to-text mode away from image-only models when a vision model is available", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "Mixed",
                baseUrl: "https://api.example.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-2",
                    capabilities: [
                      "text-to-image",
                      "image-to-image",
                      "image-to-text",
                    ],
                  },
                  {
                    id: "gpt-4o",
                    capabilities: [
                      "text-to-image",
                      "image-to-image",
                      "image-to-text",
                    ],
                  },
                ],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByLabelText("模型")).toHaveValue("gpt-image-2"),
    );

    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "image-to-text",
    );

    await waitFor(() =>
      expect(screen.getByLabelText("模型")).toHaveValue("gpt-4o"),
    );
  });

  it("blocks image-to-text submissions with likely image-only models", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "Misconfigured",
                  baseUrl: "https://api.example.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-2", capabilities: ["image-to-text"] },
                  ],
                },
              ],
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "image-to-text",
    );
    await userEvent.type(screen.getByLabelText("提示词"), "describe");
    await userEvent.upload(
      screen.getByLabelText("参考图"),
      new File(["fake-image"], "bad.png", { type: "image/png" }),
    );
    await userEvent.click(screen.getByRole("button", { name: "开始图生文" }));

    expect(await screen.findByText(/图生文需要视觉理解/)).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalledWith(
      "/api/generate/text",
      expect.objectContaining({
        method: "POST",
      }),
    );
  });

  it("shows a prominent reference image uploader in image-to-text mode", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [{ id: "gpt-4o", capabilities: ["image-to-text"] }],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "image-to-text",
    );

    expect(screen.getByText("参考图")).toBeInTheDocument();
    expect(screen.getByLabelText("参考图")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "开始图生文" }),
    ).toBeInTheDocument();
  });

  it("shows and clears a local reference image preview", async () => {
    window.history.pushState({}, "", "/");
    const createObjectURL = vi
      .spyOn(URL, "createObjectURL")
      .mockReturnValue("blob:reference-preview");
    const revokeObjectURL = vi
      .spyOn(URL, "revokeObjectURL")
      .mockImplementation(() => undefined);
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["image-to-image"] },
                ],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    const referenceInput = screen.getByLabelText("参考图") as HTMLInputElement;
    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "image-to-image",
    );
    await userEvent.upload(
      referenceInput,
      new File(["fake-image"], "reference.png", { type: "image/png" }),
    );

    expect(screen.getByRole("img", { name: "参考图预览" })).toHaveAttribute(
      "src",
      "blob:reference-preview",
    );
    expect(screen.getByText("reference.png")).toBeInTheDocument();
    expect(createObjectURL).toHaveBeenCalledTimes(1);

    await userEvent.click(screen.getByRole("button", { name: "清除参考图" }));

    expect(
      screen.queryByRole("img", { name: "参考图预览" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("reference.png")).not.toBeInTheDocument();
    expect(screen.getByText("参考图")).toBeInTheDocument();
    expect(referenceInput.value).toBe("");
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:reference-preview");
  });

  it("uses pasted clipboard images as the composer reference image", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:pasted-reference");
    vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => undefined);
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-1",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history" || input === "/api/tasks") {
        return jsonResponse({
          ok: true,
          data: input === "/api/history" ? { records: [] } : { tasks: [] },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    const pastedImage = new File(["pasted-image"], "clipboard.png", {
      type: "image/png",
    });
    fireEvent.paste(screen.getByLabelText("提示词"), {
      clipboardData: {
        files: [pastedImage],
        items: [],
      },
    });

    expect(
      await screen.findByRole("img", { name: "参考图预览" }),
    ).toHaveAttribute("src", "blob:pasted-reference");
    expect(screen.getByText("clipboard.png")).toBeInTheDocument();
    expect(screen.getByLabelText("模式")).toHaveValue("image-to-image");
  });

  it("clears the reference image when switching back to text-to-image mode", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:mode-preview");
    vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => undefined);
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-1",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    const referenceInput = screen.getByLabelText("参考图") as HTMLInputElement;
    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "image-to-image",
    );
    await userEvent.upload(
      referenceInput,
      new File(["fake-image"], "sticky.png", { type: "image/png" }),
    );
    expect(screen.getByText("sticky.png")).toBeInTheDocument();

    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "text-to-image",
    );

    expect(screen.queryByText("sticky.png")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("img", { name: "参考图预览" }),
    ).not.toBeInTheDocument();
    expect(referenceInput.value).toBe("");
  });

  it("submits image-to-image generation with a reference image", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["image-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/generate/image" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              taskId: "tsk_1",
              historyId: "his_1",
              status: "完成",
              files: [
                {
                  id: "fil_1",
                  storageProvider: "aliyun-oss",
                  bucket: "whalesing-web",
                  objectKey: "aiImg/result.png",
                  mediaType: "image",
                  mimeType: "image/png",
                  sizeBytes: 3,
                  createdAt: 1,
                },
              ],
            },
          });
        }
        if (input === "/api/files/fil_1/signed-url") {
          return jsonResponse({
            ok: true,
            data: { url: "https://signed.example/edit.png" },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "image-to-image",
    );
    await userEvent.type(screen.getByLabelText("提示词"), "make it cinematic");
    await userEvent.upload(
      screen.getByLabelText("参考图"),
      new File(["fake-image"], "ref.png", { type: "image/png" }),
    );
    await userEvent.click(screen.getByRole("button", { name: "开始图生图" }));

    await waitFor(() => {
      const generateCall = fetchMock.mock.calls.find(
        ([input, init]) =>
          input === "/api/generate/image" && init?.method === "POST",
      );
      expect(generateCall?.[1]?.body).toBeInstanceOf(FormData);
    });
    expect(
      await screen.findByRole("img", { name: "make it cinematic" }),
    ).toHaveAttribute("src", "https://signed.example/edit.png");
  });

  it("submits text-to-image mode with an attached reference image as image-to-image", async () => {
    window.history.pushState({}, "", "/");
    let submittedForm: FormData | undefined;
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    {
                      id: "gpt-image-1",
                      capabilities: ["text-to-image", "image-to-image"],
                    },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/generate/image" && init?.body instanceof FormData) {
          submittedForm = init.body;
          return jsonResponse({
            ok: true,
            data: {
              taskId: "tsk_1",
              historyId: "his_1",
              status: "完成",
              files: [],
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("模式")).toHaveValue("text-to-image");
    await userEvent.type(screen.getByLabelText("提示词"), "keep the pose");
    await userEvent.upload(
      screen.getByLabelText("参考图"),
      new File(["fake-image"], "ref.png", { type: "image/png" }),
    );
    await userEvent.click(screen.getByRole("button", { name: "开始图生图" }));

    await waitFor(() => expect(submittedForm).toBeDefined());
    expect(
      fetchMock.mock.calls.some(([, init]) => init?.body instanceof FormData),
    ).toBe(true);
    expect(submittedForm?.get("inputImage")).toBeInstanceOf(File);
    expect(submittedForm?.get("prompt")).toBe("keep the pose");
  });

  it("submits text-to-image generation from workspace", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/generate/image" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              taskId: "tsk_1",
              historyId: "his_1",
              status: "完成",
              files: [
                {
                  id: "fil_1",
                  storageProvider: "aliyun-oss",
                  bucket: "whalesing-web",
                  objectKey:
                    "aiImg/users/usr_1/OpenAI/gpt-image-1/2026-07-07/153022-tsk_1-0.png",
                  mediaType: "image",
                  mimeType: "image/png",
                  sizeBytes: 3,
                  createdAt: 1,
                },
              ],
            },
          });
        }
        if (input === "/api/files/fil_1/signed-url") {
          return jsonResponse({
            ok: true,
            data: { url: "https://signed.example/generated.png" },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "draw a whale");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/generate/image",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
        }),
      );
    });
    expect(
      await screen.findByRole("img", { name: "draw a whale" }),
    ).toHaveAttribute("src", "https://signed.example/generated.png");
  });

  it("continues as text-to-image when image-to-image mode has no reference image", async () => {
    window.history.pushState({}, "", "/");
    let submittedBody: BodyInit | null | undefined;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-2",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history" || input === "/api/tasks") {
        return jsonResponse({
          ok: true,
          data: input === "/api/history" ? { records: [] } : { tasks: [] },
        });
      }
      if (input === "/api/generate/image" && init?.method === "POST") {
        submittedBody = init.body;
        return jsonResponse({
          ok: true,
          data: {
            taskId: "tsk_1",
            historyId: "his_1",
            status: "完成",
            files: [],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "image-to-image",
    );
    expect(
      screen.getByRole("button", { name: "开始文生图" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "小猫");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    await waitFor(() => expect(submittedBody).toEqual(expect.any(String)));
    expect(JSON.parse(String(submittedBody))).toMatchObject({
      prompt: "小猫",
      modelId: "gpt-image-2",
    });
    expect(screen.queryByText("请上传参考图")).not.toBeInTheDocument();
  });

  it("loads recent image history into the workspace artwork feed", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_old",
                taskId: "tsk_old",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "old whale",
                parameters: {},
                durationMs: 12345,
                files: [
                  {
                    id: "fil_old",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/old.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_old/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/old.png" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("img", { name: "old whale" }),
    ).toHaveAttribute("src", "https://signed.example/old.png");
    expect(screen.queryByText("等待第一束灵感")).not.toBeInTheDocument();
    expect(screen.getByText("OpenAI / gpt-image-1")).toBeInTheDocument();
    expect(screen.getByText("耗时 12秒")).toBeInTheDocument();
  });

  it("shows reference images before generated images in the workspace feed", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-2",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_edit",
                taskId: "tsk_edit",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-2",
                capability: "image-to-image",
                prompt: "change the color",
                parameters: {},
                files: [
                  {
                    id: "fil_result",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/result.png",
                    role: "result",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 2,
                  },
                  {
                    id: "fil_ref",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/reference.png",
                    role: "reference",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 10,
                    createdAt: 1,
                  },
                ],
                createdAt: 2,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_ref/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/reference.png" },
        });
      }
      if (input === "/api/files/fil_result/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/result.png" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    const labels = await screen.findAllByText(/^(垫图|作品 1)$/);
    expect(labels.map((label) => label.textContent)).toEqual([
      "垫图",
      "作品 1",
    ]);
    expect(
      screen.getByRole("img", { name: "垫图 change the color" }),
    ).toHaveAttribute("src", "https://signed.example/reference.png");
    expect(
      screen.getByRole("img", { name: "作品 1 change the color" }),
    ).toHaveAttribute("src", "https://signed.example/result.png");
  });

  it("flags image-to-image workspace records that are missing the original reference image", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-2", capabilities: ["image-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_old_edit",
                taskId: "tsk_old_edit",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-2",
                capability: "image-to-image",
                prompt: "old edit",
                parameters: {},
                files: [
                  {
                    id: "fil_result",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/result.png",
                    role: "result",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 2,
                  },
                ],
                createdAt: 2,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_result/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/result.png" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(await screen.findByText("缺少原始垫图")).toBeInTheDocument();
    expect(
      screen.getByText("这条图生图记录没有保存原图，只能看到生成图。"),
    ).toBeInTheDocument();
  });

  it("scrolls to the newest workspace artwork when recent history initializes", async () => {
    window.history.pushState({}, "", "/");
    const scrollIntoView = vi.fn();
    window.HTMLElement.prototype.scrollIntoView = scrollIntoView;
    vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => {
      callback(0);
      return 1;
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_recent",
                taskId: "tsk_recent",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "latest whale",
                parameters: {},
                files: [],
                createdAt: 2,
              },
            ],
          },
        });
      }
      if (input === "/api/tasks") {
        return jsonResponse({ ok: true, data: { tasks: [] } });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(await screen.findByText("latest whale")).toBeInTheDocument();
    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalledWith({
        behavior: "auto",
        block: "end",
      });
    });
    await waitFor(
      () => {
        expect(scrollIntoView).toHaveBeenCalledTimes(3);
      },
      { timeout: 1400 },
    );
  });

  it("reserves enough bottom space for the fixed workspace composer", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(
      window.HTMLFormElement.prototype,
      "getBoundingClientRect",
    ).mockReturnValue({ height: 420 } as DOMRect);
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({ ok: true, data: { channels: [] } });
      }
      if (input === "/api/history") {
        return jsonResponse({ ok: true, data: { records: [] } });
      }
      if (input === "/api/tasks") {
        return jsonResponse({ ok: true, data: { tasks: [] } });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    const studioPage = await screen
      .findByRole("heading", { name: "创作工作台" })
      .then((heading) => heading.closest(".studio-page"));
    await waitFor(() => {
      expect(studioPage).toHaveStyle({ "--composer-reserved-space": "540px" });
    });
  });

  it("refreshes a workspace preview url when the thumbnail image fails", async () => {
    window.history.pushState({}, "", "/");
    let signedRequests = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_old",
                taskId: "tsk_old",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "retry whale",
                parameters: {},
                files: [
                  {
                    id: "fil_retry",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/retry.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_retry/signed-url") {
        signedRequests += 1;
        return jsonResponse({
          ok: true,
          data: {
            url:
              signedRequests === 1
                ? "https://signed.example/stale.png"
                : "https://signed.example/fresh.png",
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    const image = await screen.findByRole("img", { name: "retry whale" });
    expect(image).toHaveAttribute("src", "https://signed.example/stale.png");

    fireEvent.error(image);

    await waitFor(() => {
      expect(screen.getByRole("img", { name: "retry whale" })).toHaveAttribute(
        "src",
        "https://signed.example/fresh.png",
      );
    });
    expect(signedRequests).toBe(2);
  });

  it("keeps failed image history out of the workspace artwork feed", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_failed",
                taskId: "tsk_failed",
                kind: "image",
                status: "失败",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "broken whale",
                parameters: {},
                failureReason: "upstream error",
                files: [],
                createdAt: 2,
              },
              {
                id: "his_done",
                taskId: "tsk_done",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "done whale",
                parameters: {},
                files: [],
                createdAt: 1,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(await screen.findByText("done whale")).toBeInTheDocument();
    expect(screen.queryByText("broken whale")).not.toBeInTheDocument();
    expect(screen.queryByText("upstream error")).not.toBeInTheDocument();
  });

  it("shows inline success after copying a workspace image", async () => {
    window.history.pushState({}, "", "/");
    const clipboardWrite = vi.fn().mockResolvedValue(undefined);
    class TestClipboardItem {
      items: Record<string, Blob>;
      constructor(items: Record<string, Blob>) {
        this.items = items;
      }
    }
    Object.defineProperty(navigator, "clipboard", {
      value: { write: clipboardWrite },
      configurable: true,
    });
    vi.stubGlobal("ClipboardItem", TestClipboardItem);
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/tasks") {
        return jsonResponse({ ok: true, data: { tasks: [] } });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "workspace copy",
                parameters: {},
                files: [
                  {
                    id: "fil_1",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/workspace-copy.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_1/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/workspace-copy.png" },
        });
      }
      if (input === "/api/files/fil_1/content") {
        return imageResponse(new Uint8Array([1, 2, 3]), "image/png");
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    await userEvent.click(
      await screen.findByRole("button", { name: "复制图片 workspace copy" }),
    );

    expect(clipboardWrite).toHaveBeenCalledTimes(1);
    expect(
      await screen.findByRole("button", { name: "已复制 workspace copy" }),
    ).toBeInTheDocument();
  });

  it("offers supported output sizes and sends the selected size", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({ ok: true, data: { records: [] } });
        }
        if (input === "/api/generate/image" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              taskId: "tsk_1",
              historyId: "his_1",
              status: "完成",
              files: [],
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("质量")).not.toBeInTheDocument();
    expect(screen.getByLabelText("尺寸")).not.toHaveTextContent("4K");
    await userEvent.selectOptions(screen.getByLabelText("尺寸"), "1536x1024");
    expect(screen.getByLabelText("尺寸")).toHaveValue("1536x1024");
    await userEvent.type(screen.getByLabelText("提示词"), "wide whale");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    await waitFor(() => {
      const generateCall = fetchMock.mock.calls.find(
        ([input]) => input === "/api/generate/image",
      );
      expect(generateCall?.[1]?.body).toContain('"size":"1536x1024"');
      expect(generateCall?.[1]?.body).not.toContain('"quality"');
    });
  });

  it("submits Grok text-to-image with aspect ratio, resolution, and quality", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_grok",
                  name: "TogoAPI",
                  baseUrl: "https://api.togoapi.com",
                  hasApiKey: true,
                  models: [
                    {
                      id: "grok-imagine-image-2.0",
                      capabilities: ["text-to-image", "image-to-image"],
                    },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({ ok: true, data: { records: [] } });
        }
        if (input === "/api/generate/image" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              taskId: "tsk_grok",
              historyId: "his_grok",
              status: "完成",
              files: [],
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    expect(await screen.findByLabelText("比例")).toHaveValue("1:1");
    expect(screen.queryByLabelText("尺寸")).not.toBeInTheDocument();
    expect(screen.getByLabelText("清晰度")).toHaveValue("1k");
    expect(screen.getByLabelText("质量")).toHaveValue("low");
    await userEvent.selectOptions(screen.getByLabelText("比例"), "16:9");
    await userEvent.selectOptions(screen.getByLabelText("清晰度"), "1k");
    await userEvent.selectOptions(screen.getByLabelText("质量"), "low");
    await userEvent.type(screen.getByLabelText("提示词"), "neon city");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    await waitFor(() => {
      const generateCall = fetchMock.mock.calls.find(
        ([input]) => input === "/api/generate/image",
      );
      const body = String(generateCall?.[1]?.body);
      expect(body).toContain('"aspectRatio":"16:9"');
      expect(body).toContain('"resolution":"1k"');
      expect(body).toContain('"quality":"low"');
      expect(body).not.toContain('"size"');
    });
  });

  it("submits Grok image-to-image without quality and defaults aspect to auto", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_grok",
                  name: "TogoAPI",
                  baseUrl: "https://api.togoapi.com",
                  hasApiKey: true,
                  models: [
                    {
                      id: "grok-imagine-image-2.0",
                      capabilities: ["text-to-image", "image-to-image"],
                    },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({ ok: true, data: { records: [] } });
        }
        if (input === "/api/generate/image" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              taskId: "tsk_grok_edit",
              historyId: "his_grok_edit",
              status: "完成",
              files: [],
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "image-to-image",
    );
    await userEvent.type(screen.getByLabelText("提示词"), "keep the subject");
    await userEvent.upload(
      screen.getByLabelText("参考图"),
      new File(["fake-image"], "ref.png", { type: "image/png" }),
    );
    expect(screen.getByLabelText("比例")).toHaveValue("auto");
    expect(screen.getByLabelText("清晰度")).toBeInTheDocument();
    expect(screen.queryByLabelText("质量")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "开始图生图" }));

    await waitFor(() => {
      const generateCall = fetchMock.mock.calls.find(
        ([input, init]) =>
          input === "/api/generate/image" && init?.method === "POST",
      );
      expect(generateCall?.[1]?.body).toBeInstanceOf(FormData);
      const form = generateCall?.[1]?.body as FormData;
      expect(form.get("aspectRatio")).toBe("auto");
      expect(form.get("resolution")).toBe("1k");
      expect(form.get("quality")).toBeNull();
      expect(form.get("size")).toBeNull();
    });
  });

  it("submits workspace image generation without a count selector", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({ ok: true, data: { records: [] } });
        }
        if (input === "/api/generate/image" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              taskId: "tsk_1",
              historyId: "his_1",
              status: "完成",
              files: [],
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("数量")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("自定义数量")).not.toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "three whales");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    await waitFor(() => {
      const generateCall = fetchMock.mock.calls.find(
        ([input]) => input === "/api/generate/image",
      );
      expect(generateCall?.[1]?.body).not.toContain('"count"');
    });
  });

  it("opens workspace artwork previews in a lightbox", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_old",
                taskId: "tsk_old",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "preview whale",
                parameters: {},
                files: [
                  {
                    id: "fil_old",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/preview.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_old/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/preview.png" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.click(
      await screen.findByRole("button", { name: "打开作品预览 preview whale" }),
    );

    const dialog = await screen.findByRole("dialog", { name: "图片预览" });
    expect(
      within(dialog).getByRole("img", { name: "preview whale" }),
    ).toHaveAttribute("src", "https://signed.example/preview.png");
    expect(
      within(dialog).getByRole("link", { name: "打开原图" }),
    ).toHaveAttribute("href", "https://signed.example/preview.png");
  });

  it("deletes a workspace artwork feed item", async () => {
    window.history.pushState({}, "", "/");
    const confirmSpy = vi.spyOn(window, "confirm");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({
            ok: true,
            data: {
              records: [
                {
                  id: "his_old",
                  taskId: "tsk_old",
                  kind: "image",
                  status: "完成",
                  channelName: "OpenAI",
                  modelId: "gpt-image-1",
                  capability: "text-to-image",
                  prompt: "delete whale",
                  parameters: {},
                  files: [],
                  createdAt: 1,
                },
              ],
            },
          });
        }
        if (input === "/api/history/his_old" && init?.method === "DELETE") {
          return jsonResponse({ ok: true, data: {} });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(await screen.findByText("delete whale")).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: "删除作品 delete whale" }),
    );

    expect(confirmSpy).not.toHaveBeenCalled();
    const workspaceConfirm = screen.getByRole("region", {
      name: "确认删除作品",
    });
    expect(
      within(workspaceConfirm).getByText("确认删除作品？"),
    ).toBeInTheDocument();
    expect(
      within(workspaceConfirm).getByText("delete whale"),
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalledWith(
      "/api/history/his_old",
      expect.objectContaining({ method: "DELETE", credentials: "include" }),
    );
    await userEvent.click(screen.getByRole("button", { name: "确认删除作品" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/history/his_old",
        expect.objectContaining({ method: "DELETE", credentials: "include" }),
      );
    });
    expect(screen.queryByText("delete whale")).not.toBeInTheDocument();
    expect(screen.getByText("作品已删除")).toBeInTheDocument();
  });

  it("reuses a recent artwork feed item in the workspace composer", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_old",
                taskId: "tsk_old",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "reuse from feed",
                parameters: { size: "1536x1024", count: 2 },
                files: [],
                createdAt: 1,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("reuse from feed")).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: "复用作品 reuse from feed" }),
    );

    expect(screen.getByLabelText("提示词")).toHaveValue("reuse from feed");
    expect(screen.getByLabelText("模式")).toHaveValue("text-to-image");
    expect(screen.getByDisplayValue("1536x1024")).toBeInTheDocument();
    expect(screen.queryByLabelText("数量")).not.toBeInTheDocument();
  });

  it("brings a recent artwork feed item back to the composer for another generation", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_old",
                taskId: "tsk_old",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "again from feed",
                parameters: { size: "1024x1536", count: 4 },
                files: [],
                createdAt: 1,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(await screen.findByText("again from feed")).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: "再次生成作品 again from feed" }),
    );

    expect(screen.getByLabelText("提示词")).toHaveValue("again from feed");
    expect(screen.getByLabelText("模式")).toHaveValue("text-to-image");
    expect(screen.getByDisplayValue("1024x1536")).toBeInTheDocument();
    expect(screen.queryByLabelText("数量")).not.toBeInTheDocument();
    expect(
      screen.getByText("已从作品带入创作参数，可继续调整后再次生成。"),
    ).toBeInTheDocument();
  });

  it("warns that image-to-text artwork feed items need a fresh reference image before regenerating", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [{ id: "gpt-4o", capabilities: ["image-to-text"] }],
                },
              ],
            },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({
            ok: true,
            data: {
              records: [
                {
                  id: "his_old",
                  taskId: "tsk_old",
                  kind: "text",
                  status: "完成",
                  channelName: "OpenAI",
                  modelId: "gpt-4o",
                  capability: "image-to-text",
                  prompt: "describe feed image",
                  parameters: {},
                  resultText: "A compact product shot.",
                  files: [],
                  createdAt: 1,
                },
              ],
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(await screen.findByText("describe feed image")).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: "再次生成作品 describe feed image" }),
    );

    expect(screen.getByLabelText("提示词")).toHaveValue("describe feed image");
    expect(screen.getByLabelText("模式")).toHaveValue("image-to-text");
    expect(
      screen.getByText("图生文记录需要重新上传参考图后再生成。"),
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalledWith(
      "/api/generate/text",
      expect.anything(),
    );
  });

  it("uses the latest image-to-image artwork file as the next reference image", async () => {
    window.history.pushState({}, "", "/");
    let submittedForm: FormData | undefined;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["image-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_edit",
                taskId: "tsk_edit",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "image-to-image",
                prompt: "make it brighter",
                parameters: { size: "1024x1024", count: 1 },
                files: [
                  {
                    id: "fil_ref",
                    storageProvider: "aliyun-oss",
                    bucket: "bucket",
                    objectKey: "aiImg/result.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 9,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_ref/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/result.png" },
        });
      }
      if (input === "/api/files/fil_ref/content") {
        return imageResponse("ref-bytes");
      }
      if (input === "/api/generate/image" && init?.body instanceof FormData) {
        submittedForm = init.body;
        return jsonResponse({
          ok: true,
          data: { taskId: "tsk_new", historyId: "his_new", status: "running" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(await screen.findByText("make it brighter")).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: "再次生成作品 make it brighter" }),
    );

    expect(await screen.findByText("result.png")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "开始图生图" }));

    await waitFor(() => expect(submittedForm).toBeDefined());
    const referenceFile = submittedForm?.get("inputImage");
    expect(referenceFile).toBeInstanceOf(File);
    expect((referenceFile as File).name).toBe("result.png");
    expect(await (referenceFile as File).text()).toBe("ref-bytes");
  });

  it("sets any workspace artwork image as an image-to-image reference", async () => {
    window.history.pushState({}, "", "/");
    let submittedForm: FormData | undefined;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-2",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_base",
                taskId: "tsk_base",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-2",
                capability: "text-to-image",
                prompt: "make base",
                parameters: { size: "1024x1024", count: 1 },
                files: [
                  {
                    id: "fil_base",
                    storageProvider: "aliyun-oss",
                    bucket: "bucket",
                    objectKey: "aiImg/base.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 9,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_base/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/base.png" },
        });
      }
      if (input === "/api/files/fil_base/content") {
        return imageResponse("base-bytes");
      }
      if (input === "/api/generate/image" && init?.body instanceof FormData) {
        submittedForm = init.body;
        return jsonResponse({
          ok: true,
          data: { taskId: "tsk_new", historyId: "his_new", status: "running" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(await screen.findByText("make base")).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: "将作品 make base 1 设为参考图" }),
    );

    expect(await screen.findByText("base.png")).toBeInTheDocument();
    expect(screen.getByLabelText("模式")).toHaveValue("image-to-image");
    await userEvent.click(screen.getByRole("button", { name: "开始图生图" }));

    await waitFor(() => expect(submittedForm).toBeDefined());
    const referenceFile = submittedForm?.get("inputImage");
    expect(referenceFile).toBeInstanceOf(File);
    expect((referenceFile as File).name).toBe("base.png");
    expect(await (referenceFile as File).text()).toBe("base-bytes");
  });

  it("adds newly generated artwork to the bottom of the workspace feed near the composer", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({
            ok: true,
            data: {
              records: [
                {
                  id: "his_old",
                  taskId: "tsk_old",
                  kind: "image",
                  status: "完成",
                  channelName: "OpenAI",
                  modelId: "gpt-image-1",
                  capability: "text-to-image",
                  prompt: "old whale",
                  parameters: {},
                  files: [],
                  createdAt: 1,
                },
              ],
            },
          });
        }
        if (input === "/api/generate/image" && init?.method === "POST") {
          return jsonResponse({
            ok: true,
            data: {
              taskId: "tsk_new",
              historyId: "his_new",
              status: "完成",
              files: [
                {
                  id: "fil_new",
                  storageProvider: "aliyun-oss",
                  bucket: "whalesing-web",
                  objectKey: "aiImg/new.png",
                  mediaType: "image",
                  mimeType: "image/png",
                  sizeBytes: 3,
                  createdAt: 2,
                },
              ],
            },
          });
        }
        if (input === "/api/files/fil_new/signed-url") {
          return jsonResponse({
            ok: true,
            data: { url: "https://signed.example/new.png" },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "new whale");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    expect(
      await screen.findByRole("img", { name: "new whale" }),
    ).toHaveAttribute("src", "https://signed.example/new.png");
    await waitFor(() => {
      const feedCards = Array.from(document.querySelectorAll(".artwork-card"));
      expect(feedCards[0]).toHaveTextContent("old whale");
      expect(feedCards[1]).toHaveTextContent("new whale");
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/generate/image",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
  });

  it("remembers the last workspace composer selections for the signed in user", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-1",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
              {
                id: "chn_2",
                name: "Router",
                baseUrl: "https://router.example.com",
                hasApiKey: true,
                models: [
                  {
                    id: "seedream",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({ ok: true, data: { records: [] } });
      }
      return jsonResponse({ ok: false }, 404);
    });

    const firstRender = render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByLabelText("供应源")).toHaveValue("chn_1"),
    );
    await userEvent.selectOptions(screen.getByLabelText("供应源"), "chn_2");
    await waitFor(() =>
      expect(screen.getByLabelText("模型")).toHaveValue("seedream"),
    );
    await userEvent.selectOptions(
      screen.getByLabelText("模式"),
      "image-to-image",
    );
    await userEvent.selectOptions(screen.getByLabelText("尺寸"), "1536x1024");
    expect(screen.queryByLabelText("数量")).not.toBeInTheDocument();

    await waitFor(() => {
      const preferences = JSON.parse(
        window.localStorage.getItem("imgtool:composer-preferences:alice") ??
          "{}",
      );
      expect(preferences).toMatchObject({
        channelId: "chn_2",
        modelId: "seedream",
        capability: "image-to-image",
        size: "1536x1024",
      });
      expect(preferences).not.toHaveProperty("count");
      expect(preferences).not.toHaveProperty("quality");
    });

    firstRender.unmount();
    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByLabelText("供应源")).toHaveValue("chn_2");
      expect(screen.getByLabelText("模型")).toHaveValue("seedream");
      expect(screen.getByLabelText("模式")).toHaveValue("image-to-image");
      expect(screen.getByLabelText("尺寸")).toHaveValue("1536x1024");
      expect(screen.queryByLabelText("数量")).not.toBeInTheDocument();
    });
  });

  it("shows a running task card while workspace generation is pending", async () => {
    window.history.pushState({}, "", "/");
    let resolveGenerate!: (response: Response) => void;
    const generatePromise = new Promise<Response>((resolve) => {
      resolveGenerate = resolve;
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({ ok: true, data: { records: [] } });
      }
      if (input === "/api/generate/image" && init?.method === "POST") {
        return generatePromise;
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "pending whale");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    const runningTask = await screen.findByRole("article", {
      name: "运行中任务 pending whale",
    });
    expect(runningTask).toHaveTextContent("pending whale");
    expect(runningTask).toHaveTextContent("OpenAI / gpt-image-1");
    expect(runningTask).toHaveTextContent("文生图");
    expect(runningTask).toHaveTextContent("已用时");
    expect(runningTask).toHaveTextContent("正在生成画面");
    expect(runningTask).toHaveTextContent("提交请求");
    expect(runningTask).toHaveTextContent("生成中");
    expect(runningTask).toHaveTextContent("整理作品");

    resolveGenerate(
      jsonResponse({
        ok: true,
        data: {
          taskId: "tsk_pending",
          historyId: "his_pending",
          status: "完成",
          files: [],
        },
      }),
    );
    await waitFor(() => {
      expect(
        screen.queryByRole("article", { name: "运行中任务 pending whale" }),
      ).not.toBeInTheDocument();
    });
  });

  it("shows the attached reference image on the running generation card", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:running-reference");
    vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => undefined);
    const generatePromise = new Promise<Response>(() => {});
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-2",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history" || input === "/api/tasks") {
        return jsonResponse({
          ok: true,
          data: input === "/api/history" ? { records: [] } : { tasks: [] },
        });
      }
      if (input === "/api/generate/image" && init?.body instanceof FormData) {
        return generatePromise;
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "换一下这张图的颜色");
    await userEvent.upload(
      screen.getByLabelText("参考图"),
      new File(["fake-image"], "reference.png", { type: "image/png" }),
    );
    await userEvent.click(screen.getByRole("button", { name: "开始图生图" }));

    const runningTask = await screen.findByRole("article", {
      name: "运行中任务 换一下这张图的颜色",
    });
    const referencePreview = within(runningTask).getByRole("img", {
      name: "运行中参考图 reference.png",
    });
    expect(referencePreview).toHaveAttribute("src", "blob:running-reference");
    expect(runningTask).toHaveTextContent("上传参考图");
  });

  it("keeps a submitted image task running when the API returns before completion", async () => {
    window.history.pushState({}, "", "/");
    const scrollIntoView = vi.fn();
    window.HTMLElement.prototype.scrollIntoView = scrollIntoView;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history" || input === "/api/tasks") {
        return jsonResponse({
          ok: true,
          data: input === "/api/history" ? { records: [] } : { tasks: [] },
        });
      }
      if (input === "/api/generate/image" && init?.method === "POST") {
        return jsonResponse({
          ok: true,
          data: {
            taskId: "tsk_pending",
            historyId: "",
            status: "running",
            durationMs: 0,
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "async whale");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    const runningTask = await screen.findByRole("article", {
      name: "运行中任务 async whale",
    });
    expect(runningTask).toHaveTextContent("async whale");
    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalledWith({
        behavior: "smooth",
        block: "end",
      });
    });
    expect(
      screen.queryByRole("article", { name: "作品 async whale" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "开始文生图" })).toBeEnabled();
  });

  it("hydrates a submitted workspace task with a server id so it can be cancelled immediately", async () => {
    window.history.pushState({}, "", "/");
    let generationSubmitted = false;
    const generatePromise = new Promise<Response>(() => {});
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({ ok: true, data: { records: [] } });
      }
      if (input === "/api/tasks") {
        if (!generationSubmitted) {
          return jsonResponse({ ok: true, data: { tasks: [] } });
        }
        const startedAt = Date.now() - 500;
        return jsonResponse({
          ok: true,
          data: {
            tasks: [
              {
                id: "tsk_fast_cancel",
                kind: "image",
                status: "running",
                channelId: "chn_1",
                channelName: "OpenAI",
                baseUrl: "https://api.openai.com",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "fast cancel whale",
                parameters: { size: "1024x1024" },
                startedAt,
                createdAt: startedAt,
                updatedAt: startedAt,
              },
            ],
          },
        });
      }
      if (input === "/api/generate/image" && init?.method === "POST") {
        generationSubmitted = true;
        return generatePromise;
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "fast cancel whale");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    expect(
      await screen.findByRole(
        "button",
        { name: "取消任务 fast cancel whale" },
        { timeout: 1000 },
      ),
    ).toBeInTheDocument();
  });

  it("loads running workspace tasks from the server", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({ ok: true, data: { records: [] } });
        }
        if (input === "/api/tasks") {
          const startedAt = Date.now() - 3000;
          return jsonResponse({
            ok: true,
            data: {
              tasks: [
                {
                  id: "tsk_running",
                  kind: "image",
                  status: "running",
                  channelId: "chn_1",
                  channelName: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  modelId: "gpt-image-1",
                  capability: "text-to-image",
                  prompt: "server whale",
                  parameters: { size: "1024x1024" },
                  startedAt,
                  createdAt: startedAt,
                  updatedAt: startedAt,
                },
              ],
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    const runningTask = await screen.findByRole("article", {
      name: "运行中任务 server whale",
    });
    expect(runningTask).toHaveTextContent("server whale");
    expect(runningTask).toHaveTextContent("OpenAI / gpt-image-1");
    expect(runningTask).toHaveTextContent("文生图");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/tasks",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("polls running workspace tasks and refreshes the feed after completion", async () => {
    window.history.pushState({}, "", "/");
    let taskCalls = 0;
    let historyCalls = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/tasks") {
        taskCalls += 1;
        if (taskCalls === 1) {
          const startedAt = Date.now() - 3000;
          return jsonResponse({
            ok: true,
            data: {
              tasks: [
                {
                  id: "tsk_running",
                  kind: "image",
                  status: "running",
                  channelId: "chn_1",
                  channelName: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  modelId: "gpt-image-1",
                  capability: "text-to-image",
                  prompt: "server whale",
                  parameters: { size: "1024x1024" },
                  startedAt,
                  createdAt: startedAt,
                  updatedAt: startedAt,
                },
              ],
            },
          });
        }
        return jsonResponse({ ok: true, data: { tasks: [] } });
      }
      if (input === "/api/history") {
        historyCalls += 1;
        return jsonResponse({
          ok: true,
          data: {
            records:
              historyCalls === 1
                ? []
                : [
                    {
                      id: "his_done",
                      taskId: "tsk_running",
                      kind: "image",
                      status: "完成",
                      channelName: "OpenAI",
                      modelId: "gpt-image-1",
                      capability: "text-to-image",
                      prompt: "server whale",
                      parameters: { size: "1024x1024" },
                      files: [],
                      createdAt: Date.now(),
                    },
                  ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("article", { name: "运行中任务 server whale" }),
    ).toBeInTheDocument();

    await waitFor(
      () => {
        expect(
          screen.queryByRole("article", { name: "运行中任务 server whale" }),
        ).not.toBeInTheDocument();
      },
      { timeout: 4000 },
    );
    expect(await screen.findByText("server whale")).toBeInTheDocument();
  });

  it("shows the backend failure reason when an asynchronous task fails", async () => {
    window.history.pushState({}, "", "/");
    let taskCalls = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "Gemini",
                baseUrl: "https://example.com",
                hasApiKey: true,
                models: [
                  { id: "gemini-image", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({ ok: true, data: { records: [] } });
      }
      if (input === "/api/tasks") {
        taskCalls += 1;
        if (taskCalls === 1) {
          const startedAt = Date.now() - 3000;
          return jsonResponse({
            ok: true,
            data: {
              tasks: [
                {
                  id: "tsk_failed",
                  kind: "image",
                  status: "running",
                  channelName: "Gemini",
                  modelId: "gemini-image",
                  capability: "text-to-image",
                  prompt: "failed whale",
                  parameters: { size: "1024x1024" },
                  startedAt,
                  createdAt: startedAt,
                  updatedAt: startedAt,
                },
              ],
            },
          });
        }
        return jsonResponse({ ok: true, data: { tasks: [] } });
      }
      if (input === "/api/tasks/tsk_failed") {
        const endedAt = Date.now();
        return jsonResponse({
          ok: true,
          data: {
            task: {
              id: "tsk_failed",
              kind: "image",
              status: "failed",
              channelName: "Gemini",
              modelId: "gemini-image",
              capability: "text-to-image",
              prompt: "failed whale",
              parameters: { size: "1024x1024" },
              startedAt: endedAt - 3000,
              endedAt,
              failureReason: "All available accounts exhausted",
              createdAt: endedAt - 3000,
              updatedAt: endedAt,
            },
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("article", { name: "运行中任务 failed whale" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole(
        "status",
        { name: "生成失败提示" },
        { timeout: 4000 },
      ),
    ).toHaveTextContent("All available accounts exhausted");
  });

  it("keeps refreshing the workspace feed when completed history is briefly delayed", async () => {
    window.history.pushState({}, "", "/");
    let taskCalls = 0;
    let historyCalls = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/tasks") {
        taskCalls += 1;
        if (taskCalls === 1) {
          const startedAt = Date.now() - 3000;
          return jsonResponse({
            ok: true,
            data: {
              tasks: [
                {
                  id: "tsk_delayed_history",
                  kind: "image",
                  status: "running",
                  channelId: "chn_1",
                  channelName: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  modelId: "gpt-image-1",
                  capability: "text-to-image",
                  prompt: "delayed history whale",
                  parameters: { size: "1024x1024" },
                  startedAt,
                  createdAt: startedAt,
                  updatedAt: startedAt,
                },
              ],
            },
          });
        }
        return jsonResponse({ ok: true, data: { tasks: [] } });
      }
      if (input === "/api/history") {
        historyCalls += 1;
        return jsonResponse({
          ok: true,
          data: {
            records:
              historyCalls <= 2
                ? []
                : [
                    {
                      id: "his_delayed_history",
                      taskId: "tsk_delayed_history",
                      kind: "image",
                      status: "完成",
                      channelName: "OpenAI",
                      modelId: "gpt-image-1",
                      capability: "text-to-image",
                      prompt: "delayed history whale",
                      parameters: { size: "1024x1024" },
                      files: [],
                      createdAt: Date.now(),
                    },
                  ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("article", {
        name: "运行中任务 delayed history whale",
      }),
    ).toBeInTheDocument();

    await waitFor(
      () => {
        expect(
          screen.queryByRole("article", {
            name: "运行中任务 delayed history whale",
          }),
        ).not.toBeInTheDocument();
      },
      { timeout: 4000 },
    );
    expect(
      await screen.findByText("delayed history whale", {}, { timeout: 7500 }),
    ).toBeInTheDocument();
  }, 12000);

  it("cancels a server-backed running workspace task", async () => {
    window.history.pushState({}, "", "/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [
                    { id: "gpt-image-1", capabilities: ["text-to-image"] },
                  ],
                },
              ],
            },
          });
        }
        if (input === "/api/tasks") {
          const startedAt = Date.now() - 3000;
          return jsonResponse({
            ok: true,
            data: {
              tasks: [
                {
                  id: "tsk_running",
                  kind: "image",
                  status: "running",
                  channelId: "chn_1",
                  channelName: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  modelId: "gpt-image-1",
                  capability: "text-to-image",
                  prompt: "cancel whale",
                  parameters: { size: "1024x1024" },
                  startedAt,
                  createdAt: startedAt,
                  updatedAt: startedAt,
                },
              ],
            },
          });
        }
        if (
          input === "/api/tasks/tsk_running/cancel" &&
          init?.method === "POST"
        ) {
          return jsonResponse({
            ok: true,
            data: {
              cancelled: true,
              record: {
                id: "his_cancelled",
                taskId: "tsk_running",
                kind: "image",
                status: "失败",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "cancel whale",
                parameters: { size: "1024x1024" },
                failureReason: "用户取消",
                files: [],
                createdAt: Date.now(),
              },
            },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({ ok: true, data: { records: [] } });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    const runningTask = await screen.findByRole("article", {
      name: "运行中任务 cancel whale",
    });
    await userEvent.click(
      within(runningTask).getByRole("button", {
        name: "取消任务 cancel whale",
      }),
    );

    await waitFor(() => {
      expect(
        screen.queryByRole("article", { name: "运行中任务 cancel whale" }),
      ).not.toBeInTheDocument();
    });
    expect(await screen.findByText("任务已取消")).toBeInTheDocument();
    expect(screen.queryByText("用户取消")).not.toBeInTheDocument();
    expect(document.querySelectorAll(".artwork-card")).toHaveLength(0);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/tasks/tsk_running/cancel",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
  });

  it("restores the workspace composer after cancelling a submitted task before the generation request returns", async () => {
    window.history.pushState({}, "", "/");
    let generationSubmitted = false;
    let resolveGenerate!: (response: Response) => void;
    const generatePromise = new Promise<Response>((resolve) => {
      resolveGenerate = resolve;
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({ ok: true, data: { records: [] } });
      }
      if (input === "/api/tasks") {
        if (!generationSubmitted) {
          return jsonResponse({ ok: true, data: { tasks: [] } });
        }
        const startedAt = Date.now() - 3000;
        return jsonResponse({
          ok: true,
          data: {
            tasks: [
              {
                id: "tsk_pending_cancel",
                kind: "image",
                status: "running",
                channelId: "chn_1",
                channelName: "OpenAI",
                baseUrl: "https://api.openai.com",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "cancel pending generate",
                parameters: { size: "1024x1024" },
                startedAt,
                createdAt: startedAt,
                updatedAt: startedAt,
              },
            ],
          },
        });
      }
      if (input === "/api/generate/image" && init?.method === "POST") {
        generationSubmitted = true;
        return generatePromise;
      }
      if (
        input === "/api/tasks/tsk_pending_cancel/cancel" &&
        init?.method === "POST"
      ) {
        return jsonResponse({
          ok: true,
          data: {
            cancelled: true,
            record: {
              id: "his_cancelled_pending",
              taskId: "tsk_pending_cancel",
              kind: "image",
              status: "失败",
              channelName: "OpenAI",
              modelId: "gpt-image-1",
              capability: "text-to-image",
              prompt: "cancel pending generate",
              parameters: { size: "1024x1024" },
              failureReason: "用户取消",
              files: [],
              createdAt: Date.now(),
            },
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(
      screen.getByLabelText("提示词"),
      "cancel pending generate",
    );
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));
    expect(
      await screen.findByText("已提交到模型，正在等待图片返回。"),
    ).toBeInTheDocument();

    const cancelButton = await screen.findByRole(
      "button",
      { name: "取消任务 cancel pending generate" },
      { timeout: 4000 },
    );
    await userEvent.click(cancelButton);

    await waitFor(() => {
      expect(
        screen.queryByText("已提交到模型，正在等待图片返回。"),
      ).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "开始文生图" })).toBeEnabled();
    expect(await screen.findByText("任务已取消")).toBeInTheDocument();

    resolveGenerate(
      jsonResponse({
        ok: true,
        data: {
          taskId: "tsk_pending_cancel",
          historyId: "his_late",
          status: "完成",
          files: [],
        },
      }),
    );
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "开始文生图" })).toBeEnabled();
    });
    expect(document.querySelectorAll(".artwork-card")).toHaveLength(0);
    expect(screen.queryByText("用户取消")).not.toBeInTheDocument();
  });

  it("renders the legacy-style studio workspace shell", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(await screen.findByText("IMAGE FORGE")).toBeInTheDocument();
    expect(screen.getByText("等待第一束灵感")).toBeInTheDocument();
    expect(screen.getByLabelText("创作输入栏")).toBeInTheDocument();
  });

  it("uses the legacy composer layout with mode in the control dock", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-1",
                    capabilities: [
                      "text-to-image",
                      "image-to-image",
                      "image-to-text",
                    ],
                  },
                ],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    const composer = screen.getByLabelText("创作输入栏");
    expect(composer.querySelector(".mode-switch")).not.toBeInTheDocument();
    expect(screen.getByLabelText("模式")).toHaveValue("text-to-image");
    expect(screen.getByLabelText("提示词")).toHaveClass("prompt-input");
    expect(screen.getByLabelText("参考图")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "开始文生图" })).toHaveClass(
      "send-orb",
    );
  });

  it("shows generation progress while waiting for upstream image", async () => {
    window.history.pushState({}, "", "/");
    let resolveGenerate: (response: Response) => void = () => {};
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/generate/image" && init?.method === "POST") {
        return new Promise<Response>((resolve) => {
          resolveGenerate = resolve;
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("提示词"), "draw");
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    expect(
      await screen.findByRole("button", { name: "生成中..." }),
    ).toBeDisabled();
    resolveGenerate(
      jsonResponse({ ok: false, error: { message: "upstream timeout" } }, 400),
    );
    expect(await screen.findByText("upstream timeout")).toBeInTheDocument();
    expect(
      await screen.findByRole("status", { name: "生成失败提示" }),
    ).toHaveTextContent("upstream timeout");
  });

  it("restores the workspace composer when the generation request fails before a response", async () => {
    window.history.pushState({}, "", "/");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      if (input === "/api/history" || input === "/api/tasks") {
        return jsonResponse({
          ok: true,
          data: input === "/api/history" ? { records: [] } : { tasks: [] },
        });
      }
      if (input === "/api/generate/image" && init?.method === "POST") {
        throw new TypeError("Failed to fetch");
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    await userEvent.type(
      screen.getByLabelText("提示词"),
      "network outage whale",
    );
    await userEvent.click(screen.getByRole("button", { name: "开始文生图" }));

    expect(
      await screen.findByText("生成请求失败，请检查网络或后端服务后重试"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("已提交到模型，正在等待图片返回。"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("article", {
        name: "运行中任务 network outage whale",
      }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "开始文生图" })).toBeEnabled();
    expect(screen.getByLabelText("提示词")).toHaveValue("network outage whale");
  });

  it("lists text history records", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "text",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-4o",
                capability: "image-to-text",
                prompt: "describe",
                parameters: {},
                resultText: "caption result",
                files: [],
                createdAt: 1,
              },
              {
                id: "his_failed",
                taskId: "tsk_failed",
                kind: "image",
                status: "失败",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "cancelled prompt",
                parameters: {},
                failureReason: "用户取消",
                files: [],
                createdAt: 2,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("caption result")).toBeInTheDocument();
    expect(screen.getByText("OpenAI / gpt-4o")).toBeInTheDocument();
    expect(screen.queryByText("cancelled prompt")).not.toBeInTheDocument();
    expect(screen.queryByText("用户取消")).not.toBeInTheDocument();
  });

  it("automatically loads signed preview urls for history files", async () => {
    window.history.pushState({}, "", "/history");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({
            ok: true,
            data: {
              records: [
                {
                  id: "his_1",
                  taskId: "tsk_1",
                  kind: "image",
                  status: "完成",
                  channelName: "OpenAI",
                  modelId: "gpt-image-1",
                  capability: "text-to-image",
                  prompt: "draw",
                  parameters: {},
                  files: [
                    {
                      id: "fil_1",
                      storageProvider: "aliyun-oss",
                      bucket: "whalesing-web",
                      objectKey:
                        "aiImg/users/usr_1/OpenAI/gpt-image-1/result.png",
                      mediaType: "image",
                      mimeType: "image/png",
                      sizeBytes: 3,
                      createdAt: 1,
                    },
                  ],
                  createdAt: 1,
                },
              ],
            },
          });
        }
        if (input === "/api/files/fil_1/signed-url") {
          return jsonResponse({
            ok: true,
            data: { url: "https://signed.example/result.png" },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/files/fil_1/signed-url",
        expect.objectContaining({ credentials: "include" }),
      );
    });
    const previewImages = await screen.findAllByRole("img", { name: "draw" });
    expect(previewImages.length).toBeGreaterThan(0);
    expect(
      previewImages.every(
        (image) =>
          image.getAttribute("src") === "https://signed.example/result.png",
      ),
    ).toBe(true);
  });

  it("copies history images to the system clipboard", async () => {
    window.history.pushState({}, "", "/history");
    const clipboardWrite = vi.fn().mockResolvedValue(undefined);
    class TestClipboardItem {
      items: Record<string, Blob>;
      constructor(items: Record<string, Blob>) {
        this.items = items;
      }
    }
    Object.defineProperty(navigator, "clipboard", {
      value: { write: clipboardWrite },
      configurable: true,
    });
    vi.stubGlobal("ClipboardItem", TestClipboardItem);
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "copy this image",
                parameters: {},
                files: [
                  {
                    id: "fil_1",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/copy.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_1/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/copy.png" },
        });
      }
      if (input === "/api/files/fil_1/download-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/copy.png?download=1" },
        });
      }
      if (input === "/api/files/fil_1/content") {
        return imageResponse(new Uint8Array([1, 2, 3]), "image/png");
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    await userEvent.click(
      await screen.findByRole("button", { name: "复制图片 copy this image" }),
    );

    expect(clipboardWrite).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("图片已复制")).toBeInTheDocument();
    expect(
      await screen.findByRole("button", { name: "已复制 copy this image" }),
    ).toBeInTheDocument();
  });

  it("shows reference images separately in history records", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-2",
                capability: "image-to-image",
                prompt: "换一下这张图的颜色",
                parameters: {},
                files: [
                  {
                    id: "fil_ref",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/reference.png",
                    role: "reference",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 10,
                    createdAt: 1,
                  },
                  {
                    id: "fil_result",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/result.png",
                    role: "result",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 2,
                  },
                ],
                createdAt: 2,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_ref/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/reference.png" },
        });
      }
      if (input === "/api/files/fil_result/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/result.png" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("垫图")).toBeInTheDocument();
    expect(screen.getByText("作品 1")).toBeInTheDocument();
    expect(
      await screen.findByRole("img", { name: "垫图 换一下这张图的颜色" }),
    ).toHaveAttribute("src", "https://signed.example/reference.png");
  });

  it("shows direct original image links on history file cards", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "draw direct original",
                parameters: {},
                files: [
                  {
                    id: "fil_1",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey:
                      "aiImg/users/usr_1/OpenAI/gpt-image-1/original.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_1/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/original.png" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    const originalLink = await screen.findByRole("link", {
      name: "打开原图 draw direct original",
    });
    expect(originalLink).toHaveAttribute(
      "href",
      "https://signed.example/original.png",
    );
    expect(originalLink).toHaveAttribute("target", "_blank");
  });

  it("shows download links on history file cards", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "draw downloadable",
                parameters: {},
                files: [
                  {
                    id: "fil_1",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey:
                      "aiImg/users/usr_1/OpenAI/gpt-image-1/downloadable.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_1/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/preview.png" },
        });
      }
      if (input === "/api/files/fil_1/download-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/download.png" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    const downloadLink = await screen.findByRole("link", {
      name: "下载图片 draw downloadable",
    });
    expect(downloadLink).toHaveAttribute(
      "href",
      "https://signed.example/download.png",
    );
    expect(downloadLink).toHaveAttribute("download", "downloadable.png");
  });

  it("keeps history usable when signed image urls fail to load", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "draw while oss is offline",
                parameters: {},
                files: [
                  {
                    id: "fil_1",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey:
                      "aiImg/users/usr_1/OpenAI/gpt-image-1/offline.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (
        input === "/api/files/fil_1/signed-url" ||
        input === "/api/files/fil_1/download-url"
      ) {
        throw new TypeError("Failed to fetch");
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByText("draw while oss is offline"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "预览" })).toBeInTheDocument();
    expect(
      screen.getByText("aiImg/users/usr_1/OpenAI/gpt-image-1/offline.png"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", {
        name: "下载图片 draw while oss is offline",
      }),
    ).not.toBeInTheDocument();
  });

  it("prepares signed download links for selected history images", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "download selected first",
                parameters: {},
                files: [
                  {
                    id: "fil_1",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/users/usr_1/OpenAI/gpt-image-1/first.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 1,
                  },
                ],
                createdAt: 3,
              },
              {
                id: "his_2",
                taskId: "tsk_2",
                kind: "text",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-4o",
                capability: "image-to-text",
                prompt: "caption only",
                parameters: {},
                resultText: "caption",
                files: [],
                createdAt: 2,
              },
              {
                id: "his_3",
                taskId: "tsk_3",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "seedream",
                capability: "text-to-image",
                prompt: "download selected second",
                parameters: {},
                files: [
                  {
                    id: "fil_2",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey: "aiImg/users/usr_1/OpenAI/seedream/second.jpg",
                    mediaType: "image",
                    mimeType: "image/jpeg",
                    sizeBytes: 4,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_1/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/first-preview.png" },
        });
      }
      if (input === "/api/files/fil_2/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/second-preview.jpg" },
        });
      }
      if (input === "/api/files/fil_1/download-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/first-download.png" },
        });
      }
      if (input === "/api/files/fil_2/download-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/second-download.jpg" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByText("download selected first"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "下载所选图片" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "选择当前筛选" }));
    const bulkDownloadButton = screen.getByRole("button", {
      name: "下载所选图片",
    });
    await waitFor(() => expect(bulkDownloadButton).toBeEnabled());
    await userEvent.click(bulkDownloadButton);

    const downloadPanel = screen.getByRole("region", { name: "批量下载图片" });
    expect(
      within(downloadPanel).getByText("已准备 2 张图片下载"),
    ).toBeInTheDocument();
    expect(
      within(downloadPanel).queryByText("caption only"),
    ).not.toBeInTheDocument();
    const firstDownload = within(downloadPanel).getByRole("link", {
      name: "下载 download selected first",
    });
    const secondDownload = within(downloadPanel).getByRole("link", {
      name: "下载 download selected second",
    });
    expect(firstDownload).toHaveAttribute(
      "href",
      "https://signed.example/first-download.png",
    );
    expect(firstDownload).toHaveAttribute("download", "first.png");
    expect(secondDownload).toHaveAttribute(
      "href",
      "https://signed.example/second-download.jpg",
    );
    expect(secondDownload).toHaveAttribute("download", "second.jpg");
  });

  it("opens history image previews in a lightbox", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "draw lightbox",
                parameters: {},
                files: [
                  {
                    id: "fil_1",
                    storageProvider: "aliyun-oss",
                    bucket: "whalesing-web",
                    objectKey:
                      "aiImg/users/usr_1/OpenAI/gpt-image-1/lightbox.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 3,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_1/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/lightbox.png" },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    await userEvent.click(
      await screen.findByRole("button", { name: "打开预览 draw lightbox" }),
    );

    const dialog = await screen.findByRole("dialog", { name: "图片预览" });
    expect(dialog).toBeInTheDocument();
    expect(
      within(dialog).getByRole("img", { name: "draw lightbox" }),
    ).toHaveAttribute("src", "https://signed.example/lightbox.png");
    expect(
      within(dialog).getByRole("link", { name: "打开原图" }),
    ).toHaveAttribute("href", "https://signed.example/lightbox.png");

    await userEvent.click(screen.getByRole("button", { name: "关闭预览" }));
    expect(
      screen.queryByRole("dialog", { name: "图片预览" }),
    ).not.toBeInTheDocument();
  });

  it("filters history records by capability and channel", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "draw whale",
                parameters: {},
                files: [],
                createdAt: 3,
              },
              {
                id: "his_2",
                taskId: "tsk_2",
                kind: "text",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-4o",
                capability: "image-to-text",
                prompt: "describe whale",
                parameters: {},
                resultText: "caption result",
                files: [],
                createdAt: 2,
              },
              {
                id: "his_3",
                taskId: "tsk_3",
                kind: "text",
                status: "完成",
                channelName: "Gemini",
                modelId: "gemini-2.5-flash",
                capability: "image-to-text",
                prompt: "describe logo",
                parameters: {},
                resultText: "logo caption",
                files: [],
                createdAt: 1,
              },
              {
                id: "his_4",
                taskId: "tsk_4",
                kind: "image",
                status: "失败",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "cancelled image",
                parameters: {},
                failureReason: "用户取消",
                files: [],
                createdAt: 3,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("draw whale")).toBeInTheDocument();
    expect(screen.queryByText("cancelled image")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "失败" }),
    ).not.toBeInTheDocument();

    await userEvent.selectOptions(
      screen.getByLabelText("类型筛选"),
      "image-to-text",
    );
    expect(screen.queryByText("draw whale")).not.toBeInTheDocument();
    expect(screen.getByText("describe whale")).toBeInTheDocument();
    expect(screen.getByText("describe logo")).toBeInTheDocument();

    await userEvent.selectOptions(
      screen.getByLabelText("供应源筛选"),
      "Gemini",
    );
    expect(screen.queryByText("describe whale")).not.toBeInTheDocument();
    expect(screen.getByText("describe logo")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "清空筛选" }));
    expect(screen.getByText("draw whale")).toBeInTheDocument();
    expect(screen.getByText("describe whale")).toBeInTheDocument();
    expect(screen.getByText("describe logo")).toBeInTheDocument();
    expect(screen.queryByText("cancelled image")).not.toBeInTheDocument();
  });

  it("copies text history results to the clipboard", async () => {
    window.history.pushState({}, "", "/history");
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "text",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-4o",
                capability: "image-to-text",
                prompt: "describe",
                parameters: {},
                resultText: "caption result",
                files: [],
                createdAt: 1,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("caption result")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "复制结果" }));

    expect(writeText).toHaveBeenCalledWith("caption result");
    expect(await screen.findByText("结果已复制")).toBeInTheDocument();
  });

  it("copies history prompts to the clipboard", async () => {
    window.history.pushState({}, "", "/history");
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "draw a whale in neon rain",
                parameters: {},
                files: [],
                createdAt: 1,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    expect(
      await screen.findByText("draw a whale in neon rain"),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "复制提示词" }));

    expect(writeText).toHaveBeenCalledWith("draw a whale in neon rain");
    expect(await screen.findByText("提示词已复制")).toBeInTheDocument();
  });

  it("falls back when clipboard writes are blocked", async () => {
    window.history.pushState({}, "", "/history");
    const writeText = vi.fn().mockRejectedValue(new Error("blocked"));
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: vi.fn().mockReturnValue(true),
    });
    const execCommand = vi.mocked(document.execCommand);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "fallback prompt",
                parameters: {},
                files: [],
                createdAt: 1,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(await screen.findByText("fallback prompt")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "复制提示词" }));

    expect(writeText).toHaveBeenCalledWith("fallback prompt");
    expect(execCommand).toHaveBeenCalledWith("copy");
    expect(await screen.findByText("提示词已复制")).toBeInTheDocument();
  });

  it("shows manual copy text when browser clipboard access is unavailable", async () => {
    window.history.pushState({}, "", "/history");
    const writeText = vi.fn().mockRejectedValue(new Error("blocked"));
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: vi.fn().mockReturnValue(false),
    });
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "manual copy prompt",
                parameters: {},
                files: [],
                createdAt: 1,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(await screen.findByText("manual copy prompt")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "复制提示词" }));

    expect(
      await screen.findByText("复制失败，请手动复制下方文本"),
    ).toBeInTheDocument();
    const manualCopyText = screen.getByLabelText("手动复制文本");
    expect(manualCopyText).toHaveValue("manual copy prompt");
    expect(manualCopyText).toHaveFocus();
    await userEvent.click(screen.getByRole("button", { name: "关闭手动复制" }));
    expect(screen.queryByLabelText("手动复制文本")).not.toBeInTheDocument();
  });

  it("deletes a history record from the history page", async () => {
    window.history.pushState({}, "", "/history");
    const confirmSpy = vi.spyOn(window, "confirm");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/history" && (!init || init.method === undefined)) {
          return jsonResponse({
            ok: true,
            data: {
              records: [
                {
                  id: "his_1",
                  taskId: "tsk_1",
                  kind: "text",
                  status: "完成",
                  channelName: "OpenAI",
                  modelId: "gpt-4o",
                  capability: "image-to-text",
                  prompt: "delete this caption",
                  parameters: {},
                  resultText: "caption result",
                  files: [],
                  createdAt: 1,
                },
              ],
            },
          });
        }
        if (input === "/api/history/his_1" && init?.method === "DELETE") {
          return jsonResponse({ ok: true, data: { deleted: true } });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("delete this caption")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "删除" }));

    expect(confirmSpy).not.toHaveBeenCalled();
    const historyConfirm = screen.getByRole("region", {
      name: "确认删除历史记录",
    });
    expect(
      within(historyConfirm).getByText("确认删除历史记录？"),
    ).toBeInTheDocument();
    expect(
      within(historyConfirm).getByText("delete this caption"),
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalledWith(
      "/api/history/his_1",
      expect.objectContaining({
        method: "DELETE",
        credentials: "include",
      }),
    );
    await userEvent.click(screen.getByRole("button", { name: "确认删除记录" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/history/his_1",
        expect.objectContaining({
          method: "DELETE",
          credentials: "include",
        }),
      );
    });
    expect(screen.queryByText("delete this caption")).not.toBeInTheDocument();
    expect(await screen.findByText("历史记录已删除")).toBeInTheDocument();
  });

  it("copies selected history prompts from the current filtered list", async () => {
    window.history.pushState({}, "", "/history");
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "copy selected one",
                parameters: {},
                files: [],
                createdAt: 3,
              },
              {
                id: "his_2",
                taskId: "tsk_2",
                kind: "text",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-4o",
                capability: "image-to-text",
                prompt: "copy selected two",
                parameters: {},
                resultText: "caption",
                files: [],
                createdAt: 2,
              },
              {
                id: "his_3",
                taskId: "tsk_3",
                kind: "image",
                status: "完成",
                channelName: "Gemini",
                modelId: "gemini",
                capability: "text-to-image",
                prompt: "copy filtered out",
                parameters: {},
                files: [],
                createdAt: 1,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(await screen.findByText("copy selected one")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "复制所选提示词" }),
    ).toBeDisabled();
    await userEvent.selectOptions(
      screen.getByLabelText("供应源筛选"),
      "OpenAI",
    );
    await userEvent.click(screen.getByRole("button", { name: "选择当前筛选" }));
    await userEvent.click(
      screen.getByRole("button", { name: "复制所选提示词" }),
    );

    expect(writeText).toHaveBeenCalledWith(
      "copy selected one\n\ncopy selected two",
    );
    expect(await screen.findByText("已复制 2 条提示词")).toBeInTheDocument();
  });

  it("copies selected text history results and skips records without text results", async () => {
    window.history.pushState({}, "", "/history");
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "image without text result",
                parameters: {},
                files: [],
                createdAt: 3,
              },
              {
                id: "his_2",
                taskId: "tsk_2",
                kind: "text",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-4o",
                capability: "image-to-text",
                prompt: "copy text result one",
                parameters: {},
                resultText: "caption one",
                files: [],
                createdAt: 2,
              },
              {
                id: "his_3",
                taskId: "tsk_3",
                kind: "text",
                status: "完成",
                channelName: "Gemini",
                modelId: "gemini",
                capability: "image-to-text",
                prompt: "copy text result two",
                parameters: {},
                resultText: "caption two",
                files: [],
                createdAt: 1,
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByText("image without text result"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "复制所选结果" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "选择当前筛选" }));
    await userEvent.click(screen.getByRole("button", { name: "复制所选结果" }));

    expect(writeText).toHaveBeenCalledWith("caption one\n\ncaption two");
    expect(await screen.findByText("已复制 2 条结果")).toBeInTheDocument();
  });

  it("bulk deletes selected history records from the current filtered list", async () => {
    window.history.pushState({}, "", "/history");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/history" && (!init || init.method === undefined)) {
          return jsonResponse({
            ok: true,
            data: {
              records: [
                {
                  id: "his_1",
                  taskId: "tsk_1",
                  kind: "image",
                  status: "完成",
                  channelName: "OpenAI",
                  modelId: "gpt-image-1",
                  capability: "text-to-image",
                  prompt: "delete selected one",
                  parameters: {},
                  files: [],
                  createdAt: 3,
                },
                {
                  id: "his_2",
                  taskId: "tsk_2",
                  kind: "text",
                  status: "完成",
                  channelName: "OpenAI",
                  modelId: "gpt-4o",
                  capability: "image-to-text",
                  prompt: "delete selected two",
                  parameters: {},
                  resultText: "caption",
                  files: [],
                  createdAt: 2,
                },
                {
                  id: "his_3",
                  taskId: "tsk_3",
                  kind: "image",
                  status: "完成",
                  channelName: "Gemini",
                  modelId: "gemini",
                  capability: "text-to-image",
                  prompt: "keep filtered out",
                  parameters: {},
                  files: [],
                  createdAt: 1,
                },
              ],
            },
          });
        }
        if (
          (input === "/api/history/his_1" || input === "/api/history/his_2") &&
          init?.method === "DELETE"
        ) {
          return jsonResponse({ ok: true, data: { deleted: true } });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("delete selected one")).toBeInTheDocument();
    await userEvent.selectOptions(
      screen.getByLabelText("供应源筛选"),
      "OpenAI",
    );
    await userEvent.click(screen.getByRole("button", { name: "选择当前筛选" }));
    expect(screen.getByText("已选 2 项")).toBeInTheDocument();
    await userEvent.selectOptions(
      screen.getByLabelText("供应源筛选"),
      "Gemini",
    );
    expect(screen.getByText("已选 0 项")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "删除所选" })).toBeDisabled();

    await userEvent.selectOptions(
      screen.getByLabelText("供应源筛选"),
      "OpenAI",
    );
    await userEvent.click(screen.getByRole("button", { name: "选择当前筛选" }));
    expect(screen.getByText("已选 2 项")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "删除所选" }));
    expect(
      screen.getByText("确认删除选中的 2 条历史记录？"),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "确认删除" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/history/his_1",
        expect.objectContaining({ method: "DELETE", credentials: "include" }),
      );
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/history/his_2",
        expect.objectContaining({ method: "DELETE", credentials: "include" }),
      );
    });
    expect(screen.queryByText("delete selected one")).not.toBeInTheDocument();
    expect(screen.queryByText("delete selected two")).not.toBeInTheDocument();
    expect(screen.getByText("已删除 2 条历史记录")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("供应源筛选"), "");
    expect(screen.getByText("keep filtered out")).toBeInTheDocument();
  });

  it("continues bulk deletion when a selected history record is already gone", async () => {
    window.history.pushState({}, "", "/history");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input, init) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/history" && (!init || init.method === undefined)) {
          return jsonResponse({
            ok: true,
            data: {
              records: [
                {
                  id: "his_missing",
                  taskId: "tsk_missing",
                  kind: "image",
                  status: "完成",
                  channelName: "OpenAI",
                  modelId: "gpt-image-1",
                  capability: "text-to-image",
                  prompt: "already gone",
                  parameters: {},
                  files: [],
                  createdAt: 2,
                },
                {
                  id: "his_ok",
                  taskId: "tsk_ok",
                  kind: "text",
                  status: "完成",
                  channelName: "OpenAI",
                  modelId: "gpt-4o",
                  capability: "image-to-text",
                  prompt: "still deletable",
                  parameters: {},
                  resultText: "caption",
                  files: [],
                  createdAt: 1,
                },
              ],
            },
          });
        }
        if (input === "/api/history/his_missing" && init?.method === "DELETE") {
          return jsonResponse(
            {
              ok: false,
              error: { code: "history_not_found", message: "历史记录不存在" },
            },
            404,
          );
        }
        if (input === "/api/history/his_ok" && init?.method === "DELETE") {
          return jsonResponse({ ok: true, data: { deleted: true } });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("already gone")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "选择当前筛选" }));
    await userEvent.click(screen.getByRole("button", { name: "删除所选" }));
    await userEvent.click(screen.getByRole("button", { name: "确认删除" }));

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/history/his_missing",
        expect.objectContaining({ method: "DELETE", credentials: "include" }),
      );
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/history/his_ok",
        expect.objectContaining({ method: "DELETE", credentials: "include" }),
      );
    });
    expect(screen.queryByText("already gone")).not.toBeInTheDocument();
    expect(screen.queryByText("still deletable")).not.toBeInTheDocument();
    expect(screen.getByText("已删除 2 条历史记录")).toBeInTheDocument();
  });

  it("reuses a history record in the workspace composer", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "text-to-image",
                prompt: "reuse this whale",
                parameters: { size: "1536x1024", count: 3 },
                files: [],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  { id: "gpt-image-1", capabilities: ["text-to-image"] },
                ],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("reuse this whale")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "复用到创作" }));

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("提示词")).toHaveValue("reuse this whale");
    expect(screen.getByLabelText("模式")).toHaveValue("text-to-image");
    expect(screen.getByDisplayValue("1536x1024")).toBeInTheDocument();
    expect(screen.queryByLabelText("数量")).not.toBeInTheDocument();
  });

  it("brings a history record back to the workspace for another generation", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_1",
                taskId: "tsk_1",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-1",
                capability: "image-to-image",
                prompt: "make it cinematic",
                parameters: { size: "1024x1536", count: 2 },
                files: [
                  {
                    id: "fil_hist",
                    storageProvider: "aliyun-oss",
                    bucket: "bucket",
                    objectKey: "aiImg/history-reference.webp",
                    mediaType: "image",
                    mimeType: "image/webp",
                    sizeBytes: 9,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_hist/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/history-reference.webp" },
        });
      }
      if (input === "/api/files/fil_hist/download-url") {
        return jsonResponse({
          ok: true,
          data: {
            url: "https://signed.example/history-reference.webp?download=1",
          },
        });
      }
      if (input === "/api/files/fil_hist/content") {
        return imageResponse("webp-ref", "image/webp");
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-1",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    await userEvent.click(
      await screen.findByRole("button", { name: "再次生成" }),
    );

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("提示词")).toHaveValue("make it cinematic");
    expect(screen.getByLabelText("模式")).toHaveValue("image-to-image");
    expect(screen.getByDisplayValue("1024x1536")).toBeInTheDocument();
    expect(screen.queryByLabelText("数量")).not.toBeInTheDocument();
    expect(
      await screen.findByText("history-reference.webp"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("已将历史垫图带回创作栏，可继续调整后再次生成。"),
    ).toBeInTheDocument();
  });

  it("sets a history artwork image as an image-to-image reference", async () => {
    window.history.pushState({}, "", "/history");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      if (input === "/api/me") {
        return jsonResponse({
          ok: true,
          data: { user: { id: "usr_1", username: "alice", role: "user" } },
        });
      }
      if (input === "/api/history") {
        return jsonResponse({
          ok: true,
          data: {
            records: [
              {
                id: "his_base",
                taskId: "tsk_base",
                kind: "image",
                status: "完成",
                channelName: "OpenAI",
                modelId: "gpt-image-2",
                capability: "text-to-image",
                prompt: "history base",
                parameters: { size: "1536x1024", count: 1 },
                files: [
                  {
                    id: "fil_history_base",
                    storageProvider: "aliyun-oss",
                    bucket: "bucket",
                    objectKey: "aiImg/history-base.png",
                    mediaType: "image",
                    mimeType: "image/png",
                    sizeBytes: 9,
                    createdAt: 1,
                  },
                ],
                createdAt: 1,
              },
            ],
          },
        });
      }
      if (input === "/api/files/fil_history_base/signed-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/history-base.png" },
        });
      }
      if (input === "/api/files/fil_history_base/download-url") {
        return jsonResponse({
          ok: true,
          data: { url: "https://signed.example/history-base.png?download=1" },
        });
      }
      if (input === "/api/files/fil_history_base/content") {
        return imageResponse("history-base-bytes");
      }
      if (input === "/api/channels") {
        return jsonResponse({
          ok: true,
          data: {
            channels: [
              {
                id: "chn_1",
                name: "OpenAI",
                baseUrl: "https://api.openai.com",
                hasApiKey: true,
                models: [
                  {
                    id: "gpt-image-2",
                    capabilities: ["text-to-image", "image-to-image"],
                  },
                ],
              },
            ],
          },
        });
      }
      return jsonResponse({ ok: false }, 404);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    await userEvent.click(
      await screen.findByRole("button", {
        name: "将历史作品 history base 1 设为参考图",
      }),
    );

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("history-base.png")).toBeInTheDocument();
    expect(screen.getByLabelText("模式")).toHaveValue("image-to-image");
    expect(screen.getByLabelText("提示词")).toHaveValue("history base");
  });

  it("warns that image-to-text history needs a fresh reference image before regenerating", async () => {
    window.history.pushState({}, "", "/history");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input) => {
        if (input === "/api/me") {
          return jsonResponse({
            ok: true,
            data: { user: { id: "usr_1", username: "alice", role: "user" } },
          });
        }
        if (input === "/api/history") {
          return jsonResponse({
            ok: true,
            data: {
              records: [
                {
                  id: "his_1",
                  taskId: "tsk_1",
                  kind: "text",
                  status: "完成",
                  channelName: "OpenAI",
                  modelId: "gpt-4o",
                  capability: "image-to-text",
                  prompt: "summarize the uploaded image",
                  parameters: {},
                  resultText: "A bright product photo.",
                  files: [],
                  createdAt: 1,
                },
              ],
            },
          });
        }
        if (input === "/api/channels") {
          return jsonResponse({
            ok: true,
            data: {
              channels: [
                {
                  id: "chn_1",
                  name: "OpenAI",
                  baseUrl: "https://api.openai.com",
                  hasApiKey: true,
                  models: [{ id: "gpt-4o", capabilities: ["image-to-text"] }],
                },
              ],
            },
          });
        }
        return jsonResponse({ ok: false }, 404);
      });

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "历史记录" }),
    ).toBeInTheDocument();
    await userEvent.click(
      await screen.findByRole("button", { name: "再次生成" }),
    );

    expect(
      await screen.findByRole("heading", { name: "创作工作台" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("提示词")).toHaveValue(
      "summarize the uploaded image",
    );
    expect(screen.getByLabelText("模式")).toHaveValue("image-to-text");
    expect(
      screen.getByText("图生文记录需要重新上传参考图后再生成。"),
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalledWith(
      "/api/generate/text",
      expect.anything(),
    );
  });
});
