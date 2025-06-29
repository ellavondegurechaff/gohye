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
import { Separator } from "@/components/ui/separator";

// Icons
import { 
  Search, Filter, Grid3X3, List, ArrowLeft, FolderOpen,
  Database, Crown, Sparkles, Flame, TrendingUp, Users,
  Clock, Target, Star, Settings, Eye, Edit, Trash2
} from "lucide-react";

// Types
import type { CardDTO, CollectionDTO, APIResponse } from "@/lib/types";
import { apiClient } from "@/lib/api";
import { getLevelBadgeClasses } from "@/utils/card-helpers";

interface CollectionCardsPageContentProps {
  collectionId: string;
  searchParams: {
    page?: string;
    limit?: string;
    search?: string;
    level?: string;
    animated?: string;
  };
}

// Optimized Card Component for Collection View
const CollectionCardItem = memo(({ card }: { card: CardDTO }) => {
  const [isHovered, setIsHovered] = useState(false);

  const gradientStyle = useMemo(() => {
    const hue = (card.id * 137.508) % 360;
    return {
      background: `linear-gradient(135deg, 
        hsl(${hue}, 70%, 8%) 0%, 
        hsl(${(hue + 60) % 360}, 50%, 12%) 50%, 
        hsl(${(hue + 120) % 360}, 60%, 6%) 100%)`
    };
  }, [card.id]);

  return (
    <div
      className={`group relative cursor-pointer transition-all duration-300 ${
        isHovered ? 'transform -translate-y-1 scale-105' : ''
      }`}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      <Card className="overflow-hidden border-0 bg-transparent shadow-xl shadow-black/30 h-full">
        {/* Background */}
        <div 
          className="absolute inset-0 opacity-90"
          style={gradientStyle}
        />
        
        {/* Glassmorphism Overlay */}
        <div className="absolute inset-0 bg-gradient-to-br from-white/5 via-transparent to-black/20 backdrop-blur-[1px]" />
        
        {/* Card Image */}
        <div className="relative aspect-[3/4] overflow-hidden">
          {card.image_url ? (
            <img
              src={card.image_url}
              alt={card.name}
              loading="lazy"
              decoding="async"
              className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105"
              style={{ contentVisibility: 'auto' }}
            />
          ) : (
            <div className="w-full h-full bg-gradient-to-br from-zinc-800 to-zinc-700 flex items-center justify-center">
              <Database className="h-12 w-12 text-zinc-500" />
            </div>
          )}
          
          {/* Level Badge */}
          <div className="absolute bottom-2 left-2 z-10">
            <Badge 
              className={`${getLevelBadgeClasses(card.level)} relative overflow-hidden backdrop-blur-md text-xs`}
            >
              <Crown className="mr-1 h-3 w-3" />
              Level {card.level}
            </Badge>
          </div>

          {/* Type Badges */}
          <div className="absolute bottom-2 right-2 z-10 flex gap-1">
            {card.animated && (
              <Badge className="bg-pink-500/20 text-pink-300 border-pink-500/30 backdrop-blur-md text-xs">
                <Sparkles className="mr-1 h-2 w-2" />
                Animated
              </Badge>
            )}
          </div>
        </div>

        {/* Card Info */}
        <CardContent className="relative p-3 bg-black/50 backdrop-blur-md">
          <div className="space-y-2">
            <h3 className="font-semibold text-sm text-white truncate group-hover:text-transparent group-hover:bg-gradient-to-r group-hover:from-pink-400 group-hover:to-purple-400 group-hover:bg-clip-text transition-all duration-300">
              {card.name}
            </h3>
            <div className="flex items-center justify-between text-xs text-zinc-500">
              <span className="flex items-center gap-1">
                <Target className="h-3 w-3" />
                ID: {card.id}
              </span>
              <span className="flex items-center gap-1">
                <Clock className="h-3 w-3" />
                {new Date(card.created_at).toLocaleDateString()}
              </span>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
});

// Collection Header Component
const CollectionHeader = memo(({ collection, cardsCount }: { 
  collection: CollectionDTO | null; 
  cardsCount: number;
}) => {
  const router = useRouter();

  if (!collection) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-4">
          <Button
            variant="ghost"
            onClick={() => router.back()}
            className="text-zinc-400 hover:text-white"
          >
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back to Collections
          </Button>
        </div>
        <div className="text-center">
          <h1 className="text-3xl font-bold text-white">Collection Not Found</h1>
          <p className="text-zinc-400 mt-2">The requested collection could not be loaded.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Navigation */}
      <div className="flex items-center gap-4">
        <Button
          variant="ghost"
          onClick={() => router.back()}
          className="text-zinc-400 hover:text-white"
        >
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back to Collections
        </Button>
      </div>

      {/* Collection Info */}
      <div className="text-center space-y-4">
        <div className="flex items-center justify-center gap-4">
          <div className="p-4 rounded-xl bg-gradient-to-br from-blue-500 to-purple-500">
            <FolderOpen className="h-8 w-8 text-white" />
          </div>
          <div className="text-left">
            <h1 className="text-4xl font-bold bg-gradient-to-r from-white via-blue-200 to-purple-200 bg-clip-text text-transparent">
              {collection.name}
            </h1>
            {collection.description && (
              <p className="text-zinc-400 mt-2 max-w-2xl">
                {collection.description}
              </p>
            )}
          </div>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 max-w-2xl mx-auto">
          <Card className="bg-black/40 backdrop-blur-xl border-white/10 p-4 text-center">
            <div className="text-2xl font-bold text-white">{cardsCount}</div>
            <div className="text-xs text-zinc-400 uppercase tracking-wide">Total Cards</div>
          </Card>
          <Card className="bg-black/40 backdrop-blur-xl border-white/10 p-4 text-center">
            <div className="text-2xl font-bold text-white">{collection.collection_type.replace('_', ' ')}</div>
            <div className="text-xs text-zinc-400 uppercase tracking-wide">Type</div>
          </Card>
          <Card className="bg-black/40 backdrop-blur-xl border-white/10 p-4 text-center">
            <div className="text-2xl font-bold text-white">{collection.promo ? 'Yes' : 'No'}</div>
            <div className="text-xs text-zinc-400 uppercase tracking-wide">Promo</div>
          </Card>
          <Card className="bg-black/40 backdrop-blur-xl border-white/10 p-4 text-center">
            <div className="text-2xl font-bold text-white">{new Date(collection.created_at).getFullYear()}</div>
            <div className="text-xs text-zinc-400 uppercase tracking-wide">Created</div>
          </Card>
        </div>
      </div>
    </div>
  );
});

export function CollectionCardsPageContent({ collectionId, searchParams }: CollectionCardsPageContentProps) {
  // State
  const [cards, setCards] = useState<CardDTO[]>([]);
  const [collection, setCollection] = useState<CollectionDTO | null>(null);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState(searchParams.search || "");
  const [levelFilter, setLevelFilter] = useState(searchParams.level || "all");
  const [animatedFilter, setAnimatedFilter] = useState(searchParams.animated || "all");
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [displayCount, setDisplayCount] = useState(24); // Show only first 24 cards initially

  const [debouncedSearch] = useDebounce(search, 500); // Increase debounce time

  // Fetch collection and cards
  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      // Fetch cards for this collection
      const cardsResponse = await fetch(`/api/collections/${collectionId}/cards`);
      if (!cardsResponse.ok) {
        throw new Error('Failed to fetch cards');
      }
      const cardsResult: APIResponse<CardDTO[]> = await cardsResponse.json();
      
      // Fetch collection info
      const collectionsResponse = await apiClient.getCollections();
      const foundCollection = collectionsResponse.find(c => c.id === collectionId);
      
      setCards(cardsResult.data || []);
      setCollection(foundCollection || null);
    } catch (error: any) {
      console.error('Failed to fetch collection data:', error);
      toast.error('Failed to load collection data');
    } finally {
      setLoading(false);
    }
  }, [collectionId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Filter cards
  const filteredCards = useMemo(() => {
    return cards.filter(card => {
      const matchesSearch = !debouncedSearch || 
        card.name.toLowerCase().includes(debouncedSearch.toLowerCase()) ||
        card.tags?.some(tag => tag.toLowerCase().includes(debouncedSearch.toLowerCase()));
      
      const matchesLevel = levelFilter === "all" || card.level.toString() === levelFilter;
      const matchesAnimated = animatedFilter === "all" || 
        (animatedFilter === "true" && card.animated) ||
        (animatedFilter === "false" && !card.animated);
      
      return matchesSearch && matchesLevel && matchesAnimated;
    });
  }, [cards, debouncedSearch, levelFilter, animatedFilter]);

  // Display cards with pagination
  const displayedCards = useMemo(() => {
    return filteredCards.slice(0, displayCount);
  }, [filteredCards, displayCount]);

  const hasMore = filteredCards.length > displayCount;

  const loadMore = useCallback(() => {
    setDisplayCount(prev => Math.min(prev + 24, filteredCards.length));
  }, [filteredCards.length]);

  // Reset display count when filters change
  useEffect(() => {
    setDisplayCount(24);
  }, [debouncedSearch, levelFilter, animatedFilter]);

  if (loading) {
    return <div className="text-white text-center py-20">Loading collection...</div>;
  }

  return (
    <div className="container mx-auto px-6 py-8 space-y-8">
      {/* Header */}
      <CollectionHeader collection={collection} cardsCount={cards.length} />

      {/* Filters */}
      <Card className="bg-gradient-to-br from-black/80 via-zinc-900/60 to-black/80 backdrop-blur-2xl border-white/10">
        <CardContent className="p-6">
          <div className="flex flex-col md:flex-row gap-4">
            {/* Search */}
            <div className="relative flex-1">
              <Search className="absolute left-4 top-1/2 transform -translate-y-1/2 h-5 w-5 text-zinc-400" />
              <Input
                placeholder="Search cards by name or tags..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-12 h-12 bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white focus:border-blue-500/50 focus:ring-blue-500/20"
              />
            </div>

            {/* Level Filter */}
            <Select value={levelFilter} onValueChange={setLevelFilter}>
              <SelectTrigger className="w-full md:w-[150px] h-12 bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                <SelectValue placeholder="All Levels" />
              </SelectTrigger>
              <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                <SelectItem value="all">All Levels</SelectItem>
                <SelectItem value="1">Level 1</SelectItem>
                <SelectItem value="2">Level 2</SelectItem>
                <SelectItem value="3">Level 3</SelectItem>
                <SelectItem value="4">Level 4</SelectItem>
                <SelectItem value="5">Level 5</SelectItem>
              </SelectContent>
            </Select>

            {/* Animated Filter */}
            <Select value={animatedFilter} onValueChange={setAnimatedFilter}>
              <SelectTrigger className="w-full md:w-[150px] h-12 bg-black/40 backdrop-blur-xl border-zinc-700/50 text-white">
                <SelectValue placeholder="All Types" />
              </SelectTrigger>
              <SelectContent className="bg-black/90 backdrop-blur-xl border-zinc-700/50">
                <SelectItem value="all">All Types</SelectItem>
                <SelectItem value="true">Animated</SelectItem>
                <SelectItem value="false">Static</SelectItem>
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

      {/* Results Summary */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold text-white">
          {filteredCards.length} Card{filteredCards.length !== 1 ? 's' : ''}
        </h2>
        <Badge variant="secondary" className="bg-blue-500/20 text-blue-300 border-blue-500/30">
          {search && `Filtered by: "${search}"`}
        </Badge>
      </div>

      {/* Cards Display */}
      {filteredCards.length === 0 ? (
        <div className="text-center py-20">
          <div className="mb-8">
            <div className="mx-auto w-32 h-32 bg-gradient-to-br from-blue-500/20 to-purple-500/20 rounded-full flex items-center justify-center shadow-2xl backdrop-blur-xl border border-white/10">
              <Database className="h-16 w-16 text-blue-400" />
            </div>
          </div>
          <h3 className="text-3xl font-bold text-white mb-4">No cards found</h3>
          <p className="text-zinc-400 text-lg mb-8 max-w-md mx-auto">
            {search ? "No cards match your search criteria." : "This collection doesn't have any cards yet."}
          </p>
        </div>
      ) : (
        <div className="space-y-8">
          <AnimatePresence mode="wait">
            {viewMode === 'grid' ? (
              <motion.div
                key="grid"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6"
              >
                {displayedCards.map((card, index) => (
                  <motion.div
                    key={`grid-${card.id}`}
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: Math.min(index * 0.02, 0.5) }} // Cap delay for performance
                  >
                    <CollectionCardItem card={card} />
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
                {displayedCards.map((card, index) => (
                  <motion.div
                    key={`list-${card.id}`}
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: Math.min(index * 0.01, 0.3) }} // Cap delay for performance
                    className="flex items-center gap-4 p-4 bg-black/40 backdrop-blur-xl border border-white/10 rounded-lg hover:border-white/20 transition-all duration-300"
                  >
                    <div className="w-16 h-20 rounded-lg overflow-hidden bg-zinc-800 flex-shrink-0">
                      {card.image_url ? (
                        <img 
                          src={card.image_url} 
                          alt={card.name} 
                          loading="lazy"
                          className="w-full h-full object-cover" 
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center">
                          <Database className="h-6 w-6 text-zinc-500" />
                        </div>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <h3 className="font-semibold text-white truncate">{card.name}</h3>
                      <p className="text-sm text-zinc-400 truncate">Level {card.level} • ID: {card.id}</p>
                      <p className="text-xs text-zinc-500 mt-1">{new Date(card.created_at).toLocaleDateString()}</p>
                    </div>
                    <div className="flex items-center gap-2">
                      <Badge className={`${getLevelBadgeClasses(card.level)} text-xs`}>
                        Level {card.level}
                      </Badge>
                      {card.animated && (
                        <Badge className="bg-pink-500/20 text-pink-300 border-pink-500/30 text-xs">
                          Animated
                        </Badge>
                      )}
                    </div>
                  </motion.div>
                ))}
              </motion.div>
            )}
          </AnimatePresence>

          {/* Load More Button */}
          {hasMore && (
            <div className="text-center">
              <Button
                onClick={loadMore}
                variant="outline"
                className="border-zinc-700/50 hover:bg-zinc-800/50 text-white"
              >
                <TrendingUp className="mr-2 h-4 w-4" />
                Load More Cards ({filteredCards.length - displayCount} remaining)
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}