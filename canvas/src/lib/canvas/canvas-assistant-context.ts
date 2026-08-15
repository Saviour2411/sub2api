import { imageToDataUrl } from "@/services/image-storage";
import type { AiTextMessage } from "@/services/api/image";
import type { CanvasAgentSnapshot } from "@/lib/canvas/canvas-agent-ops";
import { CanvasNodeType } from "@/types/canvas";

const MAX_SELECTED_IMAGES = 2;

export async function buildAssistantUserContent(text: string, snapshot: CanvasAgentSnapshot): Promise<AiTextMessage["content"]> {
    const selectedIds = new Set(snapshot.selectedNodeIds);
    const selectedImages = snapshot.nodes.filter((node) => selectedIds.has(node.id) && node.type === CanvasNodeType.Image && (node.metadata?.storageKey || isLocalImageUrl(node.metadata?.content))).slice(0, MAX_SELECTED_IMAGES);

    const images = await Promise.all(
        selectedImages.map(async (node) => {
            const content = node.metadata?.content || "";
            const dataUrl = await imageToDataUrl({
                storageKey: node.metadata?.storageKey,
                dataUrl: content.startsWith("data:image/") ? content : undefined,
                url: content.startsWith("blob:") ? content : undefined,
            });
            return dataUrl.startsWith("data:image/") ? dataUrl : "";
        }),
    );
    const validImages = images.filter(Boolean);
    if (!validImages.length) return text;
    return [{ type: "text", text }, ...validImages.map((url) => ({ type: "image_url" as const, image_url: { url } }))];
}

function isLocalImageUrl(value: string | undefined) {
    return Boolean(value?.startsWith("data:image/") || value?.startsWith("blob:"));
}
