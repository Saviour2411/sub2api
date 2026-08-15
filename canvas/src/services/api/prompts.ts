import { nanoid } from "nanoid";

import { canvasLocalForage } from "@/lib/localforage-storage";
import { BUILTIN_SOURCE_ID, CUSTOM_SOURCE_ID } from "@/services/api/prompt-source-presets";

export type Prompt = {
    id: string;
    title: string;
    prompt: string;
    description: string;
    coverUrl: string;
    referenceImageUrls: string[];
    tags: string[];
    preview: string;
    createdAt: string;
    updatedAt: string;
    author?: string;
    sourceUrl?: string;
    sourceId: string;
    category: string;
    githubUrl: string;
};

export const ALL_PROMPTS_OPTION = "all";
const CUSTOM_PROMPTS_KEY = "items";

const BUILTIN_PROMPTS: Prompt[] = [
    builtIn("product", "产品棚拍", "高端商业产品摄影，主体居中，柔和轮廓光，材质细节清晰，纯净背景，真实阴影，广告级质感", ["产品", "摄影"]),
    builtIn("portrait", "电影感人像", "电影感人物肖像，自然肤色，柔和侧光，浅景深，克制的色彩分级，真实镜头质感", ["人像", "摄影"]),
    builtIn("poster", "品牌海报", "现代品牌海报，清晰视觉层级，大面积留白，高对比排版，精确网格，适合商业发布", ["海报", "设计"]),
    builtIn("storyboard", "分镜画面", "电影分镜画面，明确主体动作与镜头语言，环境关系清楚，构图可执行，光线方向一致", ["视频", "分镜"]),
    builtIn("architecture", "建筑可视化", "写实建筑可视化，准确透视，自然日照，真实材质与环境反射，尺度关系清晰", ["建筑", "写实"]),
    builtIn("food", "美食摄影", "精致美食摄影，食材纹理真实，柔和窗光，干净桌面陈设，自然色彩，近景构图", ["美食", "摄影"]),
    builtIn("illustration", "编辑插画", "现代编辑插画，清晰叙事主体，有限但有层次的配色，细腻纹理，适合文章配图", ["插画", "编辑"]),
    builtIn("video", "短视频镜头", "稳定流畅的短视频镜头，主体动作自然，镜头缓慢推进，光线连续，无闪烁和形变", ["视频", "运镜"]),
];

function builtIn(id: string, title: string, prompt: string, tags: string[]): Prompt {
    return {
        id,
        title,
        prompt,
        description: "Sub2API 内置提示词",
        coverUrl: "",
        referenceImageUrls: [],
        tags,
        preview: "",
        createdAt: "",
        updatedAt: "",
        sourceId: BUILTIN_SOURCE_ID,
        category: "内置提示词",
        githubUrl: "",
    };
}

function customStore() {
    return canvasLocalForage("custom_prompts");
}

async function readCustomPrompts() {
    return (await customStore().getItem<Prompt[]>(CUSTOM_PROMPTS_KEY)) || [];
}

export async function addCustomPrompt(input: { title: string; prompt: string; tags?: string[] }) {
    const now = new Date().toISOString();
    const item: Prompt = {
        id: nanoid(),
        title: input.title.trim(),
        prompt: input.prompt.trim(),
        description: "浏览器本地自定义提示词",
        coverUrl: "",
        referenceImageUrls: [],
        tags: (input.tags || []).map((tag) => tag.trim()).filter(Boolean),
        preview: "",
        createdAt: now,
        updatedAt: now,
        sourceId: CUSTOM_SOURCE_ID,
        category: "我的提示词",
        githubUrl: "",
    };
    const items = await readCustomPrompts();
    await customStore().setItem(CUSTOM_PROMPTS_KEY, [item, ...items]);
    return item;
}

export async function removeCustomPrompt(id: string) {
    await customStore().setItem(
        CUSTOM_PROMPTS_KEY,
        (await readCustomPrompts()).filter((item) => item.id !== id),
    );
}

async function allPrompts() {
    return [...BUILTIN_PROMPTS, ...(await readCustomPrompts())];
}

export async function fetchPrompts({ keyword = "", tag = [], category = ALL_PROMPTS_OPTION, page = 1, pageSize = 20 }: { keyword?: string; tag?: string[]; category?: string; page?: number; pageSize?: number } = {}) {
    const normalizedKeyword = keyword.trim().toLowerCase();
    const all = await allPrompts();
    const filtered = all.filter((item) => {
        if (category !== ALL_PROMPTS_OPTION && item.category !== category) return false;
        if (tag.length && !tag.some((value) => item.tags.includes(value))) return false;
        return !normalizedKeyword || [item.title, item.prompt, item.description, ...item.tags].join(" ").toLowerCase().includes(normalizedKeyword);
    });
    const start = (Math.max(1, page) - 1) * pageSize;
    return {
        items: filtered.slice(start, start + pageSize),
        tags: Array.from(new Set(all.flatMap((item) => item.tags))),
        categories: ["内置提示词", "我的提示词"],
        total: filtered.length,
    };
}

export async function fetchSourcePrompts(sourceId: string) {
    if (sourceId === BUILTIN_SOURCE_ID) return BUILTIN_PROMPTS;
    if (sourceId === CUSTOM_SOURCE_ID) return readCustomPrompts();
    return [];
}

export async function refreshSource(sourceId: string) {
    const items = await fetchSourcePrompts(sourceId);
    return { sourceId, sourceName: sourceId === CUSTOM_SOURCE_ID ? "我的提示词" : "内置提示词", count: items.length, lastSuccessAt: new Date().toISOString(), lastError: "", success: true };
}

export async function refreshAllSources() {
    const total = (await allPrompts()).length;
    return { results: [], total, successCount: 2, failureCount: 0 };
}

export const refreshDueSources = async (_maxAgeMs?: number) => refreshAllSources();
export const fetchPromptSourceStatuses = async (): Promise<Record<string, { sourceId: string; count: number; lastSuccessAt: string; lastError: string }>> => ({});

export function formatPromptDate(value: string, locale?: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "" : new Intl.DateTimeFormat(locale, { year: "numeric", month: "2-digit", day: "2-digit" }).format(date);
}
