"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { toast } from "sonner";

// UI Components
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

// Icons
import { 
  RefreshCw, Database, HardDrive, AlertTriangle, CheckCircle,
  XCircle, Clock, TrendingUp, BarChart3, Activity, Zap,
  Shield, Settings, FileSearch, Trash2, Download, Upload,
  Target, Layers, Gauge, Server, CloudOff, Wifi, WifiOff,
  Play, Pause, Square, RotateCcw, Volume2, VolumeX
} from "lucide-react";

// Types
import type { SyncStatus, SyncOperation } from "@/lib/types";

interface RevolutionarySyncProps {
  initialSyncStatus: SyncStatus;
}

export function RevolutionarySync({ initialSyncStatus }: RevolutionarySyncProps) {
  const [syncStatus, setSyncStatus] = useState<SyncStatus>(initialSyncStatus);
  const [operations, setOperations] = useState<SyncOperation[]>([]);
  const [loading, setLoading] = useState(false);
  const [soundEnabled, setSoundEnabled] = useState(true);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [selectedOperation, setSelectedOperation] = useState<string | null>(null);
  const [isClient, setIsClient] = useState(false);

  useEffect(() => {
    setIsClient(true);
  }, []);

  // Helper functions
  const getStaticTimestamp = (offsetMinutes: number): string => {
    const baseTime = new Date('2024-01-01T12:00:00Z').getTime();
    return new Date(baseTime + offsetMinutes * 60000).toISOString();
  };

  const formatDuration = (startTime: string, endTime?: string): string => {
    const start = new Date(startTime).getTime();
    const end = endTime ? new Date(endTime).getTime() : Date.now();
    const diffMs = Math.abs(end - start);
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    
    if (diffHours > 0) {
      return `${diffHours}h ${diffMins % 60}m`;
    }
    return `${diffMins}m`;
  };

  // Mock operations for demo (using static timestamps for SSR safety)
  const mockOperations: SyncOperation[] = [
    {
      id: "op-1",
      type: "full_sync",
      status: "completed",
      progress: 100,
      started_at: getStaticTimestamp(-300), // 5 hours ago (static)
      completed_at: getStaticTimestamp(-60), // 1 hour ago (static)
      errors: [],
      results: { processed: 1250, fixed: 23, failed: 2 }
    },
    {
      id: "op-2", 
      type: "clean_orphans",
      status: "running",
      progress: 65,
      started_at: getStaticTimestamp(-120), // 2 hours ago (static)
      errors: [],
    },
    {
      id: "op-3",
      type: "validate_all",
      status: "pending",
      progress: 0,
      started_at: getStaticTimestamp(0), // Base time (static)
      errors: [],
    }
  ];

  useEffect(() => {
    setOperations(mockOperations);
  }, []);

  // Auto-refresh status (only on client-side)
  useEffect(() => {
    if (!autoRefresh || !isClient) return;

    const interval = setInterval(async () => {
      try {
        // Mock updating sync status (only on client-side)
        setSyncStatus(prev => ({
          ...prev,
          consistency_percentage: Math.min(100, prev.consistency_percentage + Math.random() * 2),
          last_sync: new Date().toISOString(), // Safe to use on client-side
        }));
      } catch (error) {
        console.error('Failed to refresh sync status:', error);
      }
    }, 5000);

    return () => clearInterval(interval);
  }, [autoRefresh, isClient]);

  // Health status calculation
  const healthStatus = useMemo(() => {
    const { database_healthy, storage_healthy, consistency_percentage } = syncStatus;
    
    if (database_healthy && storage_healthy && consistency_percentage >= 95) {
      return { level: 'excellent', color: 'green', label: 'Excellent' };
    } else if (database_healthy && storage_healthy && consistency_percentage >= 80) {
      return { level: 'good', color: 'blue', label: 'Good' };
    } else if (database_healthy || storage_healthy) {
      return { level: 'warning', color: 'yellow', label: 'Warning' };
    } else {
      return { level: 'critical', color: 'red', label: 'Critical' };
    }
  }, [syncStatus]);

  const handleSyncOperation = useCallback(async (type: string) => {
    setLoading(true);
    
    try {
      // Mock operation start
      const newOperation: SyncOperation = {
        id: `op-${Date.now()}`,
        type: type as any,
        status: 'running',
        progress: 0,
        started_at: new Date().toISOString(),
        errors: [],
      };

      setOperations(prev => [newOperation, ...prev]);
      toast.success(`Started ${type.replace('_', ' ')} operation`);

      if (soundEnabled) {
        // Play start sound
        const audio = new Audio('data:audio/wav;base64,UklGRnoGAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQoGAACBhYqFbF1fdJivrJBhNjVgodDbq2EcBj+a2/LDciUFLIHO8tiJNwgZaLvt559NEAxQp+PwtmMcBjiR1/LMeSwFJHfH8N2QQAoUXrTp66hVFApGn+DyvmEXBze/zPLHdSMELIHO8tuFNwgZZ7zs5ZdMEAxQp+PwtmUcBzaRz+3PgykEJ2+66rBuFgU7hM7y2YU3CBlkvuzjl0wQDFCn4/C2ZRwGNZHP7K+DIggudM7u3IU7CRZq6uW7fBYDMn/P8N2NQAoTXrTp66hVFApPiOLxy2Y/CzODwcVrPAq/hM7y2YU4CRNlu+zhnUsQDFCn4/C2ZRwHNZPO7K+DIgcme82+dRUFNnfM8N+QQAoUWqzn6a5hFgo0Vn/C7r1bGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJtteKdVxYKRoPH6bRmHAc5hM7y2YU3CRNluuzjmE4QDFCn4/C2ZRwHNZLO7K+DIgcme82+dRUFN3zK8N+QQQsUWqzn6a5hFgo0Vn/C7r1bGgwzf8nw34Y/CiaAyPDajTsIG2e86qxCFwRGiuvyv2gaCj2AzPLHdCcEKne66K2DIgctjMrt5ZNIDR5ryPHRhzIJGF615OOiSQwQRa3l8bdhFgo2k8/sz4EqBypWyuqjTgwRSarl8bdhFgo2k8/sz4EpBipwyuqgTAsMSaq68LRjFgo2k8/sz4AkBipwyuqgTAsMSaq68LRjFgo2k8/sz4ApBipwyuqgTAwQD15q9+G2ZRwHNZLP7M+DIgctdM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Yk+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKTI/D67FhGgwzf8nw34Y/CiaAyu7ejDoIGmq76qx4EgM9dM/w3Ik+CSJttOKdVxYKe=');
        audio.volume = 0.15;
        audio.play().catch(() => {});
      }

      // Mock progress updates
      const progressInterval = setInterval(() => {
        setOperations(prev => prev.map(op => 
          op.id === newOperation.id && op.status === 'running'
            ? { ...op, progress: Math.min(100, op.progress + Math.random() * 15) }
            : op
        ));
      }, 1000);

      // Complete operation after random time
      setTimeout(() => {
        clearInterval(progressInterval);
        setOperations(prev => prev.map(op => 
          op.id === newOperation.id
            ? { 
                ...op, 
                status: 'completed', 
                progress: 100, 
                completed_at: new Date().toISOString(),
                results: { processed: Math.floor(Math.random() * 1000) + 500, fixed: Math.floor(Math.random() * 50), failed: Math.floor(Math.random() * 5) }
              }
            : op
        ));
        toast.success(`${type.replace('_', ' ')} completed successfully`);
      }, Math.random() * 10000 + 5000);

    } catch (error: any) {
      console.error('Sync operation failed:', error);
      toast.error('Sync operation failed: ' + error.message);
    } finally {
      setLoading(false);
    }
  }, [soundEnabled]);

  // Use the hydration-safe formatDuration from utils
  const formatOperationDuration = (start: string, end?: string) => {
    return formatDuration(start, end);
  };

  const getOperationIcon = (type: string) => {
    switch (type) {
      case 'full_sync': return RefreshCw;
      case 'clean_orphans': return Trash2;
      case 'find_missing': return FileSearch;
      case 'validate_all': return Shield;
      default: return Activity;
    }
  };

  const getOperationColor = (status: string) => {
    switch (status) {
      case 'completed': return 'text-green-400';
      case 'running': return 'text-blue-400';
      case 'failed': return 'text-red-400';
      case 'pending': return 'text-yellow-400';
      default: return 'text-zinc-400';
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-black via-zinc-900/50 to-black relative overflow-hidden">
      {/* Animated Background */}
      <div className="absolute inset-0 opacity-20">
        <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-green-500/10 rounded-full blur-3xl animate-pulse" />
        <div className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-blue-500/10 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '2s' }} />
        <div className="absolute top-1/2 left-1/2 w-96 h-96 bg-purple-500/10 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '4s' }} />
      </div>

      <div className="relative z-10 container mx-auto px-6 py-8 space-y-8">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: -30 }}
          animate={{ opacity: 1, y: 0 }}
          className="text-center space-y-6"
        >
          <div className="space-y-4">
            <h1 className="text-5xl font-black bg-gradient-to-r from-white via-green-200 to-blue-200 bg-clip-text text-transparent">
              System Sync Center
            </h1>
            <p className="text-xl text-zinc-400 max-w-2xl mx-auto">
              Monitor and manage database synchronization and system health
            </p>
          </div>

          {/* Controls */}
          <div className="flex justify-center gap-4">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSoundEnabled(!soundEnabled)}
              className="text-zinc-400 hover:text-white"
            >
              {soundEnabled ? <Volume2 className="h-4 w-4 mr-2" /> : <VolumeX className="h-4 w-4 mr-2" />}
              {soundEnabled ? 'Sound On' : 'Sound Off'}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setAutoRefresh(!autoRefresh)}
              className="text-zinc-400 hover:text-white"
            >
              {autoRefresh ? <Pause className="h-4 w-4 mr-2" /> : <Play className="h-4 w-4 mr-2" />}
              {autoRefresh ? 'Auto-Refresh On' : 'Auto-Refresh Off'}
            </Button>
          </div>
        </motion.div>

        {/* System Health Overview */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
        >
          <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10 overflow-hidden relative">
            <div className={`absolute inset-0 bg-gradient-to-r ${
              healthStatus.color === 'green' ? 'from-green-500/10 to-emerald-500/5' :
              healthStatus.color === 'blue' ? 'from-blue-500/10 to-cyan-500/5' :
              healthStatus.color === 'yellow' ? 'from-yellow-500/10 to-orange-500/5' :
              'from-red-500/10 to-rose-500/5'
            }`} />
            <CardHeader className="relative z-10">
              <CardTitle className="text-2xl text-white flex items-center gap-3">
                <motion.div
                  animate={{ rotate: syncStatus.sync_in_progress ? 360 : 0 }}
                  transition={{ duration: 2, repeat: syncStatus.sync_in_progress ? Infinity : 0, ease: "linear" }}
                >
                  <Gauge className={`h-8 w-8 ${
                    healthStatus.color === 'green' ? 'text-green-400' :
                    healthStatus.color === 'blue' ? 'text-blue-400' :
                    healthStatus.color === 'yellow' ? 'text-yellow-400' :
                    'text-red-400'
                  }`} />
                </motion.div>
                System Health Status
                <Badge className={`${
                  healthStatus.color === 'green' ? 'bg-green-500/20 text-green-300 border-green-500/30' :
                  healthStatus.color === 'blue' ? 'bg-blue-500/20 text-blue-300 border-blue-500/30' :
                  healthStatus.color === 'yellow' ? 'bg-yellow-500/20 text-yellow-300 border-yellow-500/30' :
                  'bg-red-500/20 text-red-300 border-red-500/30'
                }`}>
                  {healthStatus.label}
                </Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className="relative z-10 space-y-6">
              {/* Main Health Metrics */}
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
                {[
                  {
                    title: "Database",
                    status: syncStatus.database_healthy,
                    icon: Database,
                    description: "Database connectivity and integrity"
                  },
                  {
                    title: "Storage",
                    status: syncStatus.storage_healthy,
                    icon: HardDrive,
                    description: "File storage accessibility"
                  },
                  {
                    title: "Consistency",
                    status: syncStatus.consistency_percentage >= 95,
                    icon: Shield,
                    description: `${syncStatus.consistency_percentage.toFixed(1)}% consistency`,
                    value: syncStatus.consistency_percentage
                  },
                  {
                    title: "Network",
                    status: true, // Mock always healthy
                    icon: Wifi,
                    description: "Network connectivity status"
                  },
                ].map((metric, index) => (
                  <motion.div
                    key={metric.title}
                    initial={{ opacity: 0, scale: 0.8 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ delay: 0.5 + index * 0.1 }}
                    className="p-4 bg-zinc-800/30 rounded-lg text-center"
                  >
                    <div className="flex items-center justify-center mb-3">
                      <metric.icon className={`h-8 w-8 ${metric.status ? 'text-green-400' : 'text-red-400'}`} />
                      {metric.status ? (
                        <CheckCircle className="h-4 w-4 text-green-400 ml-2" />
                      ) : (
                        <XCircle className="h-4 w-4 text-red-400 ml-2" />
                      )}
                    </div>
                    <h3 className="font-semibold text-white mb-1">{metric.title}</h3>
                    <p className="text-sm text-zinc-400">{metric.description}</p>
                    {metric.value !== undefined && (
                      <div className="mt-3">
                        <Progress value={metric.value} className="h-2" />
                      </div>
                    )}
                  </motion.div>
                ))}
              </div>

              {/* Detailed Status */}
              <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                <div className="lg:col-span-2">
                  <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
                    <BarChart3 className="h-5 w-5 text-blue-400" />
                    System Metrics
                  </h3>
                  <div className="grid grid-cols-2 gap-4">
                    {[
                      { label: "Orphaned Files", value: syncStatus.orphaned_files, icon: AlertTriangle, color: syncStatus.orphaned_files > 0 ? 'text-yellow-400' : 'text-green-400' },
                      { label: "Missing Files", value: syncStatus.missing_files, icon: FileSearch, color: syncStatus.missing_files > 0 ? 'text-red-400' : 'text-green-400' },
                      { label: "Consistency %", value: `${syncStatus.consistency_percentage.toFixed(1)}%`, icon: Target, color: 'text-blue-400' },
                      { label: "Last Sync", value: isClient ? new Date(syncStatus.last_sync).toLocaleTimeString() : "Syncing...", icon: Clock, color: 'text-zinc-400' },
                    ].map((metric) => (
                      <div key={metric.label} className="flex items-center gap-3 p-3 bg-zinc-800/30 rounded-lg">
                        <metric.icon className={`h-5 w-5 ${metric.color}`} />
                        <div>
                          <p className="text-sm text-zinc-400">{metric.label}</p>
                          <p className="font-semibold text-white">{metric.value}</p>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
                
                <div>
                  <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
                    <Zap className="h-5 w-5 text-purple-400" />
                    Quick Actions
                  </h3>
                  <div className="space-y-3">
                    {[
                      { label: "Full Sync", type: "full_sync", icon: RefreshCw, description: "Complete database synchronization" },
                      { label: "Clean Orphans", type: "clean_orphans", icon: Trash2, description: "Remove orphaned files" },
                      { label: "Find Missing", type: "find_missing", icon: FileSearch, description: "Locate missing files" },
                      { label: "Validate All", type: "validate_all", icon: Shield, description: "Validate system integrity" },
                    ].map((action) => (
                      <Button
                        key={action.type}
                        onClick={() => handleSyncOperation(action.type)}
                        disabled={loading || syncStatus.sync_in_progress}
                        className="w-full justify-start h-auto p-3 bg-zinc-800/30 hover:bg-zinc-700/50 border-zinc-700/50 text-left"
                        variant="outline"
                      >
                        <action.icon className="h-4 w-4 mr-3 text-purple-400" />
                        <div>
                          <p className="font-medium text-white">{action.label}</p>
                          <p className="text-xs text-zinc-400">{action.description}</p>
                        </div>
                      </Button>
                    ))}
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </motion.div>

        {/* Operations Management */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.6 }}
        >
          <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10">
            <CardHeader>
              <CardTitle className="text-2xl text-white flex items-center gap-3">
                <Activity className="h-8 w-8 text-cyan-400" />
                Sync Operations
                <Badge variant="secondary" className="bg-cyan-500/20 text-cyan-300 border-cyan-500/30">
                  {operations.filter(op => op.status === 'running').length} active
                </Badge>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <Tabs defaultValue="active" className="space-y-6">
                <TabsList className="grid w-full grid-cols-3 bg-black/60 backdrop-blur-xl">
                  <TabsTrigger value="active" className="data-[state=active]:bg-zinc-700/50">
                    Active ({operations.filter(op => op.status === 'running').length})
                  </TabsTrigger>
                  <TabsTrigger value="completed" className="data-[state=active]:bg-zinc-700/50">
                    Completed ({operations.filter(op => op.status === 'completed').length})
                  </TabsTrigger>
                  <TabsTrigger value="all" className="data-[state=active]:bg-zinc-700/50">
                    All ({operations.length})
                  </TabsTrigger>
                </TabsList>

                <TabsContent value="active" className="space-y-4">
                  {operations.filter(op => op.status === 'running' || op.status === 'pending').map((operation) => {
                    const Icon = getOperationIcon(operation.type);
                    return (
                      <motion.div
                        key={operation.id}
                        layout
                        initial={{ opacity: 0, x: -20 }}
                        animate={{ opacity: 1, x: 0 }}
                        className="p-4 bg-zinc-800/30 rounded-lg border border-zinc-700/50 hover:border-zinc-600/50 transition-colors"
                      >
                        <div className="flex items-center justify-between mb-3">
                          <div className="flex items-center gap-3">
                            <motion.div
                              animate={operation.status === 'running' ? { rotate: 360 } : {}}
                              transition={{ duration: 2, repeat: operation.status === 'running' ? Infinity : 0, ease: "linear" }}
                            >
                              <Icon className={`h-5 w-5 ${getOperationColor(operation.status)}`} />
                            </motion.div>
                            <div>
                              <h3 className="font-medium text-white capitalize">
                                {operation.type.replace('_', ' ')}
                              </h3>
                              <p className="text-sm text-zinc-400">
                                Started {formatOperationDuration(operation.started_at)} ago
                              </p>
                            </div>
                          </div>
                          <Badge className={`${
                            operation.status === 'running' ? 'bg-blue-500/20 text-blue-300 border-blue-500/30' :
                            'bg-yellow-500/20 text-yellow-300 border-yellow-500/30'
                          }`}>
                            {operation.status}
                          </Badge>
                        </div>
                        <div className="space-y-2">
                          <div className="flex justify-between text-sm">
                            <span className="text-zinc-400">Progress</span>
                            <span className="text-white">{operation.progress}%</span>
                          </div>
                          <Progress value={operation.progress} className="h-2" />
                        </div>
                      </motion.div>
                    );
                  })}
                  {operations.filter(op => op.status === 'running' || op.status === 'pending').length === 0 && (
                    <div className="text-center py-8">
                      <Server className="h-12 w-12 text-zinc-500 mx-auto mb-4" />
                      <p className="text-zinc-400">No active operations</p>
                    </div>
                  )}
                </TabsContent>

                <TabsContent value="completed" className="space-y-4">
                  {operations.filter(op => op.status === 'completed' || op.status === 'failed').map((operation) => {
                    const Icon = getOperationIcon(operation.type);
                    return (
                      <motion.div
                        key={operation.id}
                        layout
                        initial={{ opacity: 0, x: -20 }}
                        animate={{ opacity: 1, x: 0 }}
                        className="p-4 bg-zinc-800/30 rounded-lg border border-zinc-700/50"
                      >
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-3">
                            <Icon className={`h-5 w-5 ${getOperationColor(operation.status)}`} />
                            <div>
                              <h3 className="font-medium text-white capitalize">
                                {operation.type.replace('_', ' ')}
                              </h3>
                              <p className="text-sm text-zinc-400">
                                {operation.completed_at ? 
                                  `Completed in ${formatOperationDuration(operation.started_at, operation.completed_at)}` :
                                  `Started ${formatOperationDuration(operation.started_at)} ago`
                                }
                              </p>
                              {operation.results && (
                                <div className="flex gap-4 mt-2 text-xs">
                                  <span className="text-green-400">✓ {operation.results.processed} processed</span>
                                  <span className="text-blue-400">⚡ {operation.results.fixed} fixed</span>
                                  {operation.results.failed > 0 && (
                                    <span className="text-red-400">✗ {operation.results.failed} failed</span>
                                  )}
                                </div>
                              )}
                            </div>
                          </div>
                          <Badge className={`${
                            operation.status === 'completed' ? 'bg-green-500/20 text-green-300 border-green-500/30' :
                            'bg-red-500/20 text-red-300 border-red-500/30'
                          }`}>
                            {operation.status}
                          </Badge>
                        </div>
                      </motion.div>
                    );
                  })}
                </TabsContent>

                <TabsContent value="all" className="space-y-4">
                  {operations.map((operation) => {
                    const Icon = getOperationIcon(operation.type);
                    return (
                      <motion.div
                        key={operation.id}
                        layout
                        initial={{ opacity: 0, x: -20 }}
                        animate={{ opacity: 1, x: 0 }}
                        className="p-4 bg-zinc-800/30 rounded-lg border border-zinc-700/50 hover:border-zinc-600/50 transition-colors cursor-pointer"
                        onClick={() => setSelectedOperation(selectedOperation === operation.id ? null : operation.id)}
                      >
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-3">
                            <motion.div
                              animate={operation.status === 'running' ? { rotate: 360 } : {}}
                              transition={{ duration: 2, repeat: operation.status === 'running' ? Infinity : 0, ease: "linear" }}
                            >
                              <Icon className={`h-5 w-5 ${getOperationColor(operation.status)}`} />
                            </motion.div>
                            <div>
                              <h3 className="font-medium text-white capitalize">
                                {operation.type.replace('_', ' ')}
                              </h3>
                              <p className="text-sm text-zinc-400">
                                {operation.completed_at ? 
                                  `Completed ${formatDuration(operation.completed_at)} ago` :
                                  `Started ${formatDuration(operation.started_at)} ago`
                                }
                              </p>
                            </div>
                          </div>
                          <div className="flex items-center gap-3">
                            {operation.status === 'running' && (
                              <div className="text-sm text-blue-400">{operation.progress}%</div>
                            )}
                            <Badge className={`${
                              operation.status === 'completed' ? 'bg-green-500/20 text-green-300 border-green-500/30' :
                              operation.status === 'running' ? 'bg-blue-500/20 text-blue-300 border-blue-500/30' :
                              operation.status === 'failed' ? 'bg-red-500/20 text-red-300 border-red-500/30' :
                              'bg-yellow-500/20 text-yellow-300 border-yellow-500/30'
                            }`}>
                              {operation.status}
                            </Badge>
                          </div>
                        </div>
                        
                        <AnimatePresence>
                          {selectedOperation === operation.id && (
                            <motion.div
                              initial={{ height: 0, opacity: 0 }}
                              animate={{ height: "auto", opacity: 1 }}
                              exit={{ height: 0, opacity: 0 }}
                              className="mt-4 pt-4 border-t border-zinc-700/50 overflow-hidden"
                            >
                              <div className="space-y-3">
                                {operation.status === 'running' && (
                                  <div className="space-y-2">
                                    <div className="flex justify-between text-sm">
                                      <span className="text-zinc-400">Progress</span>
                                      <span className="text-white">{operation.progress}%</span>
                                    </div>
                                    <Progress value={operation.progress} className="h-2" />
                                  </div>
                                )}
                                {operation.results && (
                                  <div className="grid grid-cols-3 gap-4 text-center">
                                    <div className="p-2 bg-green-500/10 rounded">
                                      <p className="text-lg font-bold text-green-400">{operation.results.processed}</p>
                                      <p className="text-xs text-green-300">Processed</p>
                                    </div>
                                    <div className="p-2 bg-blue-500/10 rounded">
                                      <p className="text-lg font-bold text-blue-400">{operation.results.fixed}</p>
                                      <p className="text-xs text-blue-300">Fixed</p>
                                    </div>
                                    <div className="p-2 bg-red-500/10 rounded">
                                      <p className="text-lg font-bold text-red-400">{operation.results.failed}</p>
                                      <p className="text-xs text-red-300">Failed</p>
                                    </div>
                                  </div>
                                )}
                                {operation.errors.length > 0 && (
                                  <div className="space-y-2">
                                    <h4 className="text-sm font-medium text-red-400">Errors:</h4>
                                    {operation.errors.map((error, index) => (
                                      <p key={index} className="text-xs text-red-300 bg-red-500/10 p-2 rounded">
                                        {error}
                                      </p>
                                    ))}
                                  </div>
                                )}
                              </div>
                            </motion.div>
                          )}
                        </AnimatePresence>
                      </motion.div>
                    );
                  })}
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>
        </motion.div>
      </div>
    </div>
  );
}