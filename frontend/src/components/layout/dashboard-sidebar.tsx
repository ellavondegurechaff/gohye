"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { 
  LayoutDashboard, 
  CreditCard, 
  FolderOpen, 
  Upload,
  RefreshCw,
  Settings
} from "lucide-react";

const navigation = [
  {
    name: "Dashboard",
    href: "/dashboard",
    icon: LayoutDashboard,
  },
  {
    name: "Collections",
    href: "/dashboard/collections",
    icon: FolderOpen,
  },
  {
    name: "Import",
    href: "/dashboard/import",
    icon: Upload,
  },
  {
    name: "Sync",
    href: "/dashboard/sync",
    icon: RefreshCw,
  },
];

export function DashboardSidebar() {
  const pathname = usePathname();

  return (
    <div className="hidden md:flex md:w-64 md:flex-col md:fixed md:inset-y-16 md:left-0 z-30">
      <div className="flex flex-col flex-grow bg-gradient-to-b from-black/95 via-zinc-900/90 to-black/95 backdrop-blur-xl border-r border-white/10 shadow-2xl shadow-purple-900/20 relative overflow-hidden">
        {/* Revolutionary animated background */}
        <div className="absolute inset-0 opacity-10">
          <div className="absolute top-0 left-0 w-32 h-32 bg-pink-500/20 rounded-full blur-2xl animate-pulse" />
          <div className="absolute bottom-20 right-0 w-24 h-24 bg-purple-500/20 rounded-full blur-2xl animate-pulse" style={{ animationDelay: '2s' }} />
          <div className="absolute top-1/2 left-1/2 w-20 h-20 bg-cyan-500/20 rounded-full blur-2xl animate-pulse" style={{ animationDelay: '4s' }} />
        </div>
        <div className="flex-1 px-4 py-6 relative z-10">
          {/* Revolutionary header */}
          <div className="mb-8">
            <div className="flex items-center justify-center mb-4">
              <div className="w-12 h-12 bg-gradient-to-br from-pink-500 via-purple-500 to-cyan-500 rounded-2xl flex items-center justify-center shadow-lg shadow-purple-500/25">
                <div className="w-6 h-6 bg-white/90 rounded-lg flex items-center justify-center">
                  <div className="w-2 h-2 bg-gradient-to-br from-pink-500 to-purple-500 rounded-full animate-pulse" />
                </div>
              </div>
            </div>
            <div className="text-center">
              <h2 className="text-sm font-bold bg-gradient-to-r from-white to-purple-200 bg-clip-text text-transparent">
                GOHYE PANEL
              </h2>
              <p className="text-xs text-zinc-500 mt-1">Revolutionary v2.0</p>
            </div>
          </div>

          <nav className="space-y-2">
            {navigation.map((item, index) => {
              const isActive = pathname === item.href || (item.href !== "/dashboard" && pathname.startsWith(item.href + "/"));
              return (
                <Link
                  key={item.name}
                  href={item.href}
                  className={cn(
                    "group flex items-center px-4 py-3.5 text-sm font-medium rounded-2xl transition-all duration-500 relative overflow-hidden backdrop-blur-sm",
                    isActive
                      ? "bg-gradient-to-r from-pink-500/30 via-purple-500/20 to-cyan-500/30 text-white border border-pink-500/40 shadow-2xl shadow-pink-500/30 transform scale-105"
                      : "text-zinc-300 hover:bg-gradient-to-r hover:from-zinc-800/60 hover:via-zinc-700/40 hover:to-zinc-800/60 hover:text-white hover:border hover:border-white/20 hover:shadow-xl hover:shadow-white/10 hover:scale-105"
                  )}
                  style={{ animationDelay: `${index * 100}ms` }}
                >
                  {/* Revolutionary glow effects */}
                  {isActive && (
                    <>
                      <div className="absolute inset-0 bg-gradient-to-r from-pink-500/20 via-purple-500/10 to-cyan-500/20 blur-lg animate-pulse" />
                      <div className="absolute -inset-1 bg-gradient-to-r from-pink-500/30 to-purple-500/30 rounded-2xl blur-sm opacity-50 animate-pulse" />
                    </>
                  )}
                  
                  {/* Hover magnetic effect */}
                  <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/5 to-transparent opacity-0 group-hover:opacity-100 transition-all duration-300 rounded-2xl" />
                  
                  <item.icon
                    className={cn(
                      "mr-4 h-5 w-5 relative z-10 transition-all duration-500",
                      isActive 
                        ? "text-pink-300 drop-shadow-[0_0_12px_rgba(236,72,153,0.8)] scale-110" 
                        : "text-zinc-400 group-hover:text-white group-hover:scale-125 group-hover:drop-shadow-[0_0_8px_rgba(255,255,255,0.5)]"
                    )}
                  />
                  <span className={cn(
                    "relative z-10 transition-all duration-500 font-semibold tracking-wide",
                    "group-hover:translate-x-2 group-hover:text-shadow-lg"
                  )}>
                    {item.name}
                  </span>
                  
                  {/* Revolutionary indicators */}
                  <div className="ml-auto flex items-center gap-2 relative z-10">
                    {/* Active pulse indicator */}
                    {isActive && (
                      <div className="w-2 h-2 bg-gradient-to-r from-pink-400 to-purple-400 rounded-full animate-pulse shadow-[0_0_8px_rgba(236,72,153,0.8)]" />
                    )}
                    
                    {/* Hover arrow */}
                    <div className={cn(
                      "w-0 h-0 border-l-[6px] border-l-transparent border-r-[6px] border-r-transparent border-b-[8px] transition-all duration-300 opacity-0 -translate-x-2",
                      isActive 
                        ? "border-b-pink-400 opacity-100 translate-x-0" 
                        : "border-b-white group-hover:opacity-100 group-hover:translate-x-0"
                    )} />
                  </div>

                  {/* Revolutionary border effects */}
                  <div className={cn(
                    "absolute inset-x-0 bottom-0 h-[1px] bg-gradient-to-r transition-all duration-500",
                    isActive
                      ? "from-pink-500/60 via-purple-500/60 to-cyan-500/60 opacity-100"
                      : "from-transparent via-white/20 to-transparent opacity-0 group-hover:opacity-100"
                  )} />
                </Link>
              );
            })}
          </nav>
        </div>
        
        {/* Revolutionary Settings Section */}
        <div className="px-4 pb-6 relative z-10">
          <div className="border-t border-gradient-to-r from-pink-500/20 via-purple-500/30 to-cyan-500/20 pt-6">
            <Link
              href="/dashboard/settings"
              className="group flex items-center px-4 py-3.5 text-sm font-medium rounded-2xl text-zinc-300 hover:bg-gradient-to-r hover:from-zinc-800/60 hover:via-zinc-700/40 hover:to-zinc-800/60 hover:text-white hover:border hover:border-white/20 hover:shadow-xl hover:shadow-white/10 hover:scale-105 transition-all duration-500 relative overflow-hidden backdrop-blur-sm"
            >
              {/* Hover magnetic effect */}
              <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/5 to-transparent opacity-0 group-hover:opacity-100 transition-all duration-300 rounded-2xl" />
              
              <Settings className="mr-4 h-5 w-5 text-zinc-400 group-hover:text-white group-hover:scale-125 group-hover:drop-shadow-[0_0_8px_rgba(255,255,255,0.5)] transition-all duration-500 relative z-10" />
              <span className="group-hover:translate-x-2 transition-all duration-500 font-semibold tracking-wide relative z-10">Settings</span>
              
              {/* Hover arrow */}
              <div className="ml-auto w-0 h-0 border-l-[6px] border-l-transparent border-r-[6px] border-r-transparent border-b-[8px] border-b-white opacity-0 group-hover:opacity-100 group-hover:translate-x-0 transition-all duration-300 -translate-x-2 relative z-10" />
              
              {/* Revolutionary border effects */}
              <div className="absolute inset-x-0 bottom-0 h-[1px] bg-gradient-to-r from-transparent via-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-all duration-500" />
            </Link>
          </div>
        </div>

        {/* Revolutionary Footer */}
        <div className="px-4 pb-6 relative z-10">
          <div className="text-center space-y-4">
            {/* Animated logo */}
            <div className="relative">
              <div className="w-16 h-16 mx-auto bg-gradient-to-br from-pink-500 via-purple-500 to-cyan-500 rounded-3xl flex items-center justify-center shadow-2xl shadow-purple-500/40 relative overflow-hidden">
                {/* Rotating ring */}
                <div className="absolute inset-2 border-2 border-white/30 rounded-2xl animate-spin" style={{ animationDuration: '8s' }} />
                <div className="absolute inset-4 border border-white/20 rounded-xl animate-spin" style={{ animationDuration: '4s', animationDirection: 'reverse' }} />
                
                {/* Center pulse */}
                <div className="w-6 h-6 bg-white/95 rounded-xl flex items-center justify-center shadow-lg relative z-10">
                  <div className="w-3 h-3 bg-gradient-to-br from-pink-500 to-purple-500 rounded-lg animate-pulse" />
                </div>
                
                {/* Orbital dots */}
                <div className="absolute inset-0 animate-spin" style={{ animationDuration: '12s' }}>
                  <div className="absolute top-1 left-1/2 w-1 h-1 bg-white/70 rounded-full transform -translate-x-1/2" />
                  <div className="absolute bottom-1 left-1/2 w-1 h-1 bg-white/70 rounded-full transform -translate-x-1/2" />
                </div>
              </div>
              
              {/* Glow effect */}
              <div className="absolute inset-0 w-16 h-16 mx-auto bg-gradient-to-br from-pink-500/30 to-purple-500/30 rounded-full blur-xl animate-pulse" />
            </div>
            
            {/* Text with gradient */}
            <div>
              <h3 className="text-lg font-black bg-gradient-to-r from-white via-pink-200 to-purple-200 bg-clip-text text-transparent">
                GOHYE
              </h3>
              <p className="text-xs text-zinc-400 font-medium tracking-wider uppercase">
                Admin Dashboard
              </p>
              <div className="flex items-center justify-center gap-2 mt-2">
                <div className="w-1 h-1 bg-green-400 rounded-full animate-pulse" />
                <p className="text-xs text-zinc-500">v2.0 Revolutionary</p>
                <div className="w-1 h-1 bg-green-400 rounded-full animate-pulse" />
              </div>
            </div>
            
            {/* Status indicator */}
            <div className="flex items-center justify-center gap-2 pt-2">
              <div className="w-3 h-3 bg-green-400 rounded-full animate-pulse shadow-[0_0_8px_rgba(34,197,94,0.6)]" />
              <span className="text-xs text-green-400 font-medium">System Online</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}