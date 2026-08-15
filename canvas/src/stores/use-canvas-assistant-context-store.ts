import { create } from "zustand";

import type { CanvasAgentOp, CanvasAgentSnapshot } from "@/lib/canvas/canvas-agent-ops";

export type CanvasAssistantContext = {
    snapshot: CanvasAgentSnapshot;
    applyOps: (ops?: CanvasAgentOp[]) => CanvasAgentSnapshot;
    undoOps: () => CanvasAgentSnapshot | null;
    canUndo: boolean;
};

type CanvasAssistantContextStore = {
    context: CanvasAssistantContext | null;
    setContext: (context: CanvasAssistantContext | null) => void;
};

export const useCanvasAssistantContextStore = create<CanvasAssistantContextStore>((set) => ({
    context: null,
    setContext: (context) => set({ context }),
}));
