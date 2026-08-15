import type { ReactNode } from "react";

import { AppTopNav } from "@/components/layout/app-top-nav";
import { SiteAssistantPanel } from "@/components/assistant/site-assistant-panel";

export default function UserLayout({ children }: { children: ReactNode }) {
    return (
        <div className="flex h-dvh overflow-hidden bg-background text-foreground">
            <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
                <AppTopNav />
                <div className="min-h-0 flex-1 overflow-hidden">{children}</div>
            </div>
            <SiteAssistantPanel />
        </div>
    );
}
