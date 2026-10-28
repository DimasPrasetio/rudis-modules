import { Outlet } from "react-router-dom";
import { SidebarInset, SidebarProvider } from "@/shared/components/primitives/sidebar";
import { TooltipProvider } from "@/shared/components/primitives/tooltip";
import { AppHeader } from "@/modules/app-shell/components/AppHeader";
import { AppSidebar } from "@/modules/app-shell/components/AppSidebar";

const MAIN_ID = "konten-utama";

/**
 * The first stop of the keyboard on every page (WCAG 2.2 · 2.4.1 Bypass Blocks): hidden until
 * focused, then it jumps past the sidebar and the header to the page itself.
 */
function SkipLink() {
  return (
    <a
      href={`#${MAIN_ID}`}
      onClick={(e) => {
        e.preventDefault();
        document.getElementById(MAIN_ID)?.focus();
      }}
      className="sr-only rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50"
    >
      Lewati ke konten utama
    </a>
  );
}

/**
 * The frame around every protected page: a navy sidebar and an inset canvas that holds the
 * header and the page. Navigation and the user menu come from other modules through
 * @/shared/composition — the shell imports none of them.
 */
export function AppShell() {
  return (
    <TooltipProvider delayDuration={0}>
      <SkipLink />
      <SidebarProvider>
        <AppSidebar />
        <SidebarInset>
          <AppHeader />
          {/* The skip link's target: focusable from script only, no ring of its own. */}
          <div id={MAIN_ID} tabIndex={-1} className="mx-auto w-full max-w-[1440px] flex-1 p-4 outline-none md:p-6">
            <Outlet />
          </div>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  );
}
