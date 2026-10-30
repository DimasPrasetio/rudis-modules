import { lazy } from "react";
import { Package } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const ItemsPage = lazy(() => import("@/modules/items/pages/ItemsPage"));
const ItemPage = lazy(() => import("@/modules/items/pages/ItemPage"));

export const itemsModule: FrontendModule = {
  id: "items",
  protected: [
    { path: "/items", element: <ItemsPage /> },
    { path: "/items/:id", element: <ItemPage /> },
  ],
  // "Persediaan" takes 300–340: items first, then stock's pages.
  nav: [{ label: "Barang", to: "/items", icon: Package, group: "Persediaan", order: 300, permission: "items.read" }],
};
