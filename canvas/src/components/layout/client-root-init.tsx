import type { ReactNode } from "react";
import { useEffect, useState } from "react";

import { clearHostCredentials, initializeHostBridge, useHostStore } from "@/host/bridge";
import { initializeHostModels, stripRuntimeKeys } from "@/host/models";
import { changeAppLocale } from "@/i18n";
import { useCanvasStore } from "@/stores/canvas/use-canvas-store";
import { useAssetStore } from "@/stores/use-asset-store";
import { useConfigStore } from "@/stores/use-config-store";
import { usePromptSourceStore } from "@/stores/use-prompt-source-store";
import { useThemeStore } from "@/stores/use-theme-store";

export function ClientRootInit({ children }: { children: ReactNode }) {
    const [ready, setReady] = useState(false);
    const [error, setError] = useState("");
    const theme = useHostStore((state) => state.theme);
    const locale = useHostStore((state) => state.locale);

    useEffect(() => {
        let active = true;
        initializeHostBridge()
            .then(async () => {
                await Promise.all([useCanvasStore.persist.rehydrate(), useAssetStore.persist.rehydrate(), useConfigStore.persist.rehydrate(), usePromptSourceStore.persist.rehydrate()]);
                await initializeHostModels();
                if (active) setReady(true);
            })
            .catch((reason) => {
                if (active) setError(reason instanceof Error ? reason.message : "画布初始化失败");
            });
        return () => {
            active = false;
            stripRuntimeKeys();
            clearHostCredentials();
        };
    }, []);

    useEffect(() => {
        useThemeStore.getState().setTheme(theme);
        void changeAppLocale(locale);
    }, [locale, theme]);

    if (error) return <div className="grid h-dvh place-items-center bg-background px-6 text-center text-sm text-red-600 dark:text-red-400">{error}</div>;
    if (!ready) return <div className="grid h-dvh place-items-center bg-background text-sm text-stone-500 dark:text-stone-400">正在加载无限画布...</div>;
    return <>{children}</>;
}
