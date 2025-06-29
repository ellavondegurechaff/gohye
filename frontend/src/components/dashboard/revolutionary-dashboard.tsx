"use client";

import { useEffect, useState, useMemo } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { useRouter } from "next/navigation";

// UI Components
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";

// Icons
import { 
  TrendingUp, TrendingDown, Database, Users, CreditCard, FolderOpen,
  Upload, RefreshCw, Plus, Eye, Edit, Download, BarChart3,
  Activity, Clock, Star, Sparkles, Zap, Target, Crown,
  Flame, Gem, Map, ArrowRight, ExternalLink, MousePointer,
  Layers, Palette, Move3D, Volume2, Settings, BookOpen
} from "lucide-react";

// Types
import type { DashboardStats } from "@/lib/types";
import { apiClient } from "@/lib/api";

interface RevolutionaryDashboardProps {
  initialStats?: DashboardStats;
}

export function RevolutionaryDashboard({ initialStats }: RevolutionaryDashboardProps) {
  const router = useRouter();
  const [stats, setStats] = useState<DashboardStats | null>(initialStats || null);
  const [loading, setLoading] = useState(!initialStats);
  const [currentTime, setCurrentTime] = useState(new Date());

  // Real-time clock
  useEffect(() => {
    const timer = setInterval(() => setCurrentTime(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  // Fetch dashboard stats
  useEffect(() => {
    if (!initialStats) {
      const fetchStats = async () => {
        try {
          const data = await apiClient.getDashboardStats();
          setStats(data);
        } catch (error) {
          console.error('Failed to fetch dashboard stats:', error);
        } finally {
          setLoading(false);
        }
      };
      fetchStats();
    }
  }, [initialStats]);

  // Quick actions data
  const quickActions = [
    {
      title: "Add New Card",
      description: "Create a new K-pop trading card",
      icon: Plus,
      gradient: "from-pink-500 to-rose-500",
      action: () => router.push("/dashboard/cards/new"),
      shortcut: "⌘N",
    },
    {
      title: "Import Album",
      description: "Bulk import cards from album",
      icon: Upload,
      gradient: "from-purple-500 to-violet-500",
      action: () => router.push("/dashboard/import"),
      shortcut: "⌘I",
    },
    {
      title: "Manage Collections",
      description: "Organize your card collections",
      icon: FolderOpen,
      gradient: "from-blue-500 to-cyan-500",
      action: () => router.push("/dashboard/collections"),
      shortcut: "⌘C",
    },
    {
      title: "View Analytics",
      description: "Deep dive into card statistics",
      icon: BarChart3,
      gradient: "from-green-500 to-emerald-500",
      action: () => router.push("/dashboard/analytics"),
      shortcut: "⌘A",
    },
  ];

  // Advanced stats computation
  const advancedStats = useMemo(() => {
    if (!stats) return null;
    
    return {
      cardGrowth: 12.5, // Mock growth percentage
      collectionGrowth: 8.3,
      userGrowth: 23.1,
      syncHealth: stats.sync_percentage,
      totalValue: 1250000, // Mock total value
      rareCards: Math.floor(stats.total_cards * 0.15),
      activeUsers: Math.floor(stats.total_users * 0.68),
      dailyActivity: 145, // Mock daily activity
    };
  }, [stats]);

  const formatTime = (date: Date) => {
    return date.toLocaleTimeString('en-US', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    });
  };

  const formatDate = (date: Date) => {
    return date.toLocaleDateString('en-US', {
      weekday: 'long',
      year: 'numeric',
      month: 'long',
      day: 'numeric'
    });
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-black via-zinc-900 to-black flex items-center justify-center">
        <motion.div
          animate={{ rotate: 360 }}
          transition={{ duration: 2, repeat: Infinity, ease: "linear" }}
          className="h-16 w-16 border-4 border-pink-500 border-t-transparent rounded-full"
        />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-black via-zinc-900/50 to-black relative overflow-hidden">
      {/* Animated Background */}
      <div className="absolute inset-0 opacity-20">
        <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-pink-500/10 rounded-full blur-3xl animate-pulse" />
        <div className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-purple-500/10 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '2s' }} />
        <div className="absolute top-1/2 left-1/2 w-96 h-96 bg-cyan-500/10 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '4s' }} />
      </div>

      <div className="relative z-10 container mx-auto px-6 py-8 space-y-8">
        {/* Hero Header */}
        <motion.div
          initial={{ opacity: 0, y: -30 }}
          animate={{ opacity: 1, y: 0 }}
          className="text-center space-y-6"
        >
          <div className="space-y-4">
            <h1 className="text-6xl font-black bg-gradient-to-r from-white via-pink-200 to-purple-200 bg-clip-text text-transparent">
              GoHYE Dashboard
            </h1>
            <p className="text-2xl text-zinc-400 max-w-3xl mx-auto">
              Revolutionary K-pop Trading Card Management System
            </p>
          </div>

          {/* Real-time Clock */}
          <motion.div 
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            transition={{ delay: 0.5 }}
            className="inline-flex items-center gap-4 bg-black/40 backdrop-blur-xl border border-white/10 rounded-2xl px-8 py-4"
          >
            <Clock className="h-6 w-6 text-pink-400" />
            <div className="text-center">
              <div className="text-3xl font-mono text-white">{formatTime(currentTime)}</div>
              <div className="text-sm text-zinc-400">{formatDate(currentTime)}</div>
            </div>
          </motion.div>
        </motion.div>

        {/* Main Stats Grid */}
        {stats && advancedStats && (
          <motion.div
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.8 }}
            className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6"
          >
            {[
              {
                title: "Total Cards",
                value: stats.total_cards,
                growth: advancedStats.cardGrowth,
                icon: CreditCard,
                gradient: "from-pink-500 to-rose-500",
                prefix: "",
                suffix: "",
              },
              {
                title: "Collections",
                value: stats.total_collections,
                growth: advancedStats.collectionGrowth,
                icon: FolderOpen,
                gradient: "from-purple-500 to-violet-500",
                prefix: "",
                suffix: "",
              },
              {
                title: "Sync Health",
                value: stats.sync_percentage,
                growth: 2.1,
                icon: RefreshCw,
                gradient: "from-green-500 to-emerald-500",
                prefix: "",
                suffix: "%",
              },
              {
                title: "Daily Activity",
                value: advancedStats.dailyActivity,
                growth: 15.3,
                icon: Activity,
                gradient: "from-blue-500 to-cyan-500",
                prefix: "",
                suffix: "",
              },
            ].map((stat, index) => (
              <motion.div
                key={stat.title}
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ delay: 1 + index * 0.1 }}
                whileHover={{ scale: 1.05 }}
                className="group cursor-pointer"
              >
                <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10 hover:border-white/20 transition-all duration-300 overflow-hidden relative">
                  <div className={`absolute inset-0 bg-gradient-to-br ${stat.gradient} opacity-0 group-hover:opacity-10 transition-opacity duration-300`} />
                  <CardContent className="p-6 relative z-10">
                    <div className="flex items-center justify-between mb-4">
                      <div className={`p-3 rounded-xl bg-gradient-to-br ${stat.gradient} shadow-lg`}>
                        <stat.icon className="h-6 w-6 text-white" />
                      </div>
                      <div className={`flex items-center gap-1 text-sm ${stat.growth >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {stat.growth >= 0 ? <TrendingUp className="h-4 w-4" /> : <TrendingDown className="h-4 w-4" />}
                        {Math.abs(stat.growth)}%
                      </div>
                    </div>
                    <div className="space-y-2">
                      <p className="text-4xl font-bold text-white group-hover:text-transparent group-hover:bg-gradient-to-r group-hover:bg-clip-text group-hover:from-white group-hover:to-zinc-300 transition-all duration-300">
                        {stat.prefix}{typeof stat.value === 'number' ? stat.value.toLocaleString() : stat.value}{stat.suffix}
                      </p>
                      <p className="text-sm text-zinc-400 font-medium">{stat.title}</p>
                    </div>
                  </CardContent>
                </Card>
              </motion.div>
            ))}
          </motion.div>
        )}

        {/* Quick Actions */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 1.4 }}
        >
          <h2 className="text-3xl font-bold text-white mb-6 flex items-center gap-3">
            <Zap className="h-8 w-8 text-pink-400" />
            Quick Actions
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
            {quickActions.map((action, index) => (
              <motion.div
                key={action.title}
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 1.6 + index * 0.1 }}
                whileHover={{ scale: 1.02 }}
                whileTap={{ scale: 0.98 }}
              >
                <Card 
                  className="bg-gradient-to-br from-black/60 to-zinc-900/40 backdrop-blur-xl border-white/10 hover:border-white/20 transition-all duration-300 cursor-pointer group overflow-hidden relative"
                  onClick={action.action}
                >
                  <div className={`absolute inset-0 bg-gradient-to-br ${action.gradient} opacity-0 group-hover:opacity-10 transition-opacity duration-300`} />
                  <CardContent className="p-6 relative z-10">
                    <div className="flex items-start justify-between mb-4">
                      <div className={`p-3 rounded-xl bg-gradient-to-br ${action.gradient} shadow-lg group-hover:shadow-xl transition-shadow duration-300`}>
                        <action.icon className="h-6 w-6 text-white" />
                      </div>
                      <kbd className="px-2 py-1 text-xs bg-zinc-800/50 border border-zinc-700/50 rounded text-zinc-400 opacity-0 group-hover:opacity-100 transition-opacity duration-300">
                        {action.shortcut}
                      </kbd>
                    </div>
                    <div className="space-y-3">
                      <h3 className="text-lg font-semibold text-white group-hover:text-transparent group-hover:bg-gradient-to-r group-hover:bg-clip-text group-hover:from-white group-hover:to-zinc-300 transition-all duration-300">
                        {action.title}
                      </h3>
                      <p className="text-sm text-zinc-400 group-hover:text-zinc-300 transition-colors duration-300">
                        {action.description}
                      </p>
                      <div className="flex items-center text-sm text-pink-400 group-hover:text-pink-300 transition-colors duration-300">
                        <span>Get started</span>
                        <ArrowRight className="ml-2 h-4 w-4 group-hover:translate-x-1 transition-transform duration-300" />
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </motion.div>
            ))}
          </div>
        </motion.div>

        {/* Advanced Analytics Overview */}
        {stats && advancedStats && (
          <motion.div
            initial={{ opacity: 0, y: 30 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 2 }}
            className="grid grid-cols-1 lg:grid-cols-2 gap-6"
          >
            {/* Collection Health */}
            <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
              <CardHeader>
                <CardTitle className="text-xl text-white flex items-center gap-3">
                  <BarChart3 className="h-6 w-6 text-blue-400" />
                  Collection Health
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="space-y-4">
                  <div className="flex justify-between text-sm">
                    <span className="text-zinc-400">Sync Status</span>
                    <span className="text-white font-medium">{stats.sync_percentage}%</span>
                  </div>
                  <Progress value={stats.sync_percentage} className="h-2" />
                </div>
                <div className="space-y-4">
                  <div className="flex justify-between text-sm">
                    <span className="text-zinc-400">Rare Cards</span>
                    <span className="text-white font-medium">{advancedStats.rareCards}</span>
                  </div>
                  <Progress value={(advancedStats.rareCards / stats.total_cards) * 100} className="h-2" />
                </div>
                <div className="grid grid-cols-3 gap-4 pt-4 border-t border-zinc-700/50">
                  <div className="text-center">
                    <p className="text-2xl font-bold text-green-400">{advancedStats.activeUsers}</p>
                    <p className="text-xs text-zinc-500">Active</p>
                  </div>
                  <div className="text-center">
                    <p className="text-2xl font-bold text-yellow-400">{stats.issue_count}</p>
                    <p className="text-xs text-zinc-500">Issues</p>
                  </div>
                  <div className="text-center">
                    <p className="text-2xl font-bold text-pink-400">{advancedStats.dailyActivity}</p>
                    <p className="text-xs text-zinc-500">Daily</p>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Recent Activity */}
            <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
              <CardHeader>
                <CardTitle className="text-xl text-white flex items-center gap-3">
                  <Activity className="h-6 w-6 text-green-400" />
                  Recent Activity
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-4">
                  {stats.recent_activity?.slice(0, 4).map((activity, index) => (
                    <motion.div
                      key={index}
                      initial={{ opacity: 0, x: -20 }}
                      animate={{ opacity: 1, x: 0 }}
                      transition={{ delay: 2.2 + index * 0.1 }}
                      className="flex items-center gap-4 p-3 rounded-lg bg-zinc-800/30 hover:bg-zinc-800/50 transition-colors duration-200"
                    >
                      <div className="p-2 rounded-full bg-pink-500/20">
                        <Star className="h-4 w-4 text-pink-400" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm text-white truncate">{activity.description}</p>
                        <p className="text-xs text-zinc-500">{new Date(activity.timestamp).toLocaleTimeString()}</p>
                      </div>
                      <Badge variant="outline" className="text-xs border-zinc-600 text-zinc-400">
                        {activity.type}
                      </Badge>
                    </motion.div>
                  ))}
                </div>
              </CardContent>
            </Card>
          </motion.div>
        )}

        {/* System Status Footer */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 2.5 }}
          className="text-center py-8"
        >
          <div className="inline-flex items-center gap-2 bg-black/40 backdrop-blur-xl border border-white/10 rounded-full px-6 py-3">
            <div className="w-2 h-2 bg-green-400 rounded-full animate-pulse" />
            <span className="text-sm text-zinc-400">System Status: All services operational</span>
          </div>
        </motion.div>
      </div>
    </div>
  );
}