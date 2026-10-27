// Every frontend module — the one place a module is registered.
// Tooling inserts lines between the rudis:* markers. Keep the markers; add modules between
// them, never around them.
import type { FrontendModule } from "@/shared/types/frontend-module";
// rudis:modules:imports:begin
import { authModule } from "@/modules/auth/module";
import { dashboardModule } from "@/modules/dashboard/module";
// rudis:modules:imports:end

export const frontendModules: FrontendModule[] = [
  // rudis:modules:begin
  authModule,
  dashboardModule,
  // rudis:modules:end
];
