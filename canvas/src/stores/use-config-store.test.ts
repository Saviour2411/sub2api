// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

import { useConfigStore } from "@/stores/use-config-store";

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
});
