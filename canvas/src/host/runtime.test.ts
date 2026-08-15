import { describe, expect, it } from "vitest";

import { getCanvasDatabaseName, getCanvasFallbackStorageKey, getCanvasSelectionKey, setHostUserId } from "@/host/runtime";

describe("画布用户存储隔离", () => {
    it("将用户 ID 写入 IndexedDB 与选择键命名空间", () => {
        setHostUserId(101);
        expect(getCanvasDatabaseName()).toBe("sub2api-infinite-canvas:101");
        expect(getCanvasSelectionKey()).toBe("sub2api-infinite-canvas:101:model-selection");
        expect(getCanvasFallbackStorageKey("app-state")).toBe("sub2api-infinite-canvas:101:app-state");
    });
});
