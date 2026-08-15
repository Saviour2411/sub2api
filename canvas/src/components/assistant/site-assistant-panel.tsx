import { useEffect, useMemo, useState } from "react";
import { App, Button, Drawer, Input, Popconfirm, Spin } from "antd";
import { Bot, Send, X } from "lucide-react";
import { nanoid } from "nanoid";

import { requestImageQuestion, type AiTextMessage } from "@/services/api/image";
import { canvasLocalForage } from "@/lib/localforage-storage";
import { sanitizeCanvasAssistantOps, summarizeCanvasAgentOps, type CanvasAgentOp } from "@/lib/canvas/canvas-agent-ops";
import { buildAssistantUserContent } from "@/lib/canvas/canvas-assistant-context";
import { useCanvasAssistantContextStore } from "@/stores/use-canvas-assistant-context-store";
import { useConfigStore } from "@/stores/use-config-store";
import { useSiteAssistantStore } from "@/stores/use-site-assistant-store";

type ChatMessage = { id: string; role: "user" | "assistant"; text: string };
type Proposal = { message: string; safe: CanvasAgentOp[]; paid: CanvasAgentOp[] };

function sessionsStore() {
    return canvasLocalForage("assistant_sessions");
}

export function SiteAssistantPanel() {
    const { message } = App.useApp();
    const open = useSiteAssistantStore((state) => state.open);
    const setOpen = useSiteAssistantStore((state) => state.setOpen);
    const canvasContext = useCanvasAssistantContextStore((state) => state.context);
    const config = useConfigStore((state) => state.config);
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [draft, setDraft] = useState("");
    const [streaming, setStreaming] = useState("");
    const [sending, setSending] = useState(false);
    const [proposal, setProposal] = useState<Proposal | null>(null);
    const projectId = canvasContext?.snapshot.projectId || "";

    useEffect(() => {
        setProposal(null);
        if (!projectId) return setMessages([]);
        void sessionsStore()
            .getItem<ChatMessage[]>(projectId)
            .then((items) => setMessages(items || []));
    }, [projectId]);

    useEffect(() => {
        if (projectId) void sessionsStore().setItem(projectId, messages);
    }, [messages, projectId]);

    const canvasSummary = useMemo(() => {
        const snapshot = canvasContext?.snapshot;
        if (!snapshot) return "当前未打开画布项目";
        return JSON.stringify({
            projectId: snapshot.projectId,
            title: snapshot.title,
            selectedNodeIds: snapshot.selectedNodeIds,
            nodes: snapshot.nodes.map((node) => ({ id: node.id, type: node.type, title: node.title, text: String(node.metadata?.content || node.metadata?.prompt || "").slice(0, 500) })),
            connections: snapshot.connections.map((connection) => ({ from: connection.fromNodeId, to: connection.toNodeId })),
        });
    }, [canvasContext?.snapshot]);

    const send = async () => {
        const text = draft.trim();
        if (!text || !canvasContext || !config.textModel) return;
        const userMessage: ChatMessage = { id: nanoid(), role: "user", text };
        setMessages((items) => [...items, userMessage]);
        setDraft("");
        setProposal(null);
        setStreaming("");
        setSending(true);
        try {
            const requestConfig = { ...config, model: config.textModel };
            const history: AiTextMessage[] = messages.slice(-10).map((item) => ({ role: item.role, content: item.text }));
            const userContent = await buildAssistantUserContent(text, canvasContext.snapshot);
            const response = await requestImageQuestion(
                requestConfig,
                [
                    {
                        role: "system",
                        content: `你是 Sub2API 无限画布助手。根据画布摘要回答，并且只能提议以下操作：add_node（仅 text/config）、update_node（仅标题、文本和生成配置）、connect_nodes、select_nodes、set_viewport、run_generation（仅 image/video）。禁止删除、脚本、插件和 URL。必须只返回 JSON：{"message":"说明","operations":[]}。画布摘要：${canvasSummary}`,
                    },
                    ...history,
                    { role: "user", content: userContent },
                ],
                setStreaming,
            );
            const parsed = parseAssistantResponse(response);
            const operations = sanitizeCanvasAssistantOps(parsed.operations);
            const next = { message: parsed.message || response, safe: operations.filter((op) => op.type !== "run_generation"), paid: operations.filter((op) => op.type === "run_generation") };
            setMessages((items) => [...items, { id: nanoid(), role: "assistant", text: next.message }]);
            setProposal(next.safe.length || next.paid.length ? next : null);
        } catch (error) {
            message.error(error instanceof Error ? error.message : "助手请求失败");
        } finally {
            setSending(false);
            setStreaming("");
        }
    };

    const applyOps = (ops: CanvasAgentOp[]) => {
        canvasContext?.applyOps(ops);
        setProposal(null);
        message.success("画布操作已执行");
    };

    return (
        <Drawer
            title={
                <span className="inline-flex items-center gap-2">
                    <Bot className="size-4" />
                    画布助手
                </span>
            }
            open={open}
            width={420}
            closeIcon={<X className="size-4" />}
            onClose={() => setOpen(false)}
        >
            <div className="flex h-full min-h-0 flex-col">
                <div className="min-h-0 flex-1 space-y-3 overflow-y-auto pb-4">
                    {!projectId ? <div className="rounded-md border border-dashed p-4 text-sm text-stone-500">打开一个画布项目后即可使用助手。</div> : null}
                    {messages.map((item) => (
                        <div key={item.id} className={item.role === "user" ? "ml-8 rounded-md bg-stone-100 p-3 text-sm dark:bg-stone-800" : "mr-8 whitespace-pre-wrap p-3 text-sm"}>
                            {item.text}
                        </div>
                    ))}
                    {streaming ? <div className="mr-8 whitespace-pre-wrap p-3 text-sm">{streaming}</div> : null}
                    {sending && !streaming ? <Spin size="small" /> : null}
                    {proposal ? (
                        <div className="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm dark:border-amber-800 dark:bg-amber-950/30">
                            <div className="font-medium">待确认操作</div>
                            {proposal.safe.length ? <p className="mt-1">{summarizeCanvasAgentOps(proposal.safe)}</p> : null}
                            {proposal.paid.length ? <p className="mt-1 text-amber-700 dark:text-amber-300">付费生成：{summarizeCanvasAgentOps(proposal.paid)}</p> : null}
                            <div className="mt-3 flex gap-2">
                                {proposal.safe.length ? (
                                    <Button size="small" onClick={() => applyOps(proposal.safe)}>
                                        确认编辑
                                    </Button>
                                ) : null}
                                {proposal.paid.length ? (
                                    <Popconfirm title="媒体生成将消耗站内额度，确认继续？" okText="确认生成" cancelText="取消" onConfirm={() => applyOps(proposal.paid)}>
                                        <Button size="small" type="primary">
                                            确认并生成
                                        </Button>
                                    </Popconfirm>
                                ) : null}
                            </div>
                        </div>
                    ) : null}
                </div>
                <div className="border-t pt-3">
                    <Input.TextArea
                        value={draft}
                        rows={3}
                        disabled={!projectId || sending}
                        placeholder="输入你的画布修改需求"
                        onChange={(event) => setDraft(event.target.value)}
                        onPressEnter={(event) => {
                            if (!event.shiftKey) {
                                event.preventDefault();
                                void send();
                            }
                        }}
                    />
                    <Button className="mt-2 w-full" type="primary" icon={<Send className="size-4" />} loading={sending} disabled={!projectId || !draft.trim()} onClick={() => void send()}>
                        发送
                    </Button>
                </div>
            </div>
        </Drawer>
    );
}

function parseAssistantResponse(text: string): { message?: string; operations?: unknown } {
    const candidate = text.match(/```(?:json)?\s*([\s\S]*?)```/i)?.[1] || text;
    try {
        return JSON.parse(candidate.trim()) as { message?: string; operations?: unknown };
    } catch {
        return { message: text, operations: [] };
    }
}
