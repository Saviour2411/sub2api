import { BookOpenText, FolderOpen, Maximize2 } from "lucide-react";
import { Link, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { StationModelControls } from "@/components/layout/station-model-controls";
import { cn } from "@/lib/utils";

const links = [
    { to: "/canvas", icon: Maximize2, key: "canvas" },
    { to: "/assets", icon: FolderOpen, key: "assets" },
    { to: "/prompts", icon: BookOpenText, key: "prompts" },
] as const;

export function AppTopNav() {
    const { t } = useTranslation();
    const location = useLocation();
    return (
        <header className="z-40 flex h-12 shrink-0 items-center gap-3 border-b border-stone-200 bg-white px-3 dark:border-stone-800 dark:bg-stone-950">
            <nav className="flex shrink-0 items-center gap-1" aria-label={t("navigation.canvas")}>
                {links.map(({ to, icon: Icon, key }) => {
                    const active = location.pathname === to || location.pathname.startsWith(`${to}/`);
                    return (
                        <Link
                            key={to}
                            to={to}
                            title={t(`navigation.${key}`)}
                            aria-label={t(`navigation.${key}`)}
                            className={cn(
                                "grid size-8 place-items-center rounded-md text-stone-500 transition hover:bg-stone-100 hover:text-stone-950 dark:text-stone-400 dark:hover:bg-stone-800 dark:hover:text-white",
                                active && "bg-stone-100 text-stone-950 dark:bg-stone-800 dark:text-white",
                            )}
                        >
                            <Icon className="size-4" />
                        </Link>
                    );
                })}
            </nav>
            <div className="min-w-0 flex-1 overflow-hidden">
                <StationModelControls />
            </div>
        </header>
    );
}
