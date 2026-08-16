// @vitest-environment jsdom
import axios from "axios";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { requestEdit, requestGeneration } from "@/services/api/image";
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

function streamResponse(chunks: string[]) {
    const encoder = new TextEncoder();
    return new Response(
        new ReadableStream({
            start(controller) {
                chunks.forEach((chunk) => controller.enqueue(encoder.encode(chunk)));
                controller.close();
            },
        }),
        { headers: { "Content-Type": "text/event-stream" } },
    );
}

describe("站内媒体请求适配", () => {
    beforeEach(() => vi.clearAllMocks());
    afterEach(() => vi.unstubAllGlobals());

    it.each(["gpt-image-2", "grok-imagine-image-pro"])("OpenAI 兼容图像模型 %s 使用站内接口和 Bearer 密钥", async (model) => {
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { data: [{ b64_json: "aGVsbG8=" }] } });

        const images = await requestGeneration(stationConfig("openai", model, "image"), "画一张图");

        expect(images[0].dataUrl).toBe("data:image/png;base64,aGVsbG8=");
        expect(axios.post).toHaveBeenCalledWith("/v1/images/generations", expect.objectContaining({ model, prompt: "画一张图" }), expect.objectContaining({ headers: expect.objectContaining({ Authorization: "Bearer sk-runtime-only" }) }));
    });

    it("Gemini 图像使用 SSE 并从跨分片事件提取、去重图片", async () => {
        const events = [
            `data: {"candidates":[{"content":{"parts":[{"text":"正在生成"}]}}]}\n\n`,
            `data: {"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aGVsbG8="}}]}}]}\n\n`,
            `data: {"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aGVsbG8="}},{"inline_data":{"mime_type":"image/jpeg","data":"d29ybGQ="}}]}}]}\n\n`,
            "data: [DONE]\n\n",
        ].join("");
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(streamResponse([events.slice(0, 31), events.slice(31, 127), events.slice(127)])));

        const images = await requestGeneration(stationConfig("gemini", "imagen-4", "image"), "画一张图");

        expect(images.map((image) => image.dataUrl)).toEqual(["data:image/png;base64,aGVsbG8=", "data:image/jpeg;base64,d29ybGQ="]);
        expect(fetch).toHaveBeenCalledWith(
            "/v1beta/models/imagen-4:streamGenerateContent?alt=sse",
            expect.objectContaining({
                method: "POST",
                headers: expect.objectContaining({ "x-goog-api-key": "sk-runtime-only", Accept: "text/event-stream" }),
                body: expect.stringContaining('"responseModalities":["TEXT","IMAGE"]'),
            }),
        );
        expect(axios.post).not.toHaveBeenCalled();
    });

    it("Gemini 图像兼容非 SSE JSON 响应", async () => {
        vi.stubGlobal(
            "fetch",
            vi.fn().mockResolvedValue(
                new Response(JSON.stringify({ candidates: [{ content: { parts: [{ inline_data: { mime_type: "image/webp", data: "aGVsbG8=" } }] } }] }), {
                    headers: { "Content-Type": "application/json" },
                }),
            ),
        );

        const images = await requestGeneration(stationConfig("gemini", "imagen-4", "image"), "画一张图");

        expect(images[0].dataUrl).toBe("data:image/webp;base64,aGVsbG8=");
    });

    it("Gemini 图片编辑携带参考图并复用 SSE 链路", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(streamResponse(['data: {"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"cmVzdWx0"}}]}}]}\n\n'])));

        const images = await requestEdit(stationConfig("gemini", "gemini-3.1-flash-image", "image"), "改成水彩风格", [{ id: "reference-1", name: "参考图", type: "image/png", dataUrl: "data:image/png;base64,aW5wdXQ=" }]);

        expect(images[0].dataUrl).toBe("data:image/png;base64,cmVzdWx0");
        const request = vi.mocked(fetch).mock.calls[0][1] as RequestInit;
        const body = JSON.parse(String(request.body)) as { contents: Array<{ parts: Array<{ inlineData?: { data?: string } }> }> };
        expect(body.contents[0].parts).toEqual(expect.arrayContaining([expect.objectContaining({ inlineData: expect.objectContaining({ data: "aW5wdXQ=" }) })]));
    });

    it("Gemini SSE 透传上游真实错误", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(streamResponse(['data: {"error":{"message":"上游图片服务拒绝请求"}}\n\n'])));

        await expect(requestGeneration(stationConfig("gemini", "imagen-4", "image"), "画一张图")).rejects.toThrow("上游图片服务拒绝请求");
    });

    it("Gemini SSE 流结束仍无图时明确报错", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(streamResponse(['data: {"candidates":[{"content":{"parts":[{"text":"没有生成图片"}]}}]}\n\ndata: [DONE]\n\n'])));

        await expect(requestGeneration(stationConfig("gemini", "imagen-4", "image"), "画一张图")).rejects.toThrow("Gemini 接口没有返回图片");
    });

    it("Gemini HTTP 错误优先显示响应中的真实原因", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: "上游额度不足" } }), { status: 429, headers: { "Content-Type": "application/json" } })));

        await expect(requestGeneration(stationConfig("gemini", "imagen-4", "image"), "画一张图")).rejects.toThrow("上游额度不足");
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
