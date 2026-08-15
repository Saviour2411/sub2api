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

    it("识别 Imagen 和 nano-banana，且不向 Gemini 暴露视频模型", () => {
        expect(classifyCanvasModels([{ name: "models/imagen-4" }, { name: "models/nano-banana-pro" }, { name: "models/veo-3" }], geminiGroup)).toEqual([
            { name: "imagen-4", capability: "image" },
            { name: "nano-banana-pro", capability: "image" },
        ]);
    });
});
