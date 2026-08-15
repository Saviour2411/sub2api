import localforage from "localforage";
import type { StateStorage } from "zustand/middleware";
import { getCanvasDatabaseName, getCanvasFallbackStorageKey } from "@/host/runtime";

let activeDatabase = "";
let activeStore: LocalForage | null = null;

export function canvasLocalForage(storeName: string) {
    const database = getCanvasDatabaseName();
    return localforage.createInstance({ name: database, storeName });
}

function appStateStore() {
    const database = getCanvasDatabaseName();
    if (!activeStore || activeDatabase !== database) {
        activeDatabase = database;
        activeStore = canvasLocalForage("app_state");
    }
    return activeStore;
}

export const localForageStorage: StateStorage = {
    getItem: async (name) => {
        if (typeof window === "undefined") return null;
        try {
            return (await appStateStore().getItem<string>(name)) || null;
        } catch {
            return window.localStorage.getItem(getCanvasFallbackStorageKey(name));
        }
    },
    setItem: async (name, value) => {
        if (typeof window === "undefined") return;
        try {
            await appStateStore().setItem(name, value);
        } catch {
            window.localStorage.setItem(getCanvasFallbackStorageKey(name), value);
        }
    },
    removeItem: async (name) => {
        if (typeof window === "undefined") return;
        try {
            await appStateStore().removeItem(name);
        } catch {
            window.localStorage.removeItem(getCanvasFallbackStorageKey(name));
        }
    },
};
