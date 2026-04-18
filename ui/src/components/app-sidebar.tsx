import * as React from "react";
import { Link, useLocation } from "react-router-dom";
import {
  Home as HomeIcon,
  FileText,
  BookOpen,
  Network,
  Terminal,
  Sparkles,
} from "lucide-react";

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { useProjectStore } from "@/stores/project";
import { useProjects } from "@/hooks/api/useProjects";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const NAV = [
  { to: "/", label: "Home", icon: HomeIcon },
  { to: "/notes", label: "Notes", icon: FileText },
  { to: "/docs", label: "Documents", icon: BookOpen },
  { to: "/graph", label: "Graph", icon: Network },
  { to: "/mcp", label: "MCP", icon: Terminal },
];

export function AppSidebar(props: React.ComponentProps<typeof Sidebar>) {
  const { pathname } = useLocation();
  const slug = useProjectStore((s) => s.slug);
  const setSlug = useProjectStore((s) => s.setSlug);
  const { data: projects } = useProjects();

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <div className="sidebar-brand">
          <Link to="/" className="sidebar-brand-link">
            <Sparkles className="size-5 shrink-0 text-primary" />
            <span className="sidebar-brand-name">docsiq</span>
          </Link>
          <SidebarTrigger className="sidebar-brand-trigger" />
        </div>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Workspace</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {NAV.map((item) => {
                const active = item.to === "/" ? pathname === "/" : pathname.startsWith(item.to);
                return (
                  <SidebarMenuItem key={item.to}>
                    <SidebarMenuButton asChild isActive={active} tooltip={item.label}>
                      <Link to={item.to}>
                        <item.icon />
                        <span>{item.label}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <div className="px-2 pb-2 group-data-[collapsible=icon]:hidden">
          <label className="block text-[10px] uppercase tracking-wider text-muted-foreground mb-1">
            Project
          </label>
          <Select value={slug} onValueChange={setSlug}>
            <SelectTrigger className="w-full h-8 text-xs font-mono">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(projects?.length ? projects : [{ slug, name: slug }]).map((p) => (
                <SelectItem key={p.slug} value={p.slug} className="font-mono text-xs">
                  {p.name || p.slug}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </SidebarFooter>
    </Sidebar>
  );
}
