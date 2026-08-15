import type { ReactNode } from "react";
import { Alert, Select, Spin, Tooltip } from "antd";
import { Image, MessageSquareText, Video } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useHostStore } from "@/host/bridge";
import { useHostModelStore } from "@/host/models";
import { modelOptionName, selectableModelsByCapability, useConfigStore } from "@/stores/use-config-store";

export function StationModelControls() {
    const { t } = useTranslation();
    const groups = useHostStore((state) => state.bootstrap?.groups || []);
    const selectedGroupId = useHostModelStore((state) => state.selectedGroupId);
    const loading = useHostModelStore((state) => state.loading);
    const error = useHostModelStore((state) => state.error);
    const selectGroup = useHostModelStore((state) => state.selectGroup);
    const selectModel = useHostModelStore((state) => state.selectModel);
    const config = useConfigStore((state) => state.config);

    const selector = (capability: "image" | "video" | "text", icon: ReactNode) => {
        const values = selectableModelsByCapability(config, capability);
        if (!values.length) return null;
        const value = capability === "image" ? config.imageModel : capability === "video" ? config.videoModel : config.textModel;
        return (
            <Tooltip title={t(`hostModels.${capability}`)}>
                <Select
                    aria-label={t(`hostModels.${capability}`)}
                    size="small"
                    className="w-40 shrink-0"
                    prefix={icon}
                    value={value || undefined}
                    options={values.map((item) => ({ value: item, label: modelOptionName(item) }))}
                    onChange={(next) => selectModel(capability, next)}
                />
            </Tooltip>
        );
    };

    return (
        <div className="flex min-w-0 items-center gap-2 overflow-x-auto py-1">
            <Select
                aria-label={t("hostModels.group")}
                size="small"
                className="w-36 shrink-0"
                value={selectedGroupId || undefined}
                loading={loading}
                options={groups.map((group) => ({ value: group.id, label: group.name }))}
                onChange={(groupId) => void selectGroup(groupId)}
            />
            {loading ? <Spin size="small" /> : null}
            {!loading ? selector("image", <Image className="size-3.5" />) : null}
            {!loading ? selector("video", <Video className="size-3.5" />) : null}
            {!loading ? selector("text", <MessageSquareText className="size-3.5" />) : null}
            {error ? <Alert type="error" showIcon={false} message={error} className="!py-0.5" /> : null}
        </div>
    );
}
