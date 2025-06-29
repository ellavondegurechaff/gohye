"use client";

import { useEffect, useState, useCallback, useMemo, memo } from "react";
import { useRouter } from "next/navigation";
import { motion, AnimatePresence } from "framer-motion";
import { toast } from "sonner";
import { useDebounce } from "use-debounce";

// UI Components
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { 
  DropdownMenu, 
  DropdownMenuContent, 
  DropdownMenuItem, 
  DropdownMenuSeparator, 
  DropdownMenuTrigger 
} from "@/components/ui/dropdown-menu";

// Icons
import { 
  Plus, Search, Filter, Grid3X3, List, MoreVertical, Eye, Edit, Trash2,
  FolderOpen, Users, Star, TrendingUp, Database, Sparkles, Crown,
  Music, Heart, Zap, Target, Clock, BarChart3, Volume2, VolumeX,
  Palette, Layers, Move3D, ExternalLink, Download, Share2, CreditCard
} from "lucide-react";

// Types
import type { CollectionDTO } from "@/lib/types";
import { apiClient } from "@/lib/api";

interface RevolutionaryCollectionsProps {
  initialCollections?: CollectionDTO[];
}

// Collection Type Colors
const collectionTypeColors = {
  girl_group: { gradient: "from-pink-500 to-rose-500", bg: "bg-pink-500/20", text: "text-pink-300", border: "border-pink-500/30" },
  boy_group: { gradient: "from-blue-500 to-cyan-500", bg: "bg-blue-500/20", text: "text-blue-300", border: "border-blue-500/30" },
  other: { gradient: "from-zinc-500 to-gray-500", bg: "bg-zinc-500/20", text: "text-zinc-300", border: "border-zinc-500/30" },
};

// Enhanced Collection Card Component
const CollectionCard = memo(({ collection, onAction }: { 
  collection: CollectionDTO; 
  onAction: (action: string, collection: CollectionDTO) => void; 
}) => {
  const colors = collectionTypeColors[collection.collection_type] || collectionTypeColors.other;
  
  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -20 }}
      whileHover={{ y: -8, scale: 1.02 }}
      className="group cursor-pointer"
    >
      <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10 hover:border-white/20 transition-all duration-300 overflow-hidden relative h-full">
        {/* Gradient Background */}
        <div className={`absolute inset-0 bg-gradient-to-br ${colors.gradient} opacity-0 group-hover:opacity-10 transition-opacity duration-300`} />
        
        {/* Header */}
        <CardHeader className="relative z-10 pb-3">
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-3">
              <div className={`p-3 rounded-xl bg-gradient-to-br ${colors.gradient} shadow-lg`}>
                <FolderOpen className="h-6 w-6 text-white" />
              </div>
              <div>
                <Badge className={`${colors.bg} ${colors.text} ${colors.border} font-medium`}>
                  {collection.collection_type.replace('_', ' ').toUpperCase()}
                </Badge>
              </div>
            </div>
            
            {/* Action Menu */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button 
                  variant="ghost" 
                  size="sm" 
                  className="opacity-0 group-hover:opacity-100 transition-opacity duration-300 h-8 w-8 p-0"
                >
                  <MoreVertical className="h-4 w-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48 bg-black/90 backdrop-blur-xl border border-white/10">
                {[
                  { icon: Eye, label: "View Details", action: "view" },
                  { icon: Edit, label: "Edit Collection", action: "edit" },
                  { icon: BarChart3, label: "View Analytics", action: "analytics" },
                  { icon: Download, label: "Export Cards", action: "export" },
                  { icon: Share2, label: "Share Collection", action: "share" },
                  null,
                  { icon: Trash2, label: "Delete", action: "delete", destructive: true },
                ].map((item, index) => 
                  item === null ? (
                    <DropdownMenuSeparator key={index} className="bg-white/10" />
                  ) : (
                    <DropdownMenuItem 
                      key={item.action}
                      onClick={() => onAction(item.action, collection)}
                      className={`cursor-pointer ${
                        item.destructive ? 'text-red-400 focus:text-red-300' : 'text-white focus:text-white'
                      }`}
                    >
                      <item.icon className="mr-3 h-4 w-4" />
                      {item.label}
                    </DropdownMenuItem>
                  )
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </CardHeader>

        <CardContent 
          className="relative z-10 space-y-4 cursor-pointer" 
          onClick={() => onAction("view_cards", collection)}
        >
          {/* Collection Name */}
          <div>
            <h3 className="text-xl font-bold text-white group-hover:text-transparent group-hover:bg-gradient-to-r group-hover:bg-clip-text group-hover:from-white group-hover:to-zinc-300 transition-all duration-300 line-clamp-2">
              {collection.name}
            </h3>
            {collection.description && (
              <p className="text-sm text-zinc-400 mt-2 line-clamp-3 group-hover:text-zinc-300 transition-colors">
                {collection.description}
              </p>
            )}
          </div>

          {/* Stats */}
          <div className="flex items-center justify-between text-sm">
            <div className="flex items-center gap-1 text-zinc-400">
              <Database className="h-4 w-4" />
              <span>{collection.card_count?.toLocaleString() || 0} cards</span>
            </div>
            <div className="flex items-center gap-1 text-zinc-500">
              <Clock className="h-3 w-3" />
              <span>{new Date(collection.created_at).toLocaleDateString()}</span>
            </div>
          </div>

          {/* Tags */}
          {collection.tags && collection.tags.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {collection.tags.slice(0, 3).map((tag, index) => (
                <Badge key={index} variant="outline" className="text-xs border-zinc-600/50 text-zinc-400 bg-zinc-800/50">
                  #{tag}
                </Badge>
              ))}
              {collection.tags.length > 3 && (
                <Badge variant="outline" className="text-xs border-zinc-600/50 text-zinc-400 bg-zinc-800/50">
                  +{collection.tags.length - 3}
                </Badge>
              )}
            </div>
          )}

          {/* Special Badges */}
          <div className="flex items-center gap-2">
            {collection.promo && (
              <Badge className="bg-orange-500/20 text-orange-300 border-orange-500/30 text-xs">
                <Star className="mr-1 h-3 w-3" />
                Promo
              </Badge>
            )}
            {collection.compressed && (
              <Badge className="bg-blue-500/20 text-blue-300 border-blue-500/30 text-xs">
                Compressed
              </Badge>
            )}
            {collection.fragments && (
              <Badge className="bg-purple-500/20 text-purple-300 border-purple-500/30 text-xs">
                Fragments
              </Badge>
            )}
          </div>
        </CardContent>

        {/* Hover Effect */}
        <div className="absolute inset-0 bg-gradient-radial from-white/5 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
      </Card>
    </motion.div>
  );
});

export function RevolutionaryCollections({ initialCollections }: RevolutionaryCollectionsProps) {
  const router = useRouter();
  
  // State
  const [collections, setCollections] = useState<CollectionDTO[]>(initialCollections || []);
  const [loading, setLoading] = useState(!initialCollections);
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState<string>("all");
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  
  const [debouncedSearch] = useDebounce(search, 300);

  // Fetch collections
  const fetchCollections = useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiClient.getCollections();
      setCollections(data);
    } catch (error: any) {
      console.error('Failed to fetch collections:', error);
      toast.error('Failed to load collections');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!initialCollections) {
      fetchCollections();
    }
  }, [fetchCollections, initialCollections]);

  // Filter collections
  const filteredCollections = useMemo(() => {
    return collections.filter(collection => {
      const matchesSearch = !debouncedSearch || 
        collection.name.toLowerCase().includes(debouncedSearch.toLowerCase()) ||
        collection.description?.toLowerCase().includes(debouncedSearch.toLowerCase()) ||
        collection.tags?.some(tag => tag.toLowerCase().includes(debouncedSearch.toLowerCase()));
      
      const matchesType = typeFilter === "all" || collection.collection_type === typeFilter;
      
      return matchesSearch && matchesType;
    });
  }, [collections, debouncedSearch, typeFilter]);

  // Stats
  const stats = useMemo(() => {
    const totalCards = collections.reduce((sum, c) => sum + (c.card_count || 0), 0);
    const typeDistribution = collections.reduce((acc, c) => {
      acc[c.collection_type] = (acc[c.collection_type] || 0) + 1;
      return acc;
    }, {} as Record<string, number>);
    
    return {
      total: collections.length,
      totalCards,
      girlGroups: typeDistribution.girl_group || 0,
      boyGroups: typeDistribution.boy_group || 0,
      other: typeDistribution.other || 0,
      promoCollections: collections.filter(c => c.promo).length,
    };
  }, [collections]);

  const handleCollectionAction = useCallback(async (action: string, collection: CollectionDTO) => {
    switch (action) {
      case 'view':
        router.push(`/dashboard/collections/${collection.id}`);
        break;
      case 'view_cards':
        router.push(`/dashboard/collections/${collection.id}/cards`);
        break;
      case 'edit':
        router.push(`/dashboard/collections/${collection.id}/edit`);
        break;
      case 'analytics':
        router.push(`/dashboard/collections/${collection.id}/analytics`);
        break;
      case 'export':
        toast.success(`Exporting ${collection.name} cards...`);
        break;
      case 'share':
        if (navigator.share) {
          navigator.share({
            title: collection.name,
            text: `Check out the ${collection.name} collection!`,
            url: window.location.href + `/${collection.id}`
          });
        } else {
          navigator.clipboard.writeText(window.location.href + `/${collection.id}`);
          toast.success('Collection link copied to clipboard');
        }
        break;
      case 'delete':
        if (confirm(`Delete "${collection.name}" collection? This will also delete all associated cards.`)) {
          try {
            await apiClient.deleteCollection(collection.id);
            toast.success(`Deleted "${collection.name}"`);
            fetchCollections();
          } catch (error: any) {
            toast.error('Failed to delete collection');
          }
        }
        break;
    }
  }, [router, fetchCollections]);

  return (
    <div className="min-h-screen bg-gradient-to-br from-black via-zinc-900/50 to-black relative overflow-hidden">
      {/* Animated Background */}
      <div className="absolute inset-0 opacity-20">
        <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-blue-500/10 rounded-full blur-3xl animate-pulse" />
        <div className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-purple-500/10 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '2s' }} />
        <div className="absolute top-1/2 left-1/2 w-96 h-96 bg-pink-500/10 rounded-full blur-3xl animate-pulse" style={{ animationDelay: '4s' }} />
      </div>

      <div className="relative z-10 container mx-auto px-6 py-8 space-y-8">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: -30 }}
          animate={{ opacity: 1, y: 0 }}
          className="text-center space-y-6"
        >
          <div className="space-y-6">
            <div className="space-y-4">
              <h1 className="text-5xl font-black bg-gradient-to-r from-white via-blue-200 to-purple-200 bg-clip-text text-transparent">
                Collections Gallery
              </h1>
              <p className="text-xl text-zinc-400 max-w-2xl mx-auto">
                Explore and manage your K-pop trading card collections
              </p>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex flex-wrap justify-center gap-4">
            <Button
              onClick={() => router.push('/dashboard/collections/new')}
              className="bg-gradient-to-r from-blue-600 via-blue-500 to-purple-600 hover:from-blue-700 hover:via-blue-600 hover:to-purple-700 text-white shadow-2xl shadow-blue-500/25 transition-all duration-300"
            >
              <Plus className="mr-2 h-4 w-4" />
              Create Collection
            </Button>
            <Button
              onClick={() => router.push('/dashboard/collections/cards/new')}
              className="bg-gradient-to-r from-green-600 via-green-500 to-emerald-600 hover:from-green-700 hover:via-green-600 hover:to-emerald-700 text-white shadow-2xl shadow-green-500/25 transition-all duration-300"
            >
              <CreditCard className="mr-2 h-4 w-4" />
              Add Card
            </Button>
            <Button
              variant="outline"
              onClick={fetchCollections}
              disabled={loading}
              className="border-zinc-700/50 hover:bg-zinc-800/50 transition-all duration-300"
            >
              <TrendingUp className={`mr-2 h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </Button>
          </div>
        </motion.div>

        {/* Stats Grid */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
          className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4"
        >
          {[
            { label: "Total Collections", value: stats.total, icon: FolderOpen, gradient: "from-blue-500 to-cyan-500" },
            { label: "Total Cards", value: stats.totalCards, icon: Database, gradient: "from-pink-500 to-rose-500" },
            { label: "Girl Groups", value: stats.girlGroups, icon: Heart, gradient: "from-pink-500 to-rose-500" },
            { label: "Boy Groups", value: stats.boyGroups, icon: Users, gradient: "from-blue-500 to-cyan-500" },
            { label: "Promo", value: stats.promoCollections, icon: Crown, gradient: "from-orange-500 to-amber-500" },
          ].map((stat, index) => (
            <motion.div
              key={stat.label}
              initial={{ opacity: 0, scale: 0.8 }}
              animate={{ opacity: 1, scale: 1 }}
              transition={{ delay: 0.5 + index * 0.1 }}
              whileHover={{ scale: 1.05 }}
            >
              <Card className="bg-gradient-to-br from-black/80 to-zinc-900/60 backdrop-blur-xl border-white/10 hover:border-white/20 transition-all duration-300">
                <CardContent className="p-4 text-center">
                  <div className={`mx-auto mb-3 p-3 rounded-xl bg-gradient-to-br ${stat.gradient} w-fit`}>
                    <stat.icon className="h-5 w-5 text-white" />
                  </div>
                  <p className="text-2xl font-bold text-white">{stat.value.toLocaleString()}</p>
                  <p className="text-xs text-zinc-400 uppercase tracking-wide">{stat.label}</p>
                </CardContent>
              </Card>
            </motion.div>
          ))}
        </motion.div>

        {/* Search and Filters */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.8 }}
        >
          <Card className="bg-gradient-to-br from-black/80 via-zinc-900/60 to-black/80 backdrop-blur-2xl border-white/10">
            <CardContent className="p-6">
              <div className="flex flex-col md:flex-row gap-4">
                {/* Search */}
                <div className="relative flex-1">
                  <Search className="absolute left-4 top-1/2 transform -translate-y-1/2 h-5 w-5 text-zinc-400" />
                  <Input
                    placeholder="Search collections by name, description, or tags..."
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    className="pl-12 h-12 bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white focus:border-blue-500/50 focus:ring-blue-500/20 transition-all duration-300"
                  />
                </div>

                {/* Type Filter */}
                <Select value={typeFilter} onValueChange={setTypeFilter}>
                  <SelectTrigger className="w-full md:w-[200px] h-12 bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                    <SelectItem value="all">All Types</SelectItem>
                    <SelectItem value="girl_group">Girl Groups</SelectItem>
                    <SelectItem value="boy_group">Boy Groups</SelectItem>
                    <SelectItem value="other">Other</SelectItem>
                  </SelectContent>
                </Select>

                {/* View Mode */}
                <Tabs value={viewMode} onValueChange={(value) => setViewMode(value as any)}>
                  <TabsList className="bg-black/60 backdrop-blur-xl border border-zinc-800/50">
                    <TabsTrigger value="grid" className="data-[state=active]:bg-zinc-700/50">
                      <Grid3X3 className="h-4 w-4" />
                    </TabsTrigger>
                    <TabsTrigger value="list" className="data-[state=active]:bg-zinc-700/50">
                      <List className="h-4 w-4" />
                    </TabsTrigger>
                  </TabsList>
                </Tabs>

              </div>
            </CardContent>
          </Card>
        </motion.div>

        {/* Collections Display */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 1 }}
        >
          {loading ? (
            <div className="flex items-center justify-center py-20">
              <div className="text-center">
                <motion.div
                  animate={{ rotate: 360 }}
                  transition={{ duration: 1, repeat: Infinity, ease: "linear" }}
                  className="inline-block"
                >
                  <FolderOpen className="h-12 w-12 text-blue-500 mx-auto mb-4" />
                </motion.div>
                <p className="text-white font-bold text-xl mb-2">Loading collections...</p>
                <p className="text-zinc-400">Building your gallery experience</p>
              </div>
            </div>
          ) : filteredCollections.length === 0 ? (
            <div className="text-center py-20">
              <div className="mb-8">
                <motion.div 
                  className="mx-auto w-32 h-32 bg-gradient-to-br from-blue-500/20 to-purple-500/20 rounded-full flex items-center justify-center shadow-2xl backdrop-blur-xl border border-white/10"
                  animate={{ 
                    scale: [1, 1.1, 1],
                    rotate: [0, 10, -10, 0]
                  }}
                  transition={{ 
                    duration: 3, 
                    repeat: Infinity,
                    ease: "easeInOut"
                  }}
                >
                  <FolderOpen className="h-16 w-16 text-blue-400" />
                </motion.div>
              </div>
              <h3 className="text-3xl font-bold text-white mb-4">No collections found</h3>
              <p className="text-zinc-400 text-lg mb-8 max-w-md mx-auto">
                {search ? "No collections match your search criteria." : "Start building your K-pop collection empire by creating your first collection."}
              </p>
              <Button
                onClick={() => router.push('/dashboard/collections/new')}
                className="bg-gradient-to-r from-blue-600 via-blue-500 to-purple-600 hover:from-blue-700 hover:via-blue-600 hover:to-purple-700 text-white shadow-2xl shadow-blue-500/25"
              >
                <Plus className="mr-2 h-4 w-4" />
                Create Your First Collection
              </Button>
            </div>
          ) : (
            <div className="space-y-6">
              <div className="flex items-center justify-between">
                <h2 className="text-2xl font-bold text-white">
                  {filteredCollections.length} Collection{filteredCollections.length !== 1 ? 's' : ''}
                </h2>
                <Badge variant="secondary" className="bg-blue-500/20 text-blue-300 border-blue-500/30">
                  {filteredCollections.reduce((sum, c) => sum + (c.card_count || 0), 0).toLocaleString()} total cards
                </Badge>
              </div>

              <AnimatePresence mode="wait">
                {viewMode === 'grid' ? (
                  <motion.div
                    key="grid"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6"
                  >
                    {filteredCollections.map((collection, index) => (
                      <motion.div
                        key={`grid-${collection.id}`}
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: index * 0.05 }}
                      >
                        <CollectionCard
                          collection={collection}
                          onAction={handleCollectionAction}
                        />
                      </motion.div>
                    ))}
                  </motion.div>
                ) : (
                  <motion.div
                    key="list"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    className="space-y-4"
                  >
                    {filteredCollections.map((collection, index) => (
                      <motion.div
                        key={`list-${collection.id}`}
                        initial={{ opacity: 0, x: -20 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{ delay: index * 0.03 }}
                        className="flex items-center gap-4 p-4 bg-black/40 backdrop-blur-xl border border-white/10 rounded-lg hover:border-white/20 transition-all duration-300"
                      >
                        <div className={`p-3 rounded-xl bg-gradient-to-br ${collectionTypeColors[collection.collection_type]?.gradient || collectionTypeColors.other.gradient}`}>
                          <FolderOpen className="h-5 w-5 text-white" />
                        </div>
                        <div className="flex-1 min-w-0">
                          <h3 className="font-semibold text-white truncate">{collection.name}</h3>
                          <p className="text-sm text-zinc-400 truncate">{collection.description || 'No description'}</p>
                          <p className="text-xs text-zinc-500 mt-1">{collection.card_count?.toLocaleString() || 0} cards</p>
                        </div>
                        <div className="flex items-center gap-2">
                          <Badge className={`${collectionTypeColors[collection.collection_type]?.bg} ${collectionTypeColors[collection.collection_type]?.text} ${collectionTypeColors[collection.collection_type]?.border} text-xs`}>
                            {collection.collection_type.replace('_', ' ')}
                          </Badge>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                                <MoreVertical className="h-4 w-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end" className="w-48 bg-black/90 backdrop-blur-xl border border-white/10">
                              {[
                                { icon: Eye, label: "View Details", action: "view" },
                                { icon: Edit, label: "Edit Collection", action: "edit" },
                                { icon: BarChart3, label: "View Analytics", action: "analytics" },
                                null,
                                { icon: Trash2, label: "Delete", action: "delete", destructive: true },
                              ].map((item, index) => 
                                item === null ? (
                                  <DropdownMenuSeparator key={index} className="bg-white/10" />
                                ) : (
                                  <DropdownMenuItem 
                                    key={item.action}
                                    onClick={() => handleCollectionAction(item.action, collection)}
                                    className={`cursor-pointer ${
                                      item.destructive ? 'text-red-400' : 'text-white'
                                    }`}
                                  >
                                    <item.icon className="mr-3 h-4 w-4" />
                                    {item.label}
                                  </DropdownMenuItem>
                                )
                              )}
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                      </motion.div>
                    ))}
                  </motion.div>
                )}
              </AnimatePresence>
            </div>
          )}
        </motion.div>
      </div>
    </div>
  );
}