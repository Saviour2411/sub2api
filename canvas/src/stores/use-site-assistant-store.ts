import { create } from "zustand";

type SiteAssistantState = { open: boolean; setOpen: (open: boolean) => void; toggle: () => void };

export const useSiteAssistantStore = create<SiteAssistantState>((set) => ({
    open: false,
    setOpen: (open) => set({ open }),
    toggle: () => set((state) => ({ open: !state.open })),
}));
