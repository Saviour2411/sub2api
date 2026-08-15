// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

import { sanitizeCanvasAssistantOps } from "@/lib/canvas/canvas-agent-ops";

describe("画布助手操作白名单", () => {
    it("拒绝删除、脚本与任意字段", () => {
        const result = sanitizeCanvasAssistantOps([
            { type: "delete_node", id: "n1" },
            { type: "add_node", nodeType: "remote-plugin", metadata: { content: "正文", script: "alert(1)", url: "https://example.com" } },
            { type: "update_node", id: "n1", metadata: { prompt: "新提示词", html: "<script />" } },
        ]);
        expect(result).toHaveLength(2);
        expect(result[0]).toMatchObject({ type: "add_node", nodeType: "text", metadata: { content: "正文" } });
        expect(result[1]).toMatchObject({ type: "update_node", id: "n1", metadata: { prompt: "新提示词" } });
        expect(JSON.stringify(result)).not.toContain("script");
        expect(JSON.stringify(result)).not.toContain("example.com");
    });

    it("仅保留需要单独确认的图像和视频生成", () => {
        expect(
            sanitizeCanvasAssistantOps([
                { type: "run_generation", nodeId: "a", mode: "image" },
                { type: "run_generation", nodeId: "b", mode: "text" },
                { type: "run_generation", nodeId: "c", mode: "video" },
            ]),
        ).toHaveLength(2);
    });
});
