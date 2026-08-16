// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

import { classifyCanvasModels } from "@/host/models";
import type { CanvasGroup } from "@/host/bridge";

const openAIGroup: CanvasGroup = { id: 1, name: "OpenAI", platform: "openai", api_format: "openai" };
const geminiGroup: CanvasGroup = { id: 2, name: "Gemini", platform: "gemini", api_format: "gemini" };

describe("画布模型分类", () => {
    it("按别名和元数据分类文本、图像与视频模型", () => {
        expect(classifyCanvasModels([{ id: "gpt-image-1" }, { id: "sora-2" }, { id: "gpt-5" }, { id: "custom-render", capabilities: ["image_generation"] }], openAIGroup)).toEqual([
            { name: "gpt-image-1", capability: "image" },
            { name: "sora-2", capability: "video" },
            { name: "gpt-5", capability: "text" },
            { name: "custom-render", capability: "image" },
        ]);
    });

    it("只暴露支持 generateContent 的 Gemini 图片模型，并隐藏 predict 与视频模型", () => {
        expect(
            classifyCanvasModels(
                [
                    { name: "models/imagen-4", supportedGenerationMethods: ["predict"] },
                    { name: "models/gemini-3.1-flash-image", supportedGenerationMethods: ["generateContent", "streamGenerateContent"] },
                    { name: "models/nano-banana-pro" },
                    { name: "models/veo-3", supportedGenerationMethods: ["generateContent"] },
                ],
                geminiGroup,
            ),
        ).toEqual([
            { name: "gemini-3.1-flash-image", capability: "image" },
            { name: "nano-banana-pro", capability: "image" },
        ]);
    });
});
