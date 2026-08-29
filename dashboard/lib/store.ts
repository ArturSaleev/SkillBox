import { create } from "zustand";

interface DashboardState {
  selectedWorkspace: string | null;
  sidebarOpen: boolean;
  searchQuery: string;
  filters: { domains: string[]; intents: string[]; statuses: string[] };
  setSidebarOpen: (open: boolean) => void;
  setSearchQuery: (query: string) => void;
  setFilters: (filters: DashboardState["filters"]) => void;
}

export const useDashboardStore = create<DashboardState>((set) => ({
  selectedWorkspace: null,
  sidebarOpen: true,
  searchQuery: "",
  filters: { domains: [], intents: [], statuses: [] },
  setSidebarOpen: (sidebarOpen) => set({ sidebarOpen }),
  setSearchQuery: (searchQuery) => set({ searchQuery }),
  setFilters: (filters) => set({ filters })
}));
