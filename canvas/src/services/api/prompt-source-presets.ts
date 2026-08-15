import { nanoid } from "nanoid";

export type PromptSource = {
    id: string;
    name: string;
    url: string;
    homepage: string;
    enabled: boolean;
    builtIn: boolean;
};

export const BUILTIN_SOURCE_ID = "sub2api-built-in";
export const CUSTOM_SOURCE_ID = "sub2api-custom";

export function createPromptSource(source?: Partial<PromptSource>): PromptSource {
    return {
        id: source?.id?.trim() || nanoid(),
        name: source?.name?.trim() || "本地提示词",
        url: "",
        homepage: "",
        enabled: source?.enabled ?? true,
        builtIn: source?.builtIn ?? false,
    };
}

export const DEFAULT_PROMPT_SOURCES: PromptSource[] = [
    { id: BUILTIN_SOURCE_ID, name: "内置提示词", url: "", homepage: "", enabled: true, builtIn: true },
    { id: CUSTOM_SOURCE_ID, name: "我的提示词", url: "", homepage: "", enabled: true, builtIn: true },
];
