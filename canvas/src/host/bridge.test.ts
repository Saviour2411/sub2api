// @vitest-environment jsdom
import { describe, expect, it } from "vitest";

import { isTrustedHostMessage } from "@/host/bridge";

describe("父子页面消息来源校验", () => {
    it("只接受同源父窗口", () => {
        expect(isTrustedHostMessage({ origin: window.location.origin, source: window.parent })).toBe(true);
        expect(isTrustedHostMessage({ origin: "https://attacker.example", source: window.parent })).toBe(false);
        expect(isTrustedHostMessage({ origin: window.location.origin, source: null })).toBe(false);
    });
});
