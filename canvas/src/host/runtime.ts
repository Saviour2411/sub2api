let currentUserId: number | null = null;

export function setHostUserId(userId: number) {
    if (!Number.isInteger(userId) || userId <= 0) throw new Error("无效的画布用户标识");
    if (currentUserId !== null && currentUserId !== userId) throw new Error("画布用户上下文已变更");
    currentUserId = userId;
}

export function getHostUserId() {
    if (currentUserId === null) throw new Error("画布尚未完成用户初始化");
    return currentUserId;
}

export function getCanvasDatabaseName() {
    return `sub2api-infinite-canvas:${getHostUserId()}`;
}

export function getCanvasSelectionKey() {
    return getCanvasFallbackStorageKey("model-selection");
}

export function getCanvasFallbackStorageKey(name: string) {
    return `${getCanvasDatabaseName()}:${name}`;
}
