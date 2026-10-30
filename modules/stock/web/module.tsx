import { lazy } from "react";
import { BarChart3, Boxes, FileStack, Warehouse } from "lucide-react";
import type { FrontendModule } from "@/shared/types/frontend-module";

const StockPage = lazy(() => import("@/modules/stock/pages/StockPage"));
const DocumentsPage = lazy(() => import("@/modules/stock/pages/DocumentsPage"));
const DocumentPage = lazy(() => import("@/modules/stock/pages/DocumentPage"));
const StockCardPage = lazy(() => import("@/modules/stock/pages/StockCardPage"));
const MovementReportPage = lazy(() => import("@/modules/stock/pages/MovementReportPage"));
const WarehousesPage = lazy(() => import("@/modules/stock/pages/WarehousesPage"));

export const stockModule: FrontendModule = {
  id: "stock",
  protected: [
    { path: "/stock", element: <StockPage /> },
    { path: "/stock/documents", element: <DocumentsPage /> },
    { path: "/stock/documents/new", element: <DocumentPage /> },
    { path: "/stock/documents/:id", element: <DocumentPage /> },
    { path: "/stock/items/:id", element: <StockCardPage /> },
    { path: "/stock/report", element: <MovementReportPage /> },
    { path: "/stock/warehouses", element: <WarehousesPage /> },
  ],
  // "Persediaan": items take 300; stock follows. Warehouses and the costing method are settings.
  nav: [
    { label: "Stok", to: "/stock", icon: Boxes, group: "Persediaan", order: 310, permission: "stock.read" },
    { label: "Dokumen stok", to: "/stock/documents", icon: FileStack, group: "Persediaan", order: 320, permission: "stock.read" },
    { label: "Laporan mutasi", to: "/stock/report", icon: BarChart3, group: "Persediaan", order: 330, permission: "stock.read" },
    { label: "Gudang", to: "/stock/warehouses", icon: Warehouse, group: "Pengaturan", order: 830, permission: "stock.read" },
  ],
};
