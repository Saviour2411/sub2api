import { createBrowserRouter, Navigate, Outlet } from "react-router-dom";

import UserLayout from "@/layouts/user-layout";
import AssetsPage from "@/pages/assets";
import CanvasPage from "@/pages/canvas";
import CanvasProjectPage from "@/pages/canvas/project";
import PromptsPage from "@/pages/prompts";

export const router = createBrowserRouter(
    [
        {
            element: (
                <UserLayout>
                    <Outlet />
                </UserLayout>
            ),
            children: [
                { path: "/", element: <Navigate to="/canvas" replace /> },
                { path: "/canvas", element: <CanvasPage /> },
                { path: "/canvas/:id", element: <CanvasProjectPage /> },
                { path: "/assets", element: <AssetsPage /> },
                { path: "/prompts", element: <PromptsPage /> },
                { path: "*", element: <Navigate to="/canvas" replace /> },
            ],
        },
    ],
    { basename: "/canvas-app" },
);
