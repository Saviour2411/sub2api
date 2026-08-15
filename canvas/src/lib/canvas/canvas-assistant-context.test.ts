// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

import { buildAssistantUserContent } from "@/lib/canvas/canvas-assistant-context";
import type { CanvasAgentSnapshot } from "@/lib/canvas/canvas-agent-ops";
import { CanvasNodeType, type CanvasNodeData } from "@/types/canvas";

function imageNode(id: string, content: string): CanvasNodeData {
    return {
        id,
        type: CanvasNodeType.Image,
        title: id,
        position: { x: 0, y: 0 },
        width: 320,
        height: 320,
        metadata: { content },
    };
}

function snapshot(nodes: CanvasNodeData[]): CanvasAgentSnapshot {
    return {
        projectId: "project-1",
        title: "测试画布",
        nodes,
        connections: [],
        selectedNodeIds: nodes.map((node) => node.id),
        viewport: { x: 0, y: 0, k: 1 },
    };
}

describe("画布助手选中图片上下文", () => {
    it("最多附带两张本地图片", async () => {
        const content = await buildAssistantUserContent("分析选中图片", snapshot([imageNode("a", "data:image/png;base64,YQ=="), imageNode("b", "data:image/png;base64,Yg=="), imageNode("c", "data:image/png;base64,Yw==")]));

        expect(Array.isArray(content)).toBe(true);
        expect(content).toHaveLength(3);
        expect(JSON.stringify(content)).not.toContain("Yw==");
    });

    it("忽略远程图片地址", async () => {
        const content = await buildAssistantUserContent("不要读取远程地址", snapshot([imageNode("remote", "https://example.com/private.png")]));

        expect(content).toBe("不要读取远程地址");
    });
});
