// @vitest-environment jsdom
import axios from "axios";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { requestEdit, requestGeneration } from "@/services/api/image";
import { createVideoGenerationTask, pollVideoGenerationTask } from "@/services/api/video";
import { defaultConfig, encodeChannelModel, type AiConfig, type ApiCallFormat, type ApiPlatform, type ModelCapability } from "@/stores/use-config-store";

vi.mock("axios", () => ({
    default: {
        get: vi.fn(),
        post: vi.fn(),
        isCancel: vi.fn(() => false),
        isAxiosError: vi.fn(() => false),
    },
}));

function stationConfig(apiFormat: ApiCallFormat, model: string, capability: ModelCapability, platform?: ApiPlatform): AiConfig {
    const value = encodeChannelModel("station", model);
    const resolvedPlatform = platform || (apiFormat === "gemini" ? "gemini" : model.startsWith("grok-") ? "grok" : apiFormat === "ark" ? "ark" : "openai");
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
                platform: resolvedPlatform,
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

    it("GPT Image 使用官方输出字段，Grok 数量限制到官方上限", async () => {
        vi.mocked(axios.post).mockResolvedValue({ data: { data: [{ b64_json: "aGVsbG8=" }] } });
        const openAIConfig = stationConfig("openai", "gpt-image-2", "image", "openai");
        const grokConfig = stationConfig("openai", "grok-imagine-image-2.0", "image", "grok");
        grokConfig.count = "15";

        await requestGeneration(openAIConfig, "OpenAI 生图");
        await requestGeneration(grokConfig, "Grok 生图");

        const openAIBody = vi.mocked(axios.post).mock.calls[0][1] as Record<string, unknown>;
        const grokBody = vi.mocked(axios.post).mock.calls[1][1] as Record<string, unknown>;
        expect(openAIBody).toEqual(expect.objectContaining({ output_format: "png" }));
        expect(openAIBody).not.toHaveProperty("response_format");
        expect(grokBody).toEqual(expect.objectContaining({ n: 10, response_format: "b64_json" }));
        expect(grokBody).not.toHaveProperty("output_format");
    });

    it("GPT Image 图片编辑不发送官方不支持的 response_format", async () => {
        const config = stationConfig("openai", "gpt-image-2", "image", "openai");
        const reference = { id: "reference-1", name: "参考图", type: "image/png", dataUrl: "data:image/png;base64,aW5wdXQ=" };
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { data: [{ b64_json: "cmVzdWx0" }] } });

        await requestEdit(config, "修改图片", [reference]);

        const body = vi.mocked(axios.post).mock.calls[0][1] as FormData;
        expect(body.get("output_format")).toBe("png");
        expect(body.has("response_format")).toBe(false);
    });

    it("Gemini 图像使用 SSE 并从跨分片事件提取、去重图片", async () => {
        const events = [
            `data: {"candidates":[{"content":{"parts":[{"text":"正在生成"}]}}]}\n\n`,
            `data: {"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aGVsbG8="}}]}}]}\n\n`,
            `data: {"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aGVsbG8="}},{"inline_data":{"mime_type":"image/jpeg","data":"d29ybGQ="}}]}}]}\n\n`,
            "data: [DONE]\n\n",
        ].join("");
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(streamResponse([events.slice(0, 31), events.slice(31, 127), events.slice(127)])));

        const images = await requestGeneration(stationConfig("gemini", "gemini-3.1-flash-image", "image"), "画一张图");

        expect(images.map((image) => image.dataUrl)).toEqual(["data:image/png;base64,aGVsbG8=", "data:image/jpeg;base64,d29ybGQ="]);
        expect(fetch).toHaveBeenCalledWith(
            "/v1beta/models/gemini-3.1-flash-image:streamGenerateContent?alt=sse",
            expect.objectContaining({
                method: "POST",
                headers: expect.objectContaining({ "x-goog-api-key": "sk-runtime-only", Accept: "text/event-stream" }),
                body: expect.stringContaining('"responseModalities":["TEXT","IMAGE"]'),
            }),
        );
        const request = vi.mocked(fetch).mock.calls[0][1] as RequestInit;
        const body = JSON.parse(String(request.body)) as { generationConfig?: { imageConfig?: { aspectRatio?: string }; responseFormat?: unknown } };
        expect(body.generationConfig?.imageConfig).toEqual({ aspectRatio: "1:1" });
        expect(body.generationConfig?.responseFormat).toBeUndefined();
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

        const images = await requestGeneration(stationConfig("gemini", "gemini-3.1-flash-image", "image"), "画一张图");

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

        await expect(requestGeneration(stationConfig("gemini", "gemini-3.1-flash-image", "image"), "画一张图")).rejects.toThrow("上游图片服务拒绝请求");
    });

    it("Gemini SSE 流结束仍无图时明确报错", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(streamResponse(['data: {"candidates":[{"content":{"parts":[{"text":"没有生成图片"}]}}]}\n\ndata: [DONE]\n\n'])));

        await expect(requestGeneration(stationConfig("gemini", "gemini-3.1-flash-image", "image"), "画一张图")).rejects.toThrow("Gemini 接口没有返回图片");
    });

    it("Gemini HTTP 错误优先显示响应中的真实原因", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: "上游额度不足" } }), { status: 429, headers: { "Content-Type": "application/json" } })));

        await expect(requestGeneration(stationConfig("gemini", "gemini-3.1-flash-image", "image"), "画一张图")).rejects.toThrow("上游额度不足");
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
        const body = vi.mocked(axios.post).mock.calls[0][1] as FormData;
        expect(Object.fromEntries(body.entries())).toEqual(expect.objectContaining({ model: "sora-2", prompt: "生成视频", seconds: "8", size: "1280x720" }));
        expect(body.has("resolution_name")).toBe(false);
        expect(body.has("preset")).toBe(false);
        expect(axios.get).toHaveBeenCalledWith("/v1/videos/task-1", expect.objectContaining({ headers: expect.objectContaining({ Authorization: "Bearer sk-runtime-only" }) }));
    });

    it("OpenAI 视频使用单数 input_reference 字段并拒绝多张参考图", async () => {
        const config = stationConfig("openai", "sora-2", "video", "openai");
        const reference = { id: "reference-1", name: "首帧", type: "image/png", dataUrl: "data:image/png;base64,aW5wdXQ=" };
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { id: "task-reference" } });

        await createVideoGenerationTask(config, "生成视频", [reference]);

        const body = vi.mocked(axios.post).mock.calls[0][1] as FormData;
        expect(body.has("input_reference")).toBe(true);
        expect(body.has("input_reference[]")).toBe(false);
        await expect(createVideoGenerationTask(config, "生成视频", [reference, { ...reference, id: "reference-2" }])).rejects.toThrow("OpenAI 视频生成最多支持 1 张首帧参考图");
    });

    it("OpenAI 视频把非官方尺寸映射到合法枚举", async () => {
        const config = stationConfig("openai", "sora-2", "video", "openai");
        config.size = "1024x1024";
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { id: "task-square" } });

        await createVideoGenerationTask(config, "生成视频");

        const body = vi.mocked(axios.post).mock.calls[0][1] as FormData;
        expect(body.get("size")).toBe("1280x720");
    });

    it("Grok 视频使用 xAI 官方 JSON 并识别 request_id", async () => {
        const config = stationConfig("openai", "grok-imagine-video", "video", "grok");
        config.vquality = "480";
        config.size = "16:9";
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { request_id: "grok-task-1" } });

        const task = await createVideoGenerationTask(config, "生成视频");

        expect(task).toEqual({ id: "grok-task-1", provider: "grok", model: "station::grok-imagine-video" });
        expect(axios.post).toHaveBeenCalledWith(
            "/v1/videos/generations",
            {
                model: "grok-imagine-video",
                prompt: "生成视频",
                duration: 6,
                resolution: "480p",
                aspect_ratio: "16:9",
            },
            expect.objectContaining({ headers: expect.objectContaining({ Authorization: "Bearer sk-runtime-only", "Content-Type": "application/json" }) }),
        );
    });

    it("Grok 图生视频使用官方 image 对象并把非标准尺寸映射到合法比例", async () => {
        const config = stationConfig("openai", "grok-imagine-video-1.5", "video", "grok");
        config.size = "1792x1024";
        config.vquality = "1000";
        const reference = { id: "reference-1", name: "首帧", type: "image/png", dataUrl: "data:image/png;base64,aW5wdXQ=" };
        vi.mocked(axios.post).mockResolvedValueOnce({ data: { request_id: "grok-task-image" } });

        await createVideoGenerationTask(config, "生成视频", [reference]);

        const body = vi.mocked(axios.post).mock.calls[0][1] as { image?: Record<string, unknown>; aspect_ratio?: string; resolution?: string };
        expect(body.image).toEqual({ url: "data:image/png;base64,aW5wdXQ=" });
        expect(body.aspect_ratio).toBe("16:9");
        expect(body.resolution).toBe("1080p");
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

    it("Grok 视频兼容官方 done 状态和 video.url 响应", async () => {
        const config = stationConfig("openai", "grok-imagine-video", "video", "grok");
        const blob = new Blob(["video"], { type: "video/mp4" });
        vi.mocked(axios.get)
            .mockResolvedValueOnce({ data: { request_id: "grok-task-2", status: "done", video: { url: "https://cdn.example.com/grok.mp4" } } })
            .mockResolvedValueOnce({ data: blob });

        const state = await pollVideoGenerationTask(config, { id: "grok-task-2", provider: "grok", model: encodeChannelModel("station", "grok-imagine-video") });

        expect(state).toEqual({ status: "completed", result: { blob } });
        expect(axios.get).toHaveBeenNthCalledWith(2, "/v1/videos/grok-task-2/content", expect.objectContaining({ responseType: "blob" }));
    });

    it("Gemini 视频首版明确拒绝且不发送请求", async () => {
        const config = stationConfig("gemini", "veo-3", "video");

        await expect(createVideoGenerationTask(config, "生成视频")).rejects.toThrow();
        expect(axios.post).not.toHaveBeenCalled();
        expect(axios.get).not.toHaveBeenCalled();
    });
});
