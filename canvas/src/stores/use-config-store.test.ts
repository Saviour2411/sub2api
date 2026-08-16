// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

import { defaultConfig, encodeChannelModel, resolveModelForCapability, useConfigStore, type AiConfig } from "@/stores/use-config-store";

describe("画布运行时密钥持久化隔离", () => {
    it("持久化状态会清空顶层和分组 API Key", () => {
        const state = useConfigStore.getState();
        const candidate = {
            ...state,
            config: {
                ...state.config,
                apiKey: "sk-top-secret",
                channels: state.config.channels.map((channel) => ({ ...channel, apiKey: "sk-channel-secret" })),
            },
        };

        const partialize = useConfigStore.persist.getOptions().partialize;
        const persisted = partialize ? partialize(candidate) : candidate;
        const serialized = JSON.stringify(persisted);

        expect(serialized).not.toContain("sk-top-secret");
        expect(serialized).not.toContain("sk-channel-secret");
    });

    it("把旧版默认三张迁移为一张", () => {
        const state = useConfigStore.getState();
        const merge = useConfigStore.persist.getOptions().merge;
        const merged = merge?.({ config: { ...defaultConfig, canvasImageCount: "3" } }, state) as typeof state;

        expect(defaultConfig.canvasImageCount).toBe("1");
        expect(merged.config.canvasImageCount).toBe("1");
    });
});

describe("当前分组模型解析", () => {
    it("重试旧分组节点时改用当前分组的同能力模型", () => {
        const currentModel = encodeChannelModel("23", "gpt-image-2");
        const config: AiConfig = {
            ...defaultConfig,
            channels: [
                {
                    id: "23",
                    name: "Image",
                    baseUrl: "/",
                    apiKey: "sk-runtime-only",
                    apiFormat: "openai",
                    models: [{ name: "gpt-image-2", capability: "image" }],
                },
            ],
            models: [currentModel],
            model: currentModel,
            imageModel: currentModel,
        };

        expect(resolveModelForCapability(config, encodeChannelModel("8", "gpt-image-2"), "image")).toBe(currentModel);
    });

    it("当前分组没有图片模型时不回退到内置假模型", () => {
        const textModel = encodeChannelModel("39", "grok-4.5");
        const config: AiConfig = {
            ...defaultConfig,
            channels: [{ id: "39", name: "Grok", baseUrl: "/", apiKey: "sk-runtime-only", apiFormat: "openai", models: [{ name: "grok-4.5", capability: "text" }] }],
            models: [textModel],
            model: textModel,
            imageModel: "",
            textModel,
        };

        expect(resolveModelForCapability(config, encodeChannelModel("23", "gpt-image-2"), "image")).toBe("");
    });
});
