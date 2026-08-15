import axios from "axios";
import { create } from "zustand";

import type { CanvasGroup } from "@/host/bridge";
import { resolveHostCredential, useHostStore } from "@/host/bridge";
import { getCanvasSelectionKey } from "@/host/runtime";
import { encodeChannelModel, guessCapability, modelOptionsFromChannels, type ModelCapability, type ModelChannel, useConfigStore } from "@/stores/use-config-store";

type DiscoveredModel = { name: string; capability: ModelCapability };
type SavedSelection = { groupId?: number; image?: string; video?: string; text?: string };

type HostModelState = {
    selectedGroupId: number | null;
    loading: boolean;
    error: string;
    selectGroup: (groupId: number) => Promise<void>;
    selectModel: (capability: "image" | "video" | "text", value: string) => void;
};

const IMAGE_ALIASES = ["gpt-image", "dall-e", "dalle", "imagen", "nano-banana", "nano_banana", "seedream", "flux", "sdxl", "image"];
const VIDEO_ALIASES = ["sora", "veo", "video", "kling", "seedance", "hailuo", "wan2", "wan-"];

function readSelection(): SavedSelection {
    try {
        return JSON.parse(localStorage.getItem(getCanvasSelectionKey()) || "{}") as SavedSelection;
    } catch {
        return {};
    }
}

function saveSelection(patch: Partial<SavedSelection>) {
    localStorage.setItem(getCanvasSelectionKey(), JSON.stringify({ ...readSelection(), ...patch }));
}

function modelCapability(model: Record<string, unknown>, group: CanvasGroup): ModelCapability {
    const name = String(model.id || model.name || "")
        .replace(/^models\//, "")
        .toLowerCase();
    const metadata = JSON.stringify({
        capabilities: model.capabilities,
        capability: model.capability,
        supported_generation_methods: model.supportedGenerationMethods || model.supported_generation_methods,
        type: model.type,
    }).toLowerCase();
    if (VIDEO_ALIASES.some((alias) => name.includes(alias)) || /video/.test(metadata)) return "video";
    if (IMAGE_ALIASES.some((alias) => name.includes(alias)) || /image|vision_generation/.test(metadata)) return "image";
    const guessed = guessCapability(name);
    if (guessed === "audio") return "text";
    if (group.api_format === "gemini" && guessed === "video") return "text";
    return guessed;
}

export function classifyCanvasModels(rawModels: unknown[], group: CanvasGroup): DiscoveredModel[] {
    const seen = new Set<string>();
    return rawModels.flatMap((entry): DiscoveredModel[] => {
        const record = entry && typeof entry === "object" ? (entry as Record<string, unknown>) : { id: entry };
        const name = String(record.id || record.name || "")
            .replace(/^models\//, "")
            .trim();
        if (!name || seen.has(name)) return [];
        seen.add(name);
        const capability = modelCapability(record, group);
        if (capability === "audio" || (group.api_format === "gemini" && capability === "video")) return [];
        return [{ name, capability }];
    });
}

async function discoverModels(group: CanvasGroup, baseUrl: string, apiKey: string) {
    const root = baseUrl.trim().replace(/\/+$/, "");
    if (group.api_format === "gemini") {
        const url = root.endsWith("/v1beta") ? `${root}/models` : `${root}/v1beta/models`;
        const response = await axios.get<{ models?: unknown[] }>(url, { headers: { "x-goog-api-key": apiKey } });
        return classifyCanvasModels(response.data.models || [], group);
    }
    const url = root.endsWith("/v1") ? `${root}/models` : `${root}/v1/models`;
    const response = await axios.get<{ data?: unknown[] }>(url, { headers: { Authorization: `Bearer ${apiKey}` } });
    return classifyCanvasModels(response.data.data || [], group);
}

function selectedValue(channel: ModelChannel, models: DiscoveredModel[], capability: "image" | "video" | "text", saved?: string) {
    const match = models.find((model) => model.capability === capability && model.name === saved) || models.find((model) => model.capability === capability);
    return match ? encodeChannelModel(channel.id, match.name) : "";
}

async function loadGroup(groupId: number) {
    const bootstrap = useHostStore.getState().bootstrap;
    const group = bootstrap?.groups.find((item) => item.id === groupId);
    if (!group) throw new Error("所选分组不可用");
    const credential = await resolveHostCredential(groupId);
    const models = await discoverModels(group, credential.base_url || "/", credential.api_key);
    const channel: ModelChannel = {
        id: String(group.id),
        name: group.name,
        baseUrl: credential.base_url || "/",
        apiKey: credential.api_key,
        apiFormat: group.api_format,
        models,
    };
    const saved = readSelection();
    const imageModel = selectedValue(channel, models, "image", saved.image);
    const videoModel = selectedValue(channel, models, "video", saved.video);
    const textModel = selectedValue(channel, models, "text", saved.text);
    const options = modelOptionsFromChannels([channel]);
    useConfigStore.setState((state) => ({
        config: {
            ...state.config,
            channelMode: "local",
            baseUrl: channel.baseUrl,
            apiKey: channel.apiKey,
            apiFormat: channel.apiFormat,
            channels: [channel],
            models: options,
            imageModel,
            videoModel,
            textModel,
            model: imageModel || textModel || videoModel,
        },
    }));
    saveSelection({ groupId });
}

export const useHostModelStore = create<HostModelState>((set) => ({
    selectedGroupId: null,
    loading: false,
    error: "",
    selectGroup: async (groupId) => {
        set({ selectedGroupId: groupId, loading: true, error: "" });
        try {
            await loadGroup(groupId);
        } catch (error) {
            set({ error: error instanceof Error ? error.message : "模型加载失败" });
            throw error;
        } finally {
            set({ loading: false });
        }
    },
    selectModel: (capability, value) => {
        const key = capability === "image" ? "imageModel" : capability === "video" ? "videoModel" : "textModel";
        useConfigStore.getState().updateConfig(key, value);
        const name = value.includes("::") ? value.slice(value.indexOf("::") + 2) : value;
        saveSelection({ [capability]: name });
    },
}));

export async function initializeHostModels() {
    const bootstrap = useHostStore.getState().bootstrap;
    if (!bootstrap?.groups.length) throw new Error("当前账号没有可用于无限画布的分组");
    const saved = readSelection();
    const groupId = bootstrap.groups.some((group) => group.id === saved.groupId) ? saved.groupId! : bootstrap.groups[0].id;
    await useHostModelStore.getState().selectGroup(groupId);
}

export function stripRuntimeKeys() {
    useConfigStore.setState((state) => ({
        config: {
            ...state.config,
            apiKey: "",
            channels: state.config.channels.map((channel) => ({ ...channel, apiKey: "" })),
        },
    }));
}
