import { create } from "zustand";

import { setHostUserId } from "@/host/runtime";

export type CanvasPlatform = "openai" | "gemini" | "antigravity" | "grok";
export type CanvasApiFormat = "openai" | "gemini";

export type CanvasGroup = {
    id: number;
    name: string;
    platform: CanvasPlatform;
    api_format: CanvasApiFormat;
};

export type CanvasBootstrap = { user_id: number; groups: CanvasGroup[] };
export type CanvasCredential = {
    group_id: number;
    platform: CanvasPlatform;
    api_format: CanvasApiFormat;
    base_url: string;
    api_key: string;
};
type HostTheme = "light" | "dark";
type HostLocale = "zh-CN" | "en-US";

type HostState = {
    bootstrap: CanvasBootstrap | null;
    theme: HostTheme;
    locale: HostLocale;
    initialized: boolean;
    setContext: (theme: HostTheme, locale: HostLocale) => void;
};

export const useHostStore = create<HostState>((set) => ({
    bootstrap: null,
    theme: "light",
    locale: "zh-CN",
    initialized: false,
    setContext: (theme, locale) => set({ theme, locale }),
}));

type PendingCredential = { resolve: (value: CanvasCredential) => void; reject: (reason: Error) => void; timer: number };
const pendingCredentials = new Map<string, PendingCredential>();
const credentialMemory = new Map<number, CanvasCredential>();
let initPromise: Promise<CanvasBootstrap> | null = null;
let resolveInit: ((value: CanvasBootstrap) => void) | null = null;
let rejectInit: ((reason: Error) => void) | null = null;

export function isTrustedHostMessage(event: Pick<MessageEvent, "origin" | "source">) {
    return event.origin === window.location.origin && event.source === window.parent;
}

function normalizeLocale(locale: unknown): HostLocale {
    return typeof locale === "string" && locale.toLowerCase().startsWith("en") ? "en-US" : "zh-CN";
}

function normalizeTheme(theme: unknown): HostTheme {
    return theme === "dark" ? "dark" : "light";
}

function onHostMessage(event: MessageEvent) {
    if (!isTrustedHostMessage(event) || !event.data || typeof event.data !== "object") return;
    const message = event.data as Record<string, unknown>;
    if (message.type === "canvas:context") {
        useHostStore.getState().setContext(normalizeTheme(message.theme), normalizeLocale(message.locale));
        return;
    }
    if (message.type === "canvas:init") {
        const bootstrap = message.bootstrap as CanvasBootstrap | undefined;
        if (!bootstrap || !Number.isInteger(bootstrap.user_id) || !Array.isArray(bootstrap.groups)) return;
        setHostUserId(bootstrap.user_id);
        credentialMemory.clear();
        useHostStore.setState({
            bootstrap,
            theme: normalizeTheme(message.theme),
            locale: normalizeLocale(message.locale),
            initialized: true,
        });
        resolveInit?.(bootstrap);
        resolveInit = null;
        rejectInit = null;
        return;
    }
    if (message.type === "canvas:credential") {
        const requestId = typeof message.request_id === "string" ? message.request_id : "";
        const pending = pendingCredentials.get(requestId);
        if (!pending) return;
        window.clearTimeout(pending.timer);
        pendingCredentials.delete(requestId);
        if (typeof message.error === "string" && message.error) {
            pending.reject(new Error(message.error));
            return;
        }
        const credential = message.credential as CanvasCredential | undefined;
        if (!credential || !Number.isInteger(credential.group_id) || typeof credential.api_key !== "string") {
            pending.reject(new Error("站内凭据响应无效"));
            return;
        }
        credentialMemory.set(credential.group_id, credential);
        pending.resolve(credential);
    }
}

function clearRuntimeCredentials() {
    credentialMemory.clear();
    for (const pending of pendingCredentials.values()) {
        window.clearTimeout(pending.timer);
        pending.reject(new Error("画布已卸载"));
    }
    pendingCredentials.clear();
}

export function initializeHostBridge() {
    if (initPromise) return initPromise;
    window.addEventListener("message", onHostMessage);
    window.addEventListener("pagehide", clearRuntimeCredentials, { once: true });
    initPromise = new Promise<CanvasBootstrap>((resolve, reject) => {
        resolveInit = resolve;
        rejectInit = reject;
        window.setTimeout(() => {
            if (useHostStore.getState().initialized) return;
            rejectInit?.(new Error("等待 Sub2API 初始化超时"));
            rejectInit = null;
            resolveInit = null;
        }, 15_000);
    });
    window.parent.postMessage({ type: "canvas:ready" }, window.location.origin);
    return initPromise;
}

export function resolveHostCredential(groupId: number) {
    const cached = credentialMemory.get(groupId);
    if (cached) return Promise.resolve(cached);
    const requestId = crypto.randomUUID();
    return new Promise<CanvasCredential>((resolve, reject) => {
        const timer = window.setTimeout(() => {
            pendingCredentials.delete(requestId);
            reject(new Error("解析站内凭据超时"));
        }, 15_000);
        pendingCredentials.set(requestId, { resolve, reject, timer });
        window.parent.postMessage({ type: "canvas:resolve-credential", request_id: requestId, group_id: groupId }, window.location.origin);
    });
}

export function clearHostCredentials() {
    clearRuntimeCredentials();
}
