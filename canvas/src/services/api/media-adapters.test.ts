// @vitest-environment jsdom
import axios from "axios";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { requestGeneration } from "@/services/api/image";
import { createVideoGenerationTask, pollVideoGenerationTask } from "@/services/api/video";
import { defaultConfig, encodeChannelModel, type AiConfig, type ApiCallFormat, type ModelCapability } from "@/stores/use-config-store";

vi.mock("axios", () => ({
    default: {
        get: vi.fn(),
        post: vi.fn(),
        isCancel: vi.fn(() => false),
        isAxiosError: vi.fn(() => false),
    },
}));

function stationConfig(apiFormat: ApiCallFormat, model: string, capability: ModelCapability): AiConfig {
    const value = encodeChannelModel("station", model);
    return {
        ...defaultConfig,
        baseUrl: "/",
        apiKey: "sk-runtime-only",
        apiFormat,
        channels: [
            {
                id: "station",
                name: "站内分组",
                baseUrl: "/",
                apiKey: "sk-runtime-only",
                apiFormat,
                models: [{ name: model, capability }],
            },
        ],
        models: [value],
        model: value,
        imageModel: capability === "image" ? value : "",
        videoModel: capability === "video" ? value : "",
        textModel: capability === "text" ? value : "",
        count: "1",
    };
}

describe("站内媒体请求适配", () => {
    beforeEach(() => vi.clearAllMocks());

    it.each(["gpt-image-2", "grok-imagine-image-pro"])("OpenAI 兼容图像模型 %s 使用站内接口和 Bearer 密钥", async (model) => {
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { data: [{ b64_json: "aGVsbG8=" }] } });

        const images = await requestGeneration(stationConfig("openai", model, "image"), "画一张图");

        expect(images[0].dataUrl).toBe("data:image/png;base64,aGVsbG8=");
        expect(axios.post).toHaveBeenCalledWith("/v1/images/generations", expect.objectContaining({ model, prompt: "画一张图" }), expect.objectContaining({ headers: expect.objectContaining({ Authorization: "Bearer sk-runtime-only" }) }));
    });

    it("Gemini 图像使用 v1beta generateContent 和 x-goog-api-key", async () => {
        vi.mocked(axios.post).mockResolvedValueOnce({
            data: { candidates: [{ content: { parts: [{ inlineData: { mimeType: "image/png", data: "aGVsbG8=" } }] } }] },
        });

        const images = await requestGeneration(stationConfig("gemini", "imagen-4", "image"), "画一张图");

        expect(images[0].dataUrl).toBe("data:image/png;base64,aGVsbG8=");
        expect(axios.post).toHaveBeenCalledWith("/v1beta/models/imagen-4:generateContent", expect.objectContaining({ contents: expect.any(Array) }), expect.objectContaining({ headers: expect.objectContaining({ "x-goog-api-key": "sk-runtime-only" }) }));
    });

    it("OpenAI 风格视频创建并轮询站内任务接口", async () => {
        const config = stationConfig("openai", "sora-2", "video");
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { id: "task-1", status: "queued" } });
        vi.mocked(axios.get).mockResolvedValueOnce({ data: { id: "task-1", status: "running" } });

        const task = await createVideoGenerationTask(config, "生成视频");
        const state = await pollVideoGenerationTask(config, task);

        expect(task).toEqual({ id: "task-1", provider: "openai", model: "station::sora-2" });
        expect(state).toEqual({ status: "pending" });
        expect(axios.post).toHaveBeenCalledWith("/v1/videos", expect.any(FormData), expect.objectContaining({ headers: expect.objectContaining({ Authorization: "Bearer sk-runtime-only" }) }));
        expect(axios.get).toHaveBeenCalledWith("/v1/videos/task-1", expect.objectContaining({ headers: expect.objectContaining({ Authorization: "Bearer sk-runtime-only" }) }));
    });

    it.each(["sora-2", "grok-imagine-video"])("视频模型 %s 完成后从站内内容接口读取 Blob", async (model) => {
        const config = stationConfig("openai", model, "video");
        const blob = new Blob(["video"], { type: "video/mp4" });
        vi.mocked(axios.get)
            .mockResolvedValueOnce({ data: { id: "task-2", status: "completed", video_url: "https://cdn.example.com/result.mp4" } })
            .mockResolvedValueOnce({ data: blob });

        const state = await pollVideoGenerationTask(config, { id: "task-2", provider: "openai", model: encodeChannelModel("station", model) });

        expect(state).toEqual({ status: "completed", result: { blob } });
        expect(axios.get).toHaveBeenNthCalledWith(2, "/v1/videos/task-2/content", expect.objectContaining({ responseType: "blob", headers: expect.objectContaining({ Authorization: "Bearer sk-runtime-only" }) }));
    });

    it("Gemini 视频首版明确拒绝且不发送请求", async () => {
        const config = stationConfig("gemini", "veo-3", "video");

        await expect(createVideoGenerationTask(config, "生成视频")).rejects.toThrow();
        expect(axios.post).not.toHaveBeenCalled();
        expect(axios.get).not.toHaveBeenCalled();
    });
});
