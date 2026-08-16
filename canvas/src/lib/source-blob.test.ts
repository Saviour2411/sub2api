// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { dataUrlToBlob, sourceToBlob } from "@/lib/source-blob";

describe("媒体来源转换", () => {
    afterEach(() => vi.unstubAllGlobals());

    it("在本地解码 base64 data URL，不发起 fetch", async () => {
        const fetchMock = vi.fn();
        vi.stubGlobal("fetch", fetchMock);

        const blob = await sourceToBlob("data:image/png;base64,aGVsbG8=");

        expect(blob.type).toBe("image/png");
        expect(blob.size).toBe(5);
        expect(fetchMock).not.toHaveBeenCalled();
    });

    it("支持非 base64 data URL", () => {
        const blob = dataUrlToBlob("data:text/plain,%E7%94%BB%E5%B8%83");

        expect(blob.type).toBe("text/plain");
        expect(blob.size).toBe(6);
    });

    it("远程媒体返回非成功状态时保留 HTTP 状态", async () => {
        vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 403 }));

        await expect(sourceToBlob("https://example.com/result.png")).rejects.toThrow("读取媒体失败（HTTP 403）");
    });
});
