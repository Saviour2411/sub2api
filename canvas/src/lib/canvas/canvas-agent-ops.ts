import { nanoid } from "nanoid";

import i18n from "@/i18n";
import { getNodeSpec, isRegisteredNodeType } from "@/lib/canvas/node-registry";
import { CanvasNodeType, type CanvasConnection, type CanvasNodeData, type CanvasNodeMetadata, type CanvasNodeTypeId, type ViewportTransform } from "@/types/canvas";

export type CanvasAgentOp =
    | { type: "add_node"; id?: string; nodeType?: CanvasNodeTypeId; title?: string; position?: { x: number; y: number }; x?: number; y?: number; width?: number; height?: number; metadata?: CanvasNodeMetadata }
    | { type: "update_node"; id: string; patch?: Partial<CanvasNodeData>; metadata?: CanvasNodeMetadata }
    | { type: "delete_node"; id?: string; ids?: string[]; nodeType?: CanvasNodeTypeId }
    | { type: "delete_connections"; id?: string; ids?: string[]; all?: boolean }
    | { type: "connect_nodes"; id?: string; fromNodeId: string; toNodeId: string }
    | { type: "set_viewport"; viewport: ViewportTransform }
    | { type: "select_nodes"; ids: string[] }
    | { type: "run_generation"; nodeId: string; mode?: "text" | "image" | "video" | "audio"; prompt?: string };

export type CanvasAgentSnapshot = {
    projectId: string;
    title: string;
    nodes: CanvasNodeData[];
    connections: CanvasConnection[];
    selectedNodeIds: string[];
    viewport: ViewportTransform;
};

const assistantMetadataKeys = new Set(["content", "prompt", "composerContent", "generationMode", "model"]);

/** 将模型输出收敛到站内画布助手允许的非破坏性操作。 */
export function sanitizeCanvasAssistantOps(value: unknown): CanvasAgentOp[] {
    if (!Array.isArray(value)) return [];
    return value.flatMap((entry): CanvasAgentOp[] => {
        if (!entry || typeof entry !== "object") return [];
        const op = entry as CanvasAgentOp;
        if (op.type === "add_node") {
            const nodeType = op.nodeType === CanvasNodeType.Config ? CanvasNodeType.Config : CanvasNodeType.Text;
            return [{ type: "add_node", id: op.id, nodeType, title: cleanText(op.title, 100), position: cleanPosition(op.position), metadata: cleanAssistantMetadata(op.metadata) }];
        }
        if (op.type === "update_node" && typeof op.id === "string") {
            return [{ type: "update_node", id: op.id, patch: op.patch?.title ? { title: cleanText(op.patch.title, 100) } : undefined, metadata: cleanAssistantMetadata({ ...op.patch?.metadata, ...op.metadata }) }];
        }
        if (op.type === "connect_nodes" && typeof op.fromNodeId === "string" && typeof op.toNodeId === "string") {
            return [{ type: "connect_nodes", fromNodeId: op.fromNodeId, toNodeId: op.toNodeId }];
        }
        if (op.type === "select_nodes" && Array.isArray(op.ids)) return [{ type: "select_nodes", ids: op.ids.filter((id): id is string => typeof id === "string").slice(0, 100) }];
        if (op.type === "set_viewport" && op.viewport) return [{ type: "set_viewport", viewport: { x: finite(op.viewport.x), y: finite(op.viewport.y), k: Math.max(0.1, Math.min(4, finite(op.viewport.k, 1))) } }];
        if (op.type === "run_generation" && typeof op.nodeId === "string" && ["image", "video"].includes(op.mode || "")) {
            return [{ type: "run_generation", nodeId: op.nodeId, mode: op.mode as "image" | "video", prompt: cleanText(op.prompt, 8000) }];
        }
        return [];
    });
}

function cleanAssistantMetadata(value: CanvasNodeMetadata | undefined): CanvasNodeMetadata {
    const result: Record<string, unknown> = {};
    Object.entries(value || {}).forEach(([key, item]) => {
        if (assistantMetadataKeys.has(key) && (typeof item === "string" || typeof item === "number" || typeof item === "boolean")) result[key] = typeof item === "string" ? cleanText(item, 20_000) : item;
    });
    return result as CanvasNodeMetadata;
}

function cleanText(value: unknown, maxLength: number) {
    return typeof value === "string" ? value.slice(0, maxLength) : undefined;
}

function cleanPosition(value: { x: number; y: number } | undefined) {
    return value ? { x: finite(value.x), y: finite(value.y) } : undefined;
}

function finite(value: unknown, fallback = 0) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
}

export function summarizeCanvasAgentOps(ops?: CanvasAgentOp[]) {
    const counts = (Array.isArray(ops) ? ops : []).reduce<Record<string, number>>((acc, op) => {
        if (!op?.type) return acc;
        acc[op.type] = (acc[op.type] || 0) + 1;
        return acc;
    }, {});
    return Object.entries(counts)
        .map(([type, count]) => `${opLabel(type)} ${count}`)
        .join("，");
}

export function applyCanvasAgentOps(snapshot: CanvasAgentSnapshot, ops?: CanvasAgentOp[]) {
    let nodes = snapshot.nodes;
    let connections = snapshot.connections;
    let selectedNodeIds = snapshot.selectedNodeIds;
    let viewport = snapshot.viewport;

    (Array.isArray(ops) ? ops : []).forEach((op, index) => {
        if (!op?.type) return;
        if (op.type === "add_node") {
            const nodeType = op.nodeType && isRegisteredNodeType(op.nodeType) ? op.nodeType : CanvasNodeType.Text;
            const spec = getNodeSpec(nodeType);
            const node: CanvasNodeData = {
                id: op.id || `${nodeType}-${Date.now()}-${index}`,
                type: nodeType,
                title: op.title || spec.title,
                position: op.position || { x: op.x ?? index * 36, y: op.y ?? index * 36 },
                width: op.width || spec.width,
                height: op.height || spec.height,
                metadata: { ...spec.metadata, ...op.metadata },
            };
            nodes = [...nodes, node];
            selectedNodeIds = [node.id];
        }
        if (op.type === "update_node") {
            if (!op.id) return;
            nodes = nodes.map((node) => (node.id === op.id ? { ...node, ...op.patch, metadata: { ...node.metadata, ...op.patch?.metadata, ...op.metadata } } : node));
        }
        if (op.type === "delete_node") {
            const ids = new Set(op.ids || (op.id ? [op.id] : op.nodeType ? nodes.filter((node) => node.type === op.nodeType).map((node) => node.id) : []));
            nodes = nodes.filter((node) => !ids.has(node.id));
            connections = connections.filter((conn) => !ids.has(conn.fromNodeId) && !ids.has(conn.toNodeId));
            selectedNodeIds = selectedNodeIds.filter((id) => !ids.has(id));
        }
        if (op.type === "delete_connections") {
            const ids = new Set(op.ids || (op.id ? [op.id] : []));
            connections = op.all ? [] : connections.filter((conn) => !ids.has(conn.id));
        }
        if (op.type === "connect_nodes") {
            if (!op.fromNodeId || !op.toNodeId) return;
            const exists = connections.some((conn) => conn.fromNodeId === op.fromNodeId && conn.toNodeId === op.toNodeId);
            const hasNodes = nodes.some((node) => node.id === op.fromNodeId) && nodes.some((node) => node.id === op.toNodeId);
            if (!exists && hasNodes) connections = [...connections, { id: op.id || nanoid(), fromNodeId: op.fromNodeId, toNodeId: op.toNodeId }];
        }
        if (op.type === "set_viewport" && op.viewport) viewport = op.viewport;
        if (op.type === "select_nodes") selectedNodeIds = (op.ids || []).filter((id) => nodes.some((node) => node.id === id));
    });

    return { ...snapshot, nodes, connections, selectedNodeIds, viewport };
}

function opLabel(type: string) {
    return i18n.t(`canvas.agentOps.${type}`, { defaultValue: type });
}
